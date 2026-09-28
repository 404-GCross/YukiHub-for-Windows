# 更新日志

本文件记录 YukiHub for Windows 自身的变更。
上游 LunaBox 的历史变更日志完整保留在 [CHANGELOG.upstream.md](CHANGELOG.upstream.md)（未作修改）。

版本号规则：`主版本.次版本.修订号`，与 git tag（`v*.*.*`）一致。

---

## 0.1.0（未发布，开发中）

首个开发版本。当前里程碑是"干净的 Windows-only 工程基线"，
产品界面尚未重建，因此不提供安装包。

### 新增

- 以 LunaBox v1.13.0 为基线建立硬分叉，并保留完整上游历史与版权声明
- 技术路线决策记录：`docs/decisions/0001-fork-lunabox-as-windows-baseline.md`
- 路线图：`docs/ROADMAP.md`
- 与 Android 版 YukiHub 的数据迁移设计：`docs/mobile-yukihub-migration.md`
- 分叉配置清单（仓库、凭据、签名、更新服务）：`docs/fork-setup.md`
- AGPL 合规材料：`NOTICE`、`docs/AGPL-COMPLIANCE.md`、
  `THIRD_PARTY_LICENSES.md`、`third_party/` 许可证文本
- CI 新增 `gofmt` 与 `go vet` 门禁

### 变更

- 品牌与身份：Go 模块名 `lunabox` → `yukihub`，应用标识 →
  `com.yukihub.desktop`，URL 协议 `lunabox://` → `yukihub://`，
  数据目录与数据库 `LunaBox` / `lunabox.db` → `YukiHub` / `yukihub.db`，
  CLI `lunacli` → `yukihubcli`，更新器命令同步更名
- 前端工作区包 `@lunabox/desktop-shell-*` → `@yukihub/desktop-shell-*`，
  Wails 生成绑定目录 `frontend/bindings/lunabox/` → `frontend/bindings/yukihub/`
- 构建期环境变量 `LUNABOX_*` → `YUKIHUB_*`
- User-Agent 改为 YukiHub 自有标识，不再沿用上游仓库地址
- 默认云备份后端由上游绑定的托管服务改为 WebDAV（用户自持存储）
- 版本号起点为 0.1.0，与上游版本线解耦
- 全仓库 Go 代码重新执行 `gofmt`（模块改名会影响导入排序）

### 修复

- CI 中 Go 测试此前实际只编译不执行（`go test -run '^$'`），现已改为真实执行
- 应用内更新检查不再默认请求上游更新服务地址
- `main.go` 的 `//go:embed` 移除对已删除 macOS 资源的引用
  （否则 main 包无法编译）
- 消除 `internal/utils/processutils` 中 `unsafe.Pointer` 的 uintptr 往返转换，
  `go vet ./...` 现在无任何告警

### 验证

- `gofmt -l .` 无输出；`go vet ./...` 无输出；`go build ./...` 通过
- `go test ./... -count=1`：29 个含测试的包全部通过，0 失败
- `cd updater && go test ./... -count=1`：通过
- 上游遗留的 114 个测试文件首次被真实执行，结果全绿

### 移除

- 源码中硬编码的上游 Hikarinagi OAuth Client ID（改为构建期注入，
  第三方凭据必须自行申请）
- 上游默认更新服务地址
- 上游的多语言 README（`README.zh-CN.md`、`README.ja.md`），
  改为 `README.md`（中文）与 `README.en.md`（English）；
  历史版本仍可在 git 历史中查阅
- 全部 macOS / iOS / Linux 平台代码、构建资源与发布链路（本项目仅面向 Windows）：
  - 64 个 Windows 构建不参与的 Go 文件（darwin / linux 实现与对应测试）
  - `build/darwin`、`build/ios`、`build/linux`、`lib/{linuxamd64,linuxarm64,macarm64}`
  - `scripts/build.sh`、`scripts/patch-wails-linux-tray.sh`
  - `release.yml` / `autobuild.yml` 中的 macOS 与 Linux 构建作业
  - `Taskfile.yml` 中的 darwin / linux 构建、打包与补丁步骤
- Wine / Proton / CrossOver 工具链（Windows 上不存在对应概念）：
  - Go：`internal/utils/protonutils`、`internal/service/compattools`、
    `internal/service/compatibility_tools.go`，以及 `IntegrationService` 的
    `GetLocalProtonTools`、`AppConfig` 的 6 个全局 Wine/CrossOver 字段
  - 前端：游戏启动面板的 Proton 工具发现与兼容层快捷工具、游戏设置面板的
    Wine / CrossOver / winetricks / protontricks 设置块、`wine_runner` 事件分支
  - 四语言文案清理 36 个孤儿键
- 共享代码中的非 Windows 死分支（23 个文件，净减 410 行）：
  - 收敛恒真的 `runtime.GOOS` 判断：协议解析的 `allowLaunch` 参数、`Frameless`、
    `ShouldQuit`、`isLaunchableEntry`、路径打开与路径比较等
  - 删除恒假分支：macOS 的 Wine 前置校验、portable 的 Linux 启动器路径、
    AppImage 协议修复、导入目录的 goos 参数、测试中的平台 skip
  - `gamehelper.IsMacAppBundlePath`（macOS .app 概念）及 6 处调用点
  - `internal/utils/tricksutils` 整包（上一轮移除 compattools 后已无调用者，
    因 Go 不检查未使用的包而被遗漏）

保留说明：游戏级的 `wine_runner` / `wine_args` / `wine_prefix` 属于导入与云同步的
数据契约，仍保留在数据模型与快照中；Steam 相关能力在 Windows 上有效，全部保留。

### 已知问题

- 游戏级 `wine_runner` / `wine_args` / `wine_prefix` 在 Windows-only 语境下的
  存废待复核。它们属于导入与云同步的数据契约，删除是数据语义变更，
  需与 Android 版按 `docs/mobile-yukihub-migration.md` 两端评审
- 发布流水线 `release.yml` 依赖 SignPath 的代码签名资格。该资格属于上游项目，
  不随代码转移，YukiHub 需自行申请或改用自有证书，否则签名与更新校验环节会失败
- 界面与领域模型仍为上游形态
- 应用图标、界面插画、截图仍为上游占位素材
