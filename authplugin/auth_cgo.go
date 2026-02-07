package main

/*
#cgo darwin pkg-config: libmosquitto libcjson
#cgo darwin LDFLAGS: -Wl,-undefined,dynamic_lookup
#cgo linux  pkg-config: libmosquitto libcjson
#include <stdlib.h>
#include <mosquitto.h>

typedef int (*mosq_event_cb)(int event, void *event_data, void *userdata);

int basic_auth_cb_c(int event, void *event_data, void *userdata);

int register_event_callback(mosquitto_plugin_id_t *id, int event, mosq_event_cb cb);
int unregister_event_callback(mosquitto_plugin_id_t *id, int event, mosq_event_cb cb);
void go_mosq_log(int level, const char* msg);
*/
import "C"

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"
	"unsafe"

	"github.com/jackc/pgx/v5/pgxpool"

	"mosquitto-plugin/internal/pluginutil"
)

var pid *C.mosquitto_plugin_id_t

// mosqLog 写入 Mosquitto 的日志系统。
func mosqLog(level C.int, msg string) {
	cs := C.CString(msg)
	defer C.free(unsafe.Pointer(cs))
	C.go_mosq_log(level, cs)
}

const (
	mosqLogInfo    = int(C.MOSQ_LOG_INFO)
	mosqLogWarning = int(C.MOSQ_LOG_WARNING)
	mosqLogError   = int(C.MOSQ_LOG_ERR)
)

// logKV 以 key=value 形式输出结构化字段。
func logKV(level int, msg string, kv ...any) {
	var b strings.Builder
	b.WriteString(msg)
	for i := 0; i+1 < len(kv); i += 2 {
		fmt.Fprintf(&b, " %v=%v", kv[i], kv[i+1])
	}
	mosqLog(C.int(level), b.String())
}

func cstr(s *C.char) string {
	if s == nil {
		return ""
	}
	return C.GoString(s)
}

// clientInfoFromBasicAuth 提取 BASIC_AUTH 事件中的客户端信息。
func clientInfoFromBasicAuth(ed *C.struct_mosquitto_evt_basic_auth) clientInfo {
	info := clientInfo{
		username: cstr(ed.username),
	}
	if ed.client == nil {
		return info
	}
	info.clientID = cstr(C.mosquitto_client_id(ed.client))
	if info.username == "" {
		info.username = cstr(C.mosquitto_client_username(ed.client))
	}
	info.peer = cstr(C.mosquitto_client_address(ed.client))
	info.protocol = pluginutil.ProtocolString(int(C.mosquitto_client_protocol_version(ed.client)))
	return info
}

// go_mosq_plugin_version 选择最高支持的插件 API 版本。
//
//export go_mosq_plugin_version
func go_mosq_plugin_version(count C.int, versions *C.int) C.int {
	for _, v := range unsafe.Slice(versions, int(count)) {
		if v == 5 {
			return 5
		}
	}
	return -1
}

// go_mosq_plugin_init 解析配置、校验参数并注册回调。
//
//export go_mosq_plugin_init
func go_mosq_plugin_init(id *C.mosquitto_plugin_id_t, userdata *unsafe.Pointer,
	opts *C.struct_mosquitto_opt, optCount C.int) (rc C.int) {

	defer func() {
		if r := recover(); r != nil {
			logKV(mosqLogError, "auth-plugin: panic in plugin_init",
				"panic", r,
				"stack", string(debug.Stack()),
			)
			rc = C.MOSQ_ERR_UNKNOWN
		}
	}()

	pid = id
	if env := os.Getenv("PG_DSN"); env != "" {
		pgDSN = env
	}
	for _, o := range unsafe.Slice(opts, int(optCount)) {
		key, value := cstr(o.key), cstr(o.value)
		switch key {
		case "pg_dsn":
			pgDSN = value
		case "timeout_ms":
			if dur, ok := pluginutil.ParseTimeoutMS(value); ok {
				timeout = dur
			} else {
				logKV(mosqLogWarning, "auth-plugin: invalid timeout_ms",
					"value", value,
					"timeout_ms", int(timeout/time.Millisecond),
				)
			}
		case "fail_open":
			if parsed, ok := pluginutil.ParseBoolOption(value); ok {
				failOpen = parsed
			} else {
				logKV(mosqLogWarning, "auth-plugin: invalid fail_open",
					"value", value,
					"fail_open", failOpen,
				)
			}
		}
	}
	if pgDSN == "" {
		logKV(mosqLogError, "auth-plugin: pg_dsn must be set")
		return C.MOSQ_ERR_UNKNOWN
	}
	if _, err := pgxpool.ParseConfig(pgDSN); err != nil {
		logKV(mosqLogError, "auth-plugin: invalid pg_dsn",
			"pg_dsn", pluginutil.SafeDSN(pgDSN),
			"error", err.Error(),
		)
		return C.MOSQ_ERR_UNKNOWN
	}

	logKV(mosqLogInfo, "auth-plugin: initializing",
		"pg_dsn", pluginutil.SafeDSN(pgDSN),
		"timeout_ms", int(timeout/time.Millisecond),
		"fail_open", failOpen,
	)

	// 数据库暂不可用时不阻塞插件加载
	ctx, cancel := timeoutContext(timeout)
	defer cancel()
	if _, err := ensurePool(ctx); err != nil {
		logKV(mosqLogWarning, "auth-plugin: initial pg connection failed",
			"error", err.Error(),
		)
	}

	// 注册回调
	if rc := C.register_event_callback(pid, C.MOSQ_EVT_BASIC_AUTH, C.mosq_event_cb(C.basic_auth_cb_c)); rc != C.MOSQ_ERR_SUCCESS {
		return rc
	}

	logKV(mosqLogInfo, "auth-plugin: plugin initialized")
	return C.MOSQ_ERR_SUCCESS
}

// go_mosq_plugin_cleanup 注销回调并释放连接池。
//
//export go_mosq_plugin_cleanup
func go_mosq_plugin_cleanup(userdata unsafe.Pointer, opts *C.struct_mosquitto_opt, optCount C.int) C.int {
	C.unregister_event_callback(pid, C.MOSQ_EVT_BASIC_AUTH, C.mosq_event_cb(C.basic_auth_cb_c))
	poolMu.Lock()
	defer poolMu.Unlock()
	if pool != nil {
		pool.Close()
		pool = nil
	}
	logKV(mosqLogInfo, "auth-plugin: plugin cleaned up")
	return C.MOSQ_ERR_SUCCESS
}

// basic_auth_cb_c 执行认证逻辑并返回结果。
//
//export basic_auth_cb_c
func basic_auth_cb_c(event C.int, event_data unsafe.Pointer, userdata unsafe.Pointer) C.int {
	ed := (*C.struct_mosquitto_evt_basic_auth)(event_data)
	password := cstr(ed.password)
	info := clientInfoFromBasicAuth(ed)

	dbAllow, dbReason, err := dbAuth(info.username, password, info.clientID)
	allow, result, reason := dbAllow, authResultFail, dbReason
	if err != nil {
		logKV(mosqLogWarning, "auth-plugin auth error", "error", err.Error())
		reason = authReasonDBError
		if failOpen {
			logKV(mosqLogInfo, "auth-plugin: fail_open allow auth", "reason", authReasonDBError)
			allow = true
			result = authResultSuccess
			reason = authReasonDBErrorFailOpen
		}
	} else if allow {
		result = authResultSuccess
	}

	if err := recordAuthEvent(info, result, reason); err != nil {
		logKV(mosqLogWarning, "auth-plugin auth event log failed", "error", err.Error())
	}
	if allow {
		return C.MOSQ_ERR_SUCCESS
	}
	return C.MOSQ_ERR_AUTH
}

func main() {}
