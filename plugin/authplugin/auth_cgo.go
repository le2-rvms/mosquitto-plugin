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
	"context"
	"os"
	"runtime/debug"
	"strings"
	"time"
	"unsafe"

	"github.com/jackc/pgx/v5/pgxpool"

	"mosquitto-plugin/internal/pluginutil"
)

var pid *C.mosquitto_plugin_id_t

const (
	mosqLogInfo    = int(C.MOSQ_LOG_INFO)
	mosqLogWarning = int(C.MOSQ_LOG_WARNING)
	mosqLogError   = int(C.MOSQ_LOG_ERR)
	mosqErrDefer   = int(C.MOSQ_ERR_PLUGIN_DEFER)
	mosqErrAuth    = int(C.MOSQ_ERR_AUTH)
	mosqErrSuccess = int(C.MOSQ_ERR_SUCCESS)
)

func log(level int, msg string, fields map[string]any) {
	// 单测场景可能直接调用逻辑函数，此时插件尚未初始化，跳过 C 日志桥接。
	if pid == nil {
		return
	}
	cs := C.CString(pluginutil.FormatLogMessage(msg, fields))
	defer C.free(unsafe.Pointer(cs))
	C.go_mosq_log(C.int(level), cs)
}

func cstr(s *C.char) string {
	if s == nil {
		return ""
	}
	return C.GoString(s)
}

// clientInfoFromBasicAuth 提取 BASIC_AUTH 事件中的客户端信息。
func clientInfoFromBasicAuth(ed *C.struct_mosquitto_evt_basic_auth) pluginutil.ClientInfo {
	info := pluginutil.ClientInfo{
		Username: cstr(ed.username),
	}
	if ed.client == nil {
		return info
	}
	info.ClientID = cstr(C.mosquitto_client_id(ed.client))
	if info.Username == "" {
		info.Username = cstr(C.mosquitto_client_username(ed.client))
	}
	info.Peer = cstr(C.mosquitto_client_address(ed.client))
	info.Protocol = pluginutil.ProtocolString(int(C.mosquitto_client_protocol_version(ed.client)))
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
			log(mosqLogError, "auth-plugin: panic in plugin_init", map[string]any{"panic": r, "stack": string(debug.Stack())})
			rc = C.MOSQ_ERR_UNKNOWN
		}
	}()

	pid = id
	pgDSN = ""
	timeout = defaultTimeout
	failOpen = false
	// 重载时先回收旧池，避免沿用过期配置。
	poolHolder.Close()

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
				log(mosqLogWarning, "auth-plugin: invalid timeout_ms", map[string]any{"value": value, "timeout_ms": int(timeout / time.Millisecond)})
			}
		case "fail_open":
			if parsed, ok := parseBoolOption(value); ok {
				failOpen = parsed
			} else {
				log(mosqLogWarning, "auth-plugin: invalid fail_open", map[string]any{"value": value, "fail_open": failOpen})
			}
		}
	}
	if pgDSN == "" {
		log(mosqLogError, "auth-plugin: pg_dsn must be set", nil)
		return C.MOSQ_ERR_UNKNOWN
	}
	if _, err := pgxpool.ParseConfig(pgDSN); err != nil {
		log(mosqLogError, "auth-plugin: invalid pg_dsn", map[string]any{"pg_dsn": pluginutil.SafeDSN(pgDSN), "error": err.Error()})
		return C.MOSQ_ERR_UNKNOWN
	}

	log(mosqLogInfo, "auth-plugin: initializing", map[string]any{"pg_dsn": pluginutil.SafeDSN(pgDSN), "timeout_ms": int(timeout / time.Millisecond), "fail_open": failOpen})

	// 数据库暂不可用时不阻塞插件加载
	ctx := context.Background()
	cancel := func() {}
	// timeout<=0 时显式使用无超时上下文，避免启动阶段被意外取消。
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	defer cancel()
	_, created, err := poolHolder.Ensure(ctx, pgDSN, authPGPoolOptions)
	if err != nil {
		log(mosqLogWarning, "auth-plugin: initial pg connection failed", map[string]any{"error": err.Error()})
	} else if created {
		log(mosqLogInfo, "auth-plugin: postgres pool connected", map[string]any{"pg_dsn": pluginutil.SafeDSN(pgDSN)})
	}

	// 注册回调
	if rc := C.register_event_callback(pid, C.MOSQ_EVT_BASIC_AUTH, C.mosq_event_cb(C.basic_auth_cb_c)); rc != C.MOSQ_ERR_SUCCESS {
		return rc
	}

	log(mosqLogInfo, "auth-plugin: plugin initialized", nil)
	return C.MOSQ_ERR_SUCCESS
}

// go_mosq_plugin_cleanup 注销回调并释放连接池。
//
//export go_mosq_plugin_cleanup
func go_mosq_plugin_cleanup(userdata unsafe.Pointer, opts *C.struct_mosquitto_opt, optCount C.int) C.int {
	C.unregister_event_callback(pid, C.MOSQ_EVT_BASIC_AUTH, C.mosq_event_cb(C.basic_auth_cb_c))
	poolHolder.Close()
	log(mosqLogInfo, "auth-plugin: plugin cleaned up", nil)
	return C.MOSQ_ERR_SUCCESS
}

func runBasicAuth(info pluginutil.ClientInfo, password string) C.int {
	if info.Username == "" || strings.HasPrefix(info.Username, "_") {
		return C.MOSQ_ERR_PLUGIN_DEFER
	}

	// 默认按拒绝处理；后续根据 DB 返回和 fail_open 逐步覆盖。
	code := C.int(mosqErrAuth)
	result := authResultFail
	reason := ""

	dbAllow, dbReason, dbErr := dbAuth(info.Username, password, info.ClientID)
	reason = dbReason
	if dbErr != nil {
		reason = authReasonDBError
		log(mosqLogWarning, "auth-plugin auth error", map[string]any{"error": dbErr.Error()})
		if failOpen {
			code = C.int(mosqErrSuccess)
			result = authResultSuccess
			reason = authReasonDBErrorFailOpen
			log(mosqLogInfo, "auth-plugin: fail_open allow auth", map[string]any{"reason": authReasonDBError})
		}
	} else if dbAllow {
		code = C.int(mosqErrSuccess)
		result = authResultSuccess
	}

	if err := recordAuthEvent(info, result, reason); err != nil {
		log(mosqLogWarning, "auth-plugin auth event log failed", map[string]any{"error": err.Error()})
	}

	return code
}

// basic_auth_cb_c 执行认证逻辑并返回结果。
//
//export basic_auth_cb_c
func basic_auth_cb_c(event C.int, event_data unsafe.Pointer, userdata unsafe.Pointer) C.int {
	if event_data == nil {
		log(mosqLogWarning, "auth-plugin: nil basic auth event_data", map[string]any{"event": int(event)})
		return C.MOSQ_ERR_AUTH
	}
	ed := (*C.struct_mosquitto_evt_basic_auth)(event_data)
	password := cstr(ed.password)
	info := clientInfoFromBasicAuth(ed)
	return runBasicAuth(info, password)
}

func main() {}
