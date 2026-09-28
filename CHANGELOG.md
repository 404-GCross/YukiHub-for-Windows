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

### 已知问题

- Go 测试尚未在真实环境中执行过，可能包含失效用例；在测试全绿之前不做平台代码裁剪
- macOS / iOS / Linux 平台代码与 CI 矩阵尚未移除
- 界面与领域模型仍为上游形态
- 应用图标、界面插画、截图仍为上游占位素材
