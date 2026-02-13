# Mosquitto 内建 ACL（acl_file）规范

本文档描述内建 ACL 的最佳实践与示例配置。

## 1. 设计原则

- **最小权限**：默认拒绝或仅放行必要主题。
- **按角色拆分**：运维/设备/应用分别配置。
- **按前缀划分**：每类客户端只访问其 namespace。
- **写权限谨慎**：写入主题尽量更精确，不允许 `#` 通配。

## 2. 推荐 ACL 示例

以下示例满足：

- 运维账户（`_ops`）拥有管理权限。
- 设备账户只能访问自己的主题。
- 应用服务账户按租户前缀访问。

```conf
# 全局规则（不在 user 块中，对所有用户生效）
# 设备账户：仅访问自身设备主题（假设 username 与 clientid 同名）
pattern readwrite v1/d/%c/#

# 运维账户：读写全局，但建议保留 $SYS 只读
user _ops
# 系统状态只读
topic read $SYS/#
# 运维全权限（按需保留或收紧）
topic readwrite v1/#

# 管理端账户：允许管理端读写所有业务主题
user app_web
topic readwrite v1/#

# 默认兜底：不匹配则拒绝
```

> 提示：Mosquitto `pattern` 使用 `%u`(username) / `%c`(clientid)。

## 3. Mosquitto 配置示例

```conf
allow_anonymous false
password_file /mosquitto/config/password_file
acl_file /mosquitto/config/acl_file
```
