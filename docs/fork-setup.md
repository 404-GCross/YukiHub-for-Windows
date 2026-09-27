# 分叉配置清单

本仓库是 LunaBox 的硬分叉。代码层面的品牌替换已经完成，但**有些东西无法靠改代码解决**，
必须在真正发布前逐项配置。本文件就是这份清单。

## 1. 仓库与身份

| 项目 | 当前占位值 | 需要确认 |
| --- | --- | --- |
| 仓库地址 | `https://github.com/xm486/YukiHub` | 确认桌面版是复用该仓库还是新建独立仓库；若新建，需全局替换 |
| 应用标识 | `com.yukihub.desktop` | 确认命名空间符合预期（安装路径、单实例 ID、注册表协议键均依赖它） |
| URL 协议 | `yukihub://` | 若与 Android 版协议冲突需重新选定 |
| 用户数据目录 | `%APPDATA%\YukiHub`、`%LOCALAPPDATA%\YukiHub` | 确认与 Android 版不冲突（Android 为应用私有目录，不冲突） |

涉及文件中已统一使用该标识，如需变更请全局搜索：
`com.yukihub.desktop`、`yukihub://`、`xm486/YukiHub`。

## 2. 第三方服务凭据（必须自行申请）

上游硬编码了自家的 Hikarinagi OAuth Client ID，本仓库已将其移除。
以下凭据全部需要 YukiHub 自行申请，并通过 CI Variables / Secrets 或本地 `.env.build` 注入：

| 服务 | 变量 | 用途 |
| --- | --- | --- |
| Bangumi | `YUKIHUB_BANGUMI_CLIENT_ID` / `_SECRET` | 账号授权、状态回写 |
| Hikarinagi | `YUKIHUB_HIKARINAGI_CLIENT_ID` / `_SECRET` | 账号登录、元数据 |
| TouchGAL | `YUKIHUB_TOUCHGAL_TOKEN` | 元数据接口 |
| Umbra | `YUKIHUB_UMBRA_CLIENT_ID` / `YUKIHUB_UMBRA_REGISTRATION_TOKEN` | 可选云备份后端 |
| 更新服务 | `YUKIHUB_UPDATE_SERVICE_URL` | 应用内更新检查地址 |

未配置时相关功能应给出"未配置"提示，而不是回退到他人的应用身份。

## 3. 代码签名

上游依赖 SignPath 的开源项目免费签名，**该资格属于上游项目，不随代码转移**。

需要做其中一项：

- 自行向 SignPath 申请开源签名资格（需证明仓库归属与开源属性）
- 或购买普通代码签名证书（OV / EV）

无论选哪种，都要同步检查：

- `updater/updateutils/signature_windows.go` 的 Authenticode 校验逻辑
- `.github/workflows/release.yml` 中的签名步骤与相关 Variables
- NSIS 安装器的签名配置

## 4. 更新服务

上游的更新链路包含：S3 兼容对象存储 + Cloudflare Worker（`update-server/`）+ 各 channel 清单。

需要决定：

- **方案 A**：自建 S3 兼容存储 + 自建清单服务，复用现有 `updater/` 与 CI 流程
- **方案 B**：暂不做应用内更新，移除 `update-server/` 与相关 CI，仅通过 Releases 分发

注意：`internal/service/update_service.go` 的默认更新地址列表已置空，
在配置 `YUKIHUB_UPDATE_SERVICE_URL` 之前，更新检查会直接跳过（不会请求任何第三方域名）。

## 5. CI Variables / Secrets

发布相关工作流（`release.yml`、`autobuild.yml`、`update-test.yml`）需要以下配置：

**Variables**

- `UPDATE_PUBLIC_BASE_URL`
- `UPDATE_S3_ENDPOINT`、`UPDATE_S3_REGION`、`UPDATE_S3_BUCKET`
- `YUKIHUB_BANGUMI_CLIENT_ID`、`YUKIHUB_HIKARINAGI_CLIENT_ID`、`YUKIHUB_UMBRA_CLIENT_ID`
- `SIGNPATH_*`（若使用 SignPath）

**Secrets**

- `UPDATE_S3_ACCESS_KEY_ID`、`UPDATE_S3_SECRET_ACCESS_KEY`
- `YUKIHUB_BANGUMI_CLIENT_SECRET`、`YUKIHUB_TOUCHGAL_TOKEN`、`YUKIHUB_UMBRA_REGISTRATION_TOKEN`
- `SIGNPATH_API_TOKEN`（若使用 SignPath）

这些工作流目前仅支持手动触发或 tag 触发，在配置完成前不会产生失败的自动构建。

## 6. 品牌素材（待替换）

以下位置仍是上游占位素材，需要替换为 YukiHub 素材：

| 位置 | 说明 |
| --- | --- |
| `build/appicon.png` | 应用图标（源图） |
| `build/windows/icon.ico` | Windows 可执行文件与安装器图标 |
| `build/windows/tray.png` | 系统托盘图标 |
| `frontend/src/assets/branding/brand-1.webp`、`brand-2.webp` | 新增/导入弹窗中的品牌插画 |
| `frontend/src/assets/branding/appicon.png`、`topbar-title.png` | 侧边栏与顶栏品牌图 |
| `screenshot/**` | README 截图与宣传图（当前为上游界面截图） |

## 7. 版本号

- 版本号由构建期从 git tag 注入（`v0.1.0` → `0.1.0`）。
- 需要同步维护的地方：
  - `build/config.yml` 的 `info.version`
  - `build/windows/info.json`、`build/windows/nsis/wails_tools.nsh`
  - `sync/version.json`（发布工作流会校验其 `version` 与 tag 一致）
- `scripts/update-build-assets.*` 可在本地批量同步这些文件。

## 8. 本地开发环境

- Go 版本见 `go.mod`（当前 1.27.1）
- Node.js 24 + pnpm 9（前端）
- Wails v3 CLI，版本必须与 `go.mod` 中的 `github.com/wailsapp/wails/v3` 完全一致
  （CI 会用 `go list -m` 自动安装同版本）

常用命令：

```bash
# 安装前端依赖
cd frontend && pnpm install

# 生成 Wails 绑定（修改后端 service 方法签名后必须执行）
wails3 generate bindings -clean=true -ts

# 本地开发运行
wails3 dev -config ./build/config.yml -port 9245

# 检查
gofmt -l . && go vet ./... && go test ./... -count=1

# 构建
wails3 build
```

> 注意：仓库目录名包含空格（`YukiHub for Windows`），部分脚本对含空格路径敏感。
> 如果构建脚本报路径错误，可把仓库检出到无空格路径下（例如 `D:\work\yukihub`）。
