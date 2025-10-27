#include <mosquitto.h>
#include <mosquitto_plugin.h>
#include <mosquitto_broker.h>

/* Go 导出的别名函数（避免与官方符号重名） */
int go_mosq_plugin_version(int supported_version_count, int *supported_versions);
int go_mosq_plugin_init(mosquitto_plugin_id_t *identifier, void **userdata,
                        struct mosquitto_opt *options, int option_count);
int go_mosq_plugin_cleanup(void *userdata, struct mosquitto_opt *options, int option_count);

/* —— 官方要求的 3 个入口：C 层精确匹配原型，再转调 Go —— */
int mosquitto_plugin_version(int supported_version_count, const int *supported_versions) {
    return go_mosq_plugin_version(supported_version_count, (int*)supported_versions);
}

int mosquitto_plugin_init(mosquitto_plugin_id_t *identifier, void **userdata,
                          struct mosquitto_opt *options, int option_count) {
    return go_mosq_plugin_init(identifier, userdata, options, option_count);
}

/* 这里是关键修复：cleanup 的第一个参数是 void *（单指针），不是 void ** */
int mosquitto_plugin_cleanup(void *userdata, struct mosquitto_opt *options, int option_count) {
    return go_mosq_plugin_cleanup(userdata, options, option_count);
}

/* 事件注册/反注册保持不变 */
int basic_auth_cb_c(int event, void *event_data, void *userdata);
int acl_check_cb_c (int event, void *event_data, void *userdata);

int register_basic_auth(mosquitto_plugin_id_t *id) {
    return mosquitto_callback_register(id, MOSQ_EVT_BASIC_AUTH, basic_auth_cb_c, NULL, NULL);
}
int unregister_basic_auth(mosquitto_plugin_id_t *id) {
    return mosquitto_callback_unregister(id, MOSQ_EVT_BASIC_AUTH, basic_auth_cb_c, NULL);
}
int register_acl_check(mosquitto_plugin_id_t *id) {
    return mosquitto_callback_register(id, MOSQ_EVT_ACL_CHECK, acl_check_cb_c, NULL, NULL);
}
int unregister_acl_check(mosquitto_plugin_id_t *id) {
    return mosquitto_callback_unregister(id, MOSQ_EVT_ACL_CHECK, acl_check_cb_c, NULL);
}

/* 避免 Go 直接调可变参 */
void go_mosq_log(int level, const char* msg) {
    mosquitto_log_printf(level, "%s", msg);
}
