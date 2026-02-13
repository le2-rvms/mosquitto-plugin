# AuthPlugin 与内建认证/ACL 混合方案（草案）

本文档是独立草案，等待开发完成后合并至主文档。

## 1. 目标

- 同时支持内建认证（`password_file`）与 `authplugin` 数据库认证。
- 通过用户名前缀分流：
  - 以 `_` 开头的用户名：走内建认证。
  - 非 `_` 开头的用户名：走 `authplugin`。
- ACL 采用 Mosquitto 内建 `acl_file`。

## 2. 认证分流规则

### 2.1 认证流程

在 `MOSQ_EVT_BASIC_AUTH` 中：

- `username` 为空或以 `_` 开头：`authplugin` 返回 `MOSQ_ERR_PLUGIN_DEFER`。
- 其它用户名：`authplugin` 执行数据库认证并返回 `MOSQ_ERR_SUCCESS` 或 `MOSQ_ERR_AUTH`。

**影响：**

- `authplugin` 不再为 `_` 前缀用户提供“最终失败”结果。
- 若要记录这类请求，需将结果标记为 `defer`，而非 `fail`。
- 把“是否允许空用户名/匿名”交给内建认证决定。如果你配置 `allow_anonymous false`，内建也会拒绝，效果是一样的；如果你将来允许匿名，插件不会阻断。

### 2.2 password_file 示例

```conf
allow_anonymous false
password_file /mosquitto/config/password_file
```

用户名示例：

- `_ops`、`_local`：走内建认证。
- `device123`、`app01`：走 `authplugin`。

### 2.3 password_file 维护方式

使用 `mosquitto_passwd` 管理密码文件。建议在宿主机或容器内执行。

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

## 3. ACL 文档

ACL 采用内建 `acl_file`。详细规则与示例见：`docs/acl-file.md`。

示例（含插件）：

```conf
allow_anonymous false
password_file /mosquitto/config/password_file
acl_file /mosquitto/config/acl_file

plugin /mosquitto/build/auth-plugin
plugin_opt_pg_dsn postgres://user:pass@127.0.0.1:5432/mqtt?sslmode=disable
```

## 4. 变更影响与注意事项

- 当用户名命中“内建认证前缀规则”时，`authplugin` 对该用户返回 `DEFER`，因此不会产生最终失败结果，内建认证会继续执行。
- `_` 前缀用户认证完全由内建认证负责。
- ACL 统一使用内建 `acl_file`，`authplugin` 不参与 ACL。

## 5. 验收清单

- `_` 前缀用户能通过 `password_file` 登录。
- 非 `_` 前缀用户能通过 `authplugin` 登录。
- `_` 前缀用户数据库认证不会被调用。
