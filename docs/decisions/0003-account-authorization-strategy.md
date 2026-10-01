# ADR-0003：账号授权方式（Bangumi 个人令牌、Hikarinagi/NextMoe 原生 OAuth）

- 状态：已采纳
- 日期：2026-10-01
- 决策者：YukiHub 项目组
- 相关：[ADR-0002](0002-android-authoritative-and-sync-backend.md)、
  [fork-setup.md](../fork-setup.md)

## 背景

桌面端继承自上游 LunaBox 的第三方账号授权，与 Android 版的做法差得很远：

- 上游对 Bangumi / Hikarinagi 都要求**构建时注入 OAuth 凭据**
  （`YUKIHUB_BANGUMI_CLIENT_ID` / `_SECRET`、`YUKIHUB_HIKARINAGI_CLIENT_ID`）。
  本地或自建构建不注入时，点「去授权」直接报「请在构建时通过 XXX 注入」。
- Android 版（YukiHub 手机版）不是这样：
  - **Bangumi（含镜像）**：用户在设置里粘贴个人 Access Token，**不做 OAuth**；
  - **Hikarinagi**：OAuth（OIDC + PKCE，public client，无 secret）；
  - **NextMoe（未萌 / 鲲）**：OAuth（public client + PKCE，无 secret）；
  - 另有一套自建账号（`yukihub.zh.kg`，邮箱 / 密码 + 云同步），
    与第三方授权无关，桌面端尚未接入。

目标：与手机版对齐，用户**不需要为 Bangumi 单独申请 OAuth 应用**。

## 决策

### 1. Bangumi 与镜像改为个人 Access Token

桌面端后端的「legacy token」通路**本来就存在**，只是被 OAuth 流程盖住了：

- `config.BangumiAccessToken` 有值、`BangumiRefreshToken` 为空时，
  `getValidAccessToken` 直接把它当 Bearer token 用；
- `buildAuthStatusLocked` 也把它判为已授权（`legacy_token` 标志）。

因此**不需要任何 OAuth 应用**：用户在 bgm.tv 个人设置里建一个个人令牌即可。

- 前端账户卡片把「去授权」按钮换成令牌输入框 + 保存。
- 令牌**不回显**；要换令牌走「断开」（清凭据）再填。
- 镜像站与主站**共用同一个 token**（原本就是）。
- 上游的 OAuth 实现（`StartAuth`、本地回调、refresh）保留在后端作为可选路径，
  但不再暴露给用户；`YUKIHUB_BANGUMI_CLIENT_ID` / `_SECRET` 不再是必需项。

### 2. Hikarinagi / NextMoe 保留原生 OAuth

两者都是 native public client：不需要 client_secret，也不需要自建后端中转。

- **NextMoe**：复用现役「YukiHub Android」客户端 id（`16cc...`），已硬编码，开箱可用。
- **Hikarinagi**：默认复用 Android 客户端 id（`hkn_...`），
  可用 `YUKIHUB_HIKARINAGI_CLIENT_ID` 构建注入覆盖。

#### scope 必须与「该 client 被授权的那一组」完全一致

OAuth 的 scope 不是想要就能要：服务端校验的是**这个 client 被授予了哪些 scope**，
多要一个就整条授权失败，而且用户只会看到浏览器里一页
`invalid_scope: requested scope is not allowed` —— YukiHub 这边**拿不到任何错误信息**。

上游 LunaBox 用的是 `openid catalog:full user:read status:write offline_access`，
那是配它自己申请的 client id 的。YukiHub 复用的 Android 客户端只被授权了
`openid user:read`，照抄上游那串必然被拒（这正是第一版 PC 端授权失败的原因）。

默认值因此取 Android 的那一组：

| scope | 作用 |
| --- | --- |
| `openid` | 必须，用于拿 id_token |
| `user:read` | 读 `/v3/user/me`（账号名、头像） |

元数据走的是**公开 API**（`api.hikarinagi.org/v3`），不需要额外 scope。

**相对的代价**（与上游那串比）：

- 没有 `status:write` → 「同步游戏状态到 Hikarinagi」会被服务端拒绝；
- 没有 `offline_access` → 服务端可能不下发 refresh token，
  access token 过期后需要用户重新授权。

想拿回这两项：先在 Hikarinagi 后台给应用加上对应权限，
再用 `YUKIHUB_HIKARINAGI_SCOPES` 注入覆盖（注入值优先于默认值）。
`internal/service/hikarinagi_service_internal_test.go` 里有一个测试钉住默认值，
防止以后有人又照抄上游那串。

### 3. 回调地址：桌面端走 loopback

- Android 用自定义 scheme：`yukihub://hikarinagi/callback`、`yukihub://oauth/callback`。
- 桌面端按 RFC 8252 用 loopback：`http://127.0.0.1:<固定端口>/callback`。
  端口是**固定的**（Bangumi 23679 / Hikarinagi 14791 / NextMoe 14792），
  因此 redirect_uri 确定，**注册一次长期有效**。
- 是否复用同一个 OAuth 应用，取决于各平台后台能否为一个应用登记**多个 redirect_uri**：
  - **能** → 复用（首选，少申请一次；token 按用户发放，与设备无关，不会串数据）；
  - **不能** → 新开一个桌面端应用，用构建变量注入新 client id。

## 后果

- 用户不需要为 Bangumi 申请任何东西，填个人令牌即可（与手机版一致）。
- Hikarinagi / NextMoe 要真正跑通，需在对应平台后台**登记桌面端 loopback 回调地址**。
- 上游 OAuth 代码成为未暴露的备用路径；若后续确定不再需要，可单独评估清理
  （会连带清理 `StartAuth` 绑定、CI 变量与 `version` 中的凭据字段）。
