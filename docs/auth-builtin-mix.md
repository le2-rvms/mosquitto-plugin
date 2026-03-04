# PRD：AuthPlugin 与内建认证/ACL 混合方案

**文档状态**：正式 PRD（阶段性单独维护，开发完成后合并至主文档）
**适用范围**：认证分流与内建 ACL 配置

## 1. 背景

当前系统需要同时支持：
- 通过 `authplugin` 进行数据库认证。
- 通过 Mosquitto 内建 `password_file` 进行本地认证。
- ACL 采用内建 `acl_file`。

目标是让两套认证并存且可控，且不引入额外的认证链路复杂度。

## 2. 目标

- `_` 前缀用户走内建认证（`password_file`）。
- 非 `_` 前缀用户走 `authplugin` 数据库认证。
- ACL 由内建 `acl_file` 统一管理。

## 3. 非目标

- 不实现 HTTP API 使用 `password_file` 的认证（Mosquitto 本身不支持）。
- 不实现插件级 ACL（`MOSQ_EVT_ACL_CHECK`）。
- 不支持内建认证优先于插件的反向流程（Mosquitto 不支持）。

## 4. 需求

### 4.1 认证分流规则

- `username` 为空或以 `_` 开头：`authplugin` 返回 `MOSQ_ERR_PLUGIN_DEFER`，交给内建认证处理。
- 其它用户名：`authplugin` 执行数据库认证并返回 `MOSQ_ERR_SUCCESS` 或 `MOSQ_ERR_AUTH`。

**影响与说明**
- `_` 前缀用户不由 `authplugin` 作最终裁决。
- 若需要记录这类请求，可记录为 `defer`，不应记录为 `fail`。
- 把“是否允许空用户名/匿名”交给内建认证决定。如果你配置 `allow_anonymous false`，内建也会拒绝，效果是一样的；如果你将来允许匿名，插件不会阻断。

### 4.2 password_file 维护

新增/更新用户（命令行直接指定密码）：

```bash
mosquitto_passwd -b /mosquitto/config/password_file _ops 'YourStrongPassword'
```

删除用户：

```bash
mosquitto_passwd -D /mosquitto/config/password_file _ops
```

查看用户列表（不显示明文密码）：

```bash
cut -d: -f1 /mosquitto/config/password_file
```

## 5. ACL 方案

ACL 采用内建 `acl_file`，详见：`docs/acl-file.md`。

## 6. 配置示例

```conf
allow_anonymous false
password_file /mosquitto/config/password_file
acl_file /mosquitto/config/acl_file

plugin /mosquitto/plugins/auth-plugin
plugin_opt_pg_dsn postgres://user:pass@127.0.0.1:5432/mqtt?sslmode=disable
```

## 7. 变更影响与注意事项

- 当用户名命中“内建认证前缀规则”时，`authplugin` 对该用户返回 `DEFER`，内建认证会继续执行。
- `_` 前缀用户认证完全由内建认证负责。
- ACL 统一使用内建 `acl_file`，`authplugin` 不参与 ACL。

## 7. 验收清单

- `_` 前缀用户能通过 `password_file` 登录。
- 非 `_` 前缀用户能通过 `authplugin` 登录。
- `_` 前缀用户数据库认证不会被调用。
- ACL 生效：设备仅可访问自身主题。
- ACL 生效：运维用户能访问运维主题。
