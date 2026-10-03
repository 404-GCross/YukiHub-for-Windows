# YukiHub 账号服务 API 契约（服务端 → 客户端）

> 本文是**服务端下发的接口契约**，由账号服务维护方提供给各端客户端参考。
> 桌面端实现见 `internal/service/yukihubaccount/client.go`，
> 契约相关回归测试见 `client_test.go`。
> 客户端侧只需遵守本文，**任何与服务端行为不一致的改动都应先回来核对本文**。

> 交给电脑端 AI 执行。本文档只讲**客户端要做什么**，不涉及服务器改动。
> 服务端已经改好了，PC 端目前还没发布，所以趁现在改，避免以后重复踩坑。

---

## 一、为什么必须改

旧版客户端把**账号密码放在 URL query string** 里，例如：

```
GET https://yukihub.zh.kg/api/auth/login?email=xxx&password=yyy
```

问题：URL 会被**服务器访问日志、代理日志、浏览器历史、Referer** 原样记录，密码等于明文外泄。

**服务端目前仍然兼容 GET，只是为了不弄挂已经发布的 Android 老版本。** PC 端未发布，没有兼容包袱，请直接按 POST 实现。等 Android 更新后，服务端会删掉 GET 分支，届时还用 GET 的客户端会直接失效。

---

## 二、硬性规则（违反任何一条都算做错）

1. **登录、注册、发验证码、重置密码、刷新 Token 一律用 POST + JSON body**，绝不把密码或验证码放进 URL。
2. 请求头固定两个：
   - `Content-Type: application/json`
   - 需要登录态的接口加 `Authorization: Bearer <accessToken>`
3. **所有响应都是 JSON**。如果拿到以 `<` 开头的 body（HTML），说明请求打到了非 API 地址或服务器异常，**不要尝试解析**，直接提示"服务异常，请稍后重试"。
4. **业务错误不要按 404 判断**。本项目里 404 只代表"接口地址不存在"，业务错误统一是 400 / 401 / 403 / 429（详见第五节）。历史代码里曾有"帖子不存在返回 404"的写法，现已全部改为 400。
5. **收到 429 不要立刻重试**，按提示信息引导用户等待。

---

## 三、接口契约

统一前缀：`https://yukihub.zh.kg/api`（以线上为准，不要用其它域名拼接）
请求头 `Content-Type: application/json`。
跨域：服务端已返回 `Access-Control-Allow-Origin: *`，允许 `Content-Type`、`Authorization`，并处理 `OPTIONS` 预检，浏览器环境可直接调用。

### 3.1 登录

```
POST /api/auth/login
Body: { "email": "...", "password": "..." }
```

| 状态码 | body | 含义 |
|---|---|---|
| 200 | `{accessToken, refreshToken, user:{id,uid,nickname,email,avatarUrl,signature,kungalBound,hikarinagiBound}}` | 成功 |
| 400 | `{error:"请填写邮箱和密码"}` | 缺参数 |
| 400 | `{error:"该邮箱尚未注册"}` | 账号不存在（**以便区分场景，方便用户**） |
| 401 | `{error:"密码错误"}` | 密码错 |
| 403 | `{error:"账号已被封禁：<理由>"}` 或 `{error:"该账号已被禁用，请联系管理员"}` | 被封禁 |
| 429 | `{error:"登录尝试次数过多，请 N 秒后再试"}` | 触发限速 |

**限速规则**：同一邮箱或同一 IP，15 分钟内连续失败 8 次 → 锁定 10 分钟。锁定期间即使密码正确也会被拒。登录成功会清零失败计数。
错误提示请**原样展示** `error` 文案（含剩余秒数），不要自己改写。

### 3.2 注册

```
POST /api/auth/register
Body: { "email": "...", "password": "...", "nickname": "...", "code": "123456" }
```

| 状态码 | body | 含义 |
|---|---|---|
| 201 | `{accessToken, refreshToken, user:{...}}` | 注册成功并直接登录 |
| 400 | `请填写邮箱、密码、昵称和验证码` / `请输入有效的邮箱地址` / `密码至少需要6位` / `昵称需要2-20个字符` / `昵称包含不允许的字符` | 参数问题 |
| 400 | `验证码错误，还可尝试 N 次` / `验证码错误或已过期` | 验证码问题 |
| 400 | `该邮箱已被注册` | 邮箱已存在 |
| 429 | `验证码错误次数过多，请重新获取验证码` 或 `注册尝试过于频繁，请 N 秒后再试` | 限速 |

规则提醒：
- 昵称 2–20 字符，不能含 `< > { } [ ] \ /`。
- 密码至少 6 位。
- 单个验证码最多错 5 次，超了要重新发码。
- 同一 IP 每小时最多 10 次注册提交（含失败）。

### 3.3 发送注册验证码

```
POST /api/auth/send_code
Body: { "email": "..." }
```
200 → `{success:true, message:"验证码已发送至邮箱"}`

### 3.4 找回密码：发码 + 重置

```
POST /api/auth/send_reset_code
Body: { "email": "..." }
```
200 → `{success:true, message:"如果该邮箱已注册，验证码将发送至邮箱"}`
> 注意：未注册邮箱也返回这个成功信息（防账号枚举）。**不要根据返回值判断邮箱是否存在。**

```
POST /api/auth/reset_password
Body: { "email": "...", "code": "123456", "password": "新密码" }
```
200 → `{success:true, message:"密码已重置，请用新密码登录"}`；400 / 429 同上风格。

**发码限速（三档，任一命中返回 429）**：
| 条件 | 文案 |
|---|---|
| 同邮箱 60 秒内重复请求 | `验证码发送过于频繁，请60秒后重试` |
| 同邮箱 24 小时内超过 3 次 | `该邮箱今日验证码发送次数已达上限（3次），请24小时后再试` |
| 同 IP 1 小时内超过 5 次 | `请求过于频繁，请1小时后再试` |

注册码与重置码**共用**这三档额度。

### 3.5 刷新 Token

```
POST /api/auth/refresh
Body: { "refreshToken": "..." }
```

| 状态码 | body | 含义 |
|---|---|---|
| 200 | `{accessToken, refreshToken, user:{...}}` | 换新成功（**refresh 也会轮换，请保存新的那个**） |
| 400 | `{error:"缺少 refreshToken"}` | 没传 |
| 401 | `{error:"refreshToken 无效或已过期"}` | 需要重新登录 |
| 403 | `{error:"该账号已被禁用，请联系管理员"}` | 封禁 |
| 405 | `{error:"Method Not Allowed"}` | 用了 GET |

### 3.6 获取当前用户（示例：需要登录态的接口怎么调）

```
GET /api/me
Header: Authorization: Bearer <accessToken>
```
200 → 用户资料（含等级、萌萌点、头像框等字段）
401 → `{error:"缺少 Authorization 头"}` 或 `{error:"accessToken 无效或已过期"}`

其它业务接口（好友、群组、社区帖子、签到、商店、云同步等）全部是同样的鉴权方式：
**401 表示 token 问题**，`{error, code}` 里 `code` 可能是 `ACCOUNT_DISABLED`、`LOGIN_REQUIRED`。

---

## 四、Token 生命周期与错误处理流程

- `accessToken` 有效期 **24 小时**；`refreshToken` 有效期 **30 天**。
- 建议流程：
  1. 请求带 accessToken；
  2. 收到 **401** 且本次确实带了 token → 调 `/api/auth/refresh` 换新 token → **原请求重试一次**（只重试一次，别循环）；
  3. refresh 也返回 401/403 → 清空本地 token，回到登录界面；
  4. **网络异常（超时/断连）不要清 token**，提示重试即可，否则会莫名其妙被登出。
- 并发多个请求同时 401 时，refresh **只发一次**，其余请求等结果（避免刷新风暴）。

---

## 五、状态码语义总表（PC 端按这个写分支）

| 状态码 | 含义 | 客户端动作 |
|---|---|---|
| 200 / 201 | 成功 | 正常处理 |
| 400 | 业务/参数错误 | 显示 `error` 文案 |
| 401 | 未登录或 token 失效 | 走第四节的刷新流程 |
| 403 | 权限不足/账号被封禁 | 显示 `error`，不要重试 |
| 405 | 请求方法不对（如用 GET 调了 POST 接口） | 这是客户端 bug，去检查 |
| 429 | 触发限速 | 显示 `error`（含等待时间），**不要自动重试** |
| 500 | 服务端异常 | 提示稍后重试 |
| 404 | **仅表示接口地址不存在** | 说明 URL 写错了 |

> 唯一例外：`/api/sync/download.php` 用 404 表示"云端还没有同步数据"。如果 PC 端要实现云同步，这个接口要把 404 当作正常情况处理。

---

## 六、改造清单

在 PC 端代码里搜索以下字符串，逐个改成 POST + JSON body：

- [ ] `/auth/login`（含 `?email=` 或 `?password=` 拼接的写法）
- [ ] `/auth/register`
- [ ] `/auth/send_code`
- [ ] `/auth/send_reset_code`
- [ ] `/auth/reset_password`（应该已经是 POST，确认一下）
- [ ] `/auth/refresh`（必须是 POST）
- [ ] 其它所有 `?` 拼接敏感字段的请求（password / code / token）

同时确认：

- [ ] 密码、验证码、token **不出现在 URL、日志、错误上报**里
- [ ] 解析响应前先判断 body 是不是 HTML（防御）
- [ ] 401 自动刷新只重试一次，并且并发时只发一次 refresh
- [ ] 429 不做自动重试
- [ ] 所有 `error` 文案原样展示给用户

---

## 七、验收自测（可直接复制到终端）

```bash
BASE=https://yukihub.zh.kg/api

# 1. 登录成功（换成真实账号）
curl -sS -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"yourpassword"}'

# 2. 未注册邮箱 → 期望 400 + {"error":"该邮箱尚未注册"}
curl -sS -o /dev/null -w '%{http_code}\n' -X POST "$BASE/auth/login" \
  -H 'Content-Type: application/json' -d '{"email":"nobody_zzz@example.com","password":"x"}'

# 3. 密码错 → 期望 401 + {"error":"密码错误"}
# 4. 刷新 Token → 期望 200，且返回新的 accessToken / refreshToken
curl -sS -X POST "$BASE/auth/refresh" -H 'Content-Type: application/json' \
  -d '{"refreshToken":"<上一步拿到的>"}'

# 5. 带 token 取资料
curl -sS "$BASE/me" -H "Authorization: Bearer <accessToken>"
```

自测通过标准：
1. 全部请求的 `Content-Type` 都是 JSON，**没有任何一次返回 HTML**；
2. 上面第 2、3 条的状态码分别是 400、401，且 `error` 文案能正常显示；
3. 密码/验证码一次都没出现在 URL 里（可用抓包或日志确认）。

---

## 八、不要做的事

- ❌ 不要再用 GET 传 `password` / `code` / `token`
- ❌ 不要把密码写进日志、崩溃上报、URL 参数、Referer
- ❌ 不要按 404 判断业务错误（业务错误是 400）
- ❌ 不要遇到 429 就自动重试（会被锁更久）
- ❌ 不要在网络超时时清空登录态（只有 refresh 明确 401/403 才清）
- ❌ 不要假设响应一定合法：先判 HTML、再解析 JSON、最后做兜底

---

## 九、需要向项目维护者确认的事

1. PC 端是否已经实现了"记住登录态"（access + refresh 的存储位置）？没有的话本次一起做。
2. PC 端是否需要云同步（`/api/sync/*`）？如果要，注意 3.4 节 404 的例外。
3. PC 端面向浏览器还是原生窗口？浏览器环境直接用 fetch 即可（服务端已开 CORS）；原生环境注意自己处理 TLS 与超时。
