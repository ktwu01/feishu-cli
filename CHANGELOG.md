# Changelog

所有重要的项目变更都会记录在此文件。

版本格式：[MAJOR.MINOR.PATCH](https://semver.org/lang/zh-CN/)

## [v1.18.0] - 未发布

### Breaking Changes — OAuth 认证全面对齐官方 lark-cli

彻底删除 Authorization Code Flow，只保留 Device Flow（RFC 8628）。本地桌面、SSH 远程、容器、CI 全环境统一使用同一条命令，**无需任何重定向 URL 白名单配置**。

#### 删除的 flag

`auth login` 命令删除以下 flag：

- `--manual` — SSH 远程手动粘贴回调模式（Device Flow 下 SSH 和本地一视同仁）
- `--no-manual` — 强制本地回调模式（本地回调 HTTP server 已移除）
- `--port` — 本地回调端口（不再需要）
- `--print-url` — 非交互两步式第一步（改用 `--no-wait` + `--device-code` 对齐官方）
- `--method` — 授权方式选择（Device Flow 是唯一方式）
- `--scopes` — 请求 OAuth scope（飞书 token v2 端点实际忽略此参数，返回应用预配置的全部 scope）

#### 删除的子命令

- `auth callback <url> --state <state>` — Authorization Code Flow 换 token 专用，整体删除

#### 删除的代码

- `internal/auth/oauth.go`：`Login` / `loginLocal` / `loginManual` / `buildAuthURL` / `GenerateAuthURL` / `ParseCallbackURL` / `ExchangeToken` 等函数
- `internal/auth/browser.go`：`isLocalEnvironment()` 函数（曾在 macOS 上无条件返回 true 导致 SSH 远程 Darwin bug）
- `cmd/auth_callback.go`：整个文件

#### 修复的 bug

- **Issue #95**：飞书错误码 20029（重定向 URL 有误）。根因是 Authorization Code Flow 需要用户在飞书开放平台配置 `http://127.0.0.1:9768/callback` 白名单，Device Flow 直接绕过此要求
- **Darwin SSH bug**：`isLocalEnvironment()` 在 macOS 上无条件返回 `true`，SSH 到 Mac 服务器时错误走本地回调模式会 2 分钟超时失败。已通过删除该函数消除

### 新增 — `auth login` 的 JSON 事件流模式

- **`auth login --json`**：阻塞轮询 + JSON 事件流输出到 stdout。AI Agent 推荐配合 Claude Code 的 `run_in_background=true` 使用
  - 首次输出：`{"event":"device_authorization","verification_uri":"...","verification_uri_complete":"...","user_code":"...","device_code":"...","expires_in":240,"interval":5}`
  - 成功输出：`{"event":"authorization_success","expires_at":"...","refresh_expires_at":"...","scope":"..."}`

- **`auth login --no-wait --json`**：两步模式第一步。只请求 `device_code` 并立即输出 JSON，不启动轮询。适合 AI Agent 希望把"请求"和"轮询"拆到两次独立 Bash 调用的场景

- **`auth login --device-code <code> --json`**：两步模式第二步。用已有的 `device_code` 继续轮询直到授权完成

### 新增 — `auth check` 子命令

预检当前 Token 是否包含指定 scope，专为 AI Agent 在执行业务命令前做前置判断而设计：

```bash
feishu-cli auth check --scope "search:docs:read"
feishu-cli auth check --scope "search:docs:read im:message:readonly"
```

输出 JSON：

```json
{
  "ok": true,
  "granted": ["search:docs:read"],
  "missing": null
}
```

或失败情况：

```json
{
  "ok": false,
  "error": "not_logged_in",
  "missing": ["search:docs:read"],
  "suggestion": "feishu-cli auth login"
}
```

退出码 0 = 满足，非 0 = 缺少或未登录，AI Agent 可直接分支。

参考官方 lark-cli 的 `auth check` 实现（`cli/cmd/auth/check.go`）。

### 不变

- **Token 存储格式**：`~/.feishu-cli/token.json` 仍是明文 JSON，数据结构完全兼容。升级后**不需要重新登录**
- **Token 自动刷新**：`ResolveUserAccessToken()` 路径和 `RefreshAccessToken()` 逻辑保持不动，access_token 过期时用 refresh_token 自动刷新
- **`config create-app`** 和 **`config add-scopes`** 命令完全不变（它们本来就用 Device Flow）
- **`auth status`** / **`auth logout`** 行为不变
- **所有业务命令**（doc/msg/search/wiki/task/calendar/...）行为不变

### 迁移指引

#### 人类用户

无需任何迁移。一条命令通吃所有场景：

```bash
feishu-cli auth login
```

本地桌面会自动开浏览器，SSH 远程需要手动复制 stderr 里的链接在本机浏览器打开，一模一样的命令。

#### AI Agent / 脚本用户

旧的两步式：
```bash
feishu-cli auth login --print-url --scopes "..."
feishu-cli auth callback "<回调URL>" --state "<state>"
```

迁移为以下**任一**方案：

**方案 A（推荐）**：阻塞 + 后台运行：
```bash
# run_in_background=true
feishu-cli auth login --json
# 读 stdout 第一行拿 verification_uri_complete，展示给用户
# 等后台进程退出，读第二行 stdout 拿 authorization_success
```

**方案 B**：对齐官方 lark-cli 的两步模式：
```bash
# 第一步
feishu-cli auth login --no-wait --json  # → device_code JSON
# 把链接展示给用户等待授权
# 第二步
feishu-cli auth login --device-code <code> --json  # → authorization_success
```

#### CI / 无头脚本

**Authorization Code Flow 本来就无法无头完成**（需要浏览器授权），Device Flow 同样需要人类介入一次。如果 CI 需要 User Token，应该预先在本地通过 `auth login` 拿到 token.json 然后把它作为 secret 部署到 CI 环境，**不需要任何迁移**。

### 详细对比

| 方面 | v1.17.0 及以前 | v1.18.0 |
|---|---|---|
| OAuth Flow | Authorization Code Flow（默认）+ Device Flow（`--method device`） | 仅 Device Flow |
| 子命令 | `login` / `callback` / `status` / `logout` | `login` / `check` / `status` / `logout` |
| `auth login` 的 flag | `--port` / `--manual` / `--no-manual` / `--print-url` / `--scopes` / `--method` | `--json` / `--no-wait` / `--device-code` |
| 重定向 URL 白名单 | 必须（Authorization Code Flow 前置条件） | 不需要 |
| SSH 远程支持 | 要么手动粘贴（`--manual`）要么非交互两步（`--print-url`） | 一条命令通吃 |
| AI Agent 非交互方案 | `--print-url` + `auth callback` | `--json` + `run_in_background` 或 `--no-wait` / `--device-code` |
| `offline_access` 注入 | 用户手动通过 `--scopes` 传 | CLI 强制注入，用户无需操心 |
| scope 预检 | 手动解析 `auth status` JSON 的 scope 字段 | `auth check --scope "..."` |

---

更早的版本请参考 [GitHub Releases](https://github.com/riba2534/feishu-cli/releases)。
