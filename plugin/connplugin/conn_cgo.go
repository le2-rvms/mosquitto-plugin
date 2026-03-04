package main

/*
#cgo darwin pkg-config: libmosquitto libcjson
#cgo darwin LDFLAGS: -Wl,-undefined,dynamic_lookup
#cgo linux  pkg-config: libmosquitto libcjson
#include <stdlib.h>
#include <mosquitto.h>

typedef int (*mosq_event_cb)(int event, void *event_data, void *userdata);

int connect_cb_c(int event, void *event_data, void *userdata);
int disconnect_cb_c(int event, void *event_data, void *userdata);

int register_event_callback(mosquitto_plugin_id_t *id, int event, mosq_event_cb cb);
int unregister_event_callback(mosquitto_plugin_id_t *id, int event, mosq_event_cb cb);
void go_mosq_log(int level, const char* msg);
*/
import "C"

import (
	"context"
	"os"
	"runtime/debug"
	"sync"
	"time"
	"unsafe"

	"github.com/jackc/pgx/v5/pgxpool"

	"mosquitto-plugin/internal/pluginutil"
)

var pid *C.mosquitto_plugin_id_t

const (
	mosqLogDebug   = int(C.MOSQ_LOG_DEBUG)
	mosqLogInfo    = int(C.MOSQ_LOG_INFO)
	mosqLogWarning = int(C.MOSQ_LOG_WARNING)
	mosqLogError   = int(C.MOSQ_LOG_ERR)
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

type connTracker struct {
	mu     sync.Mutex
	active map[uintptr]struct{}
}

func newConnTracker() *connTracker {
	return &connTracker{active: map[uintptr]struct{}{}}
}

func (t *connTracker) Reset() {
	t.mu.Lock()
	t.active = map[uintptr]struct{}{}
	t.mu.Unlock()
}

// MarkConnected 标记连接已建立。
func (t *connTracker) MarkConnected(key uintptr) {
	t.mu.Lock()
	t.active[key] = struct{}{}
	t.mu.Unlock()
}

// ConsumeConnected 原子地检查并清除连接状态；返回 true 表示该连接此前已建立。
func (t *connTracker) ConsumeConnected(key uintptr) bool {
	t.mu.Lock()
	_, ok := t.active[key]
	if ok {
		delete(t.active, key)
	}
	t.mu.Unlock()
	return ok
}

func connKey(client *C.struct_mosquitto) (uintptr, bool) {
	if client == nil {
		return 0, false
	}
	// 用客户端指针地址作为会话键，跨 connect/disconnect 回调关联同一连接。
	return uintptr(unsafe.Pointer(client)), true
}

// onConnect 统一处理连接事件：先标记连接状态，再记录 connect 事件。
func onConnect(client *C.struct_mosquitto) {
	key, ok := connKey(client)
	if !ok {
		return
	}
	activeConnections.MarkConnected(key)
	if err := recordEvent(clientInfoFromClient(client), connEventTypeConnect, nil); err != nil {
		log(mosqLogWarning, "conn-plugin: record connect event failed", map[string]any{"error": err.Error()})
	}
}

// onDisconnect 统一处理断连事件：去重后记录 disconnect 事件。
func onDisconnect(client *C.struct_mosquitto, reasonCode int) {
	key, ok := connKey(client)
	if !ok {
		return
	}
	// 原子地 “检查并清除” 连接状态，避免并发下重复记录 disconnect。
	if !activeConnections.ConsumeConnected(key) {
		if pluginutil.ShouldSample(&debugSkipCounter, debugSampleEvery) {
			log(mosqLogDebug, "conn-plugin: skip disconnect record", map[string]any{"client_ptr": key})
		}
		return
	}
	// disconnect 才会携带 reason code；connect 场景传 nil。
	reason := reasonCode
	if err := recordEvent(clientInfoFromClient(client), connEventTypeDisconnect, &reason); err != nil {
		log(mosqLogWarning, "conn-plugin: record disconnect event failed", map[string]any{"error": err.Error()})
	}
}

func clientInfoFromClient(client *C.struct_mosquitto) pluginutil.ClientInfo {
	info := pluginutil.ClientInfo{}
	if client == nil {
		return info
	}
	info.ClientID = cstr(C.mosquitto_client_id(client))
	info.Username = cstr(C.mosquitto_client_username(client))
	info.Peer = cstr(C.mosquitto_client_address(client))
	info.Protocol = pluginutil.ProtocolString(int(C.mosquitto_client_protocol_version(client)))
	return info
}

//export go_mosq_plugin_version
func go_mosq_plugin_version(count C.int, versions *C.int) C.int {
	for _, v := range unsafe.Slice(versions, int(count)) {
		if v == 5 {
			return 5
		}
	}
	return -1
}

//export go_mosq_plugin_init
func go_mosq_plugin_init(id *C.mosquitto_plugin_id_t, userdata *unsafe.Pointer,
	opts *C.struct_mosquitto_opt, optCount C.int) (rc C.int) {

	defer func() {
		if r := recover(); r != nil {
			log(mosqLogError, "conn-plugin: panic in plugin_init", map[string]any{"panic": r, "stack": string(debug.Stack())})
			rc = C.MOSQ_ERR_UNKNOWN
		}
	}()

	pid = id
	pgDSN = ""
	timeout = defaultTimeout
	debugSkipCounter = 0
	debugRecordCounter = 0
	// 插件重载时先回收旧池，避免复用旧配置连接。
	poolHolder.Close()
	activeConnections.Reset()

	if env := os.Getenv("PG_DSN"); env != "" {
		pgDSN = env
	}
	for _, o := range unsafe.Slice(opts, int(optCount)) {
		key, value := cstr(o.key), cstr(o.value)
		switch key {
		case "conn_pg_dsn":
			pgDSN = value
		case "conn_timeout_ms":
			if dur, ok := pluginutil.ParseTimeoutMS(value); ok {
				timeout = dur
			} else {
				log(mosqLogWarning, "conn-plugin: invalid conn_timeout_ms", map[string]any{"value": value, "timeout_ms": int(timeout / time.Millisecond)})
			}
		}
	}

	if pgDSN == "" {
		log(mosqLogError, "conn-plugin: conn_pg_dsn must be set", nil)
		return C.MOSQ_ERR_UNKNOWN
	}
	if _, err := pgxpool.ParseConfig(pgDSN); err != nil {
		log(mosqLogError, "conn-plugin: invalid pg_dsn", map[string]any{"pg_dsn": pluginutil.SafeDSN(pgDSN), "error": err.Error()})
		return C.MOSQ_ERR_UNKNOWN
	}

	log(mosqLogInfo, "conn-plugin: initializing", map[string]any{"pg_dsn": pluginutil.SafeDSN(pgDSN), "timeout_ms": int(timeout / time.Millisecond)})

	ctx := context.Background()
	cancel := func() {}
	// timeout<=0 时显式使用无超时上下文，保持与历史行为一致。
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	defer cancel()
	_, created, err := poolHolder.Ensure(ctx, pgDSN, connPGPoolOptions)
	if err != nil {
		log(mosqLogWarning, "conn-plugin: initial pg connection failed", map[string]any{"error": err.Error()})
	} else if created {
		log(mosqLogInfo, "conn-plugin: postgres pool connected", map[string]any{"pg_dsn": pluginutil.SafeDSN(pgDSN)})
	}

	if rc := C.register_event_callback(pid, C.MOSQ_EVT_CONNECT, C.mosq_event_cb(C.connect_cb_c)); rc != C.MOSQ_ERR_SUCCESS {
		return rc
	}
	if rc := C.register_event_callback(pid, C.MOSQ_EVT_DISCONNECT, C.mosq_event_cb(C.disconnect_cb_c)); rc != C.MOSQ_ERR_SUCCESS {
		C.unregister_event_callback(pid, C.MOSQ_EVT_CONNECT, C.mosq_event_cb(C.connect_cb_c))
		return rc
	}

	log(mosqLogInfo, "conn-plugin: plugin initialized", nil)
	return C.MOSQ_ERR_SUCCESS
}

//export go_mosq_plugin_cleanup
func go_mosq_plugin_cleanup(userdata unsafe.Pointer, opts *C.struct_mosquitto_opt, optCount C.int) C.int {
	C.unregister_event_callback(pid, C.MOSQ_EVT_CONNECT, C.mosq_event_cb(C.connect_cb_c))
	C.unregister_event_callback(pid, C.MOSQ_EVT_DISCONNECT, C.mosq_event_cb(C.disconnect_cb_c))
	poolHolder.Close()

	activeConnections.Reset()

	log(mosqLogInfo, "conn-plugin: plugin cleaned up", nil)
	return C.MOSQ_ERR_SUCCESS
}

//export connect_cb_c
func connect_cb_c(event C.int, event_data unsafe.Pointer, userdata unsafe.Pointer) C.int {
	ed := (*C.struct_mosquitto_evt_connect)(event_data)
	if ed == nil {
		return C.MOSQ_ERR_SUCCESS
	}

	onConnect(ed.client)
	return C.MOSQ_ERR_SUCCESS
}

//export disconnect_cb_c
func disconnect_cb_c(event C.int, event_data unsafe.Pointer, userdata unsafe.Pointer) C.int {
	ed := (*C.struct_mosquitto_evt_disconnect)(event_data)
	if ed == nil {
		return C.MOSQ_ERR_SUCCESS
	}
	onDisconnect(ed.client, int(ed.reason))
	return C.MOSQ_ERR_SUCCESS
}

func main() {}
