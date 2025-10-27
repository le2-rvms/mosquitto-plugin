#include <mosquitto_broker.h>
#include <mosquitto_plugin.h>

/* 已有声明 */
int basic_auth_cb_c(int event, void *event_data, void *userdata);
int acl_check_cb_c(int event, void *event_data, void *userdata);

/* 新增：声明 Go 导出的版本函数（非同名） */
int go_mosq_plugin_version(int supported_version_count, int *supported_versions);

/* 提供头文件期望的真正符号，并把 const int* 转成 int* 再转调 */
int mosquitto_plugin_version(int supported_version_count, const int *supported_versions) {
    return go_mosq_plugin_version(supported_version_count, (int*)supported_versions);
}

int register_basic_auth(mosquitto_plugin_id_t *id) {
    return mosquitto_callback_register(id, MOSQ_EVT_BASIC_AUTH,
        basic_auth_cb_c, NULL, NULL);
}
int unregister_basic_auth(mosquitto_plugin_id_t *id) {
    return mosquitto_callback_unregister(id, MOSQ_EVT_BASIC_AUTH,
        basic_auth_cb_c, NULL);
}
int register_acl_check(mosquitto_plugin_id_t *id) {
    return mosquitto_callback_register(id, MOSQ_EVT_ACL_CHECK,
        acl_check_cb_c, NULL, NULL);
}
int unregister_acl_check(mosquitto_plugin_id_t *id) {
    return mosquitto_callback_unregister(id, MOSQ_EVT_ACL_CHECK,
        acl_check_cb_c, NULL);
}

/* 固定签名日志包装：避免从 Go 直接调可变参函数 */
void go_mosq_log(int level, const char* msg) {
    mosquitto_log_printf(level, "%s", msg);
}
