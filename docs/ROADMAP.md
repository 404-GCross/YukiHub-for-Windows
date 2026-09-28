# YukiHub for Windows 路线图

阶段划分以"可验收的结果"为单位，不给日期承诺。上一个阶段未通过验收，不进入下一个阶段。

## 现状（阶段 0 已完成）

- [x] 完成三条技术路线的评估并形成决策：[ADR-0001](decisions/0001-fork-lunabox-as-windows-baseline.md)
- [x] 以 LunaBox v1.13.0 建立硬分叉基线，仓库纳入版本控制
- [x] 完成去品牌化（模块名、应用标识、协议、数据目录、CLI、绑定路径、构建变量、安装器）
- [x] 移除上游硬编码凭据与指向上游更新服务/托管云的默认地址
- [x] 补齐 AGPL 合规材料（`NOTICE`、`THIRD_PARTY_LICENSES.md`、`third_party/`、合规说明）
- [x] 修复 CI 中"只编译不执行测试"的问题，新增 `gofmt` / `go vet` 门禁
- [x] 全仓库 Go 代码 `gofmt` 通过

已知遗留问题（进入阶段 1 前必须处理）：

- Go 测试从未被真实执行过，可能包含**已经失效的用例**，需要逐条修复或删除。
  在测试全绿之前，任何裁剪都缺少安全网。
- 产品界面、领域模型仍是上游形态，尚未替换为 YukiHub 形态。

---

## 阶段 1：可构建、可安装、测试可信

目标：得到一个"干净的 Windows-only 工程"——能构建、能安装、测试真实运行且通过。

验收标准：

1. `gofmt -l .` 输出为空；`go vet ./...` 无错误。**（已验证通过）**
2. `go test ./... -count=1` 与 `cd updater && go test ./... -count=1` 全部通过。**（已验证通过）**
   - 失效用例允许删除，但必须在提交信息中说明原因；不允许跳过或标记为 Skip 来"凑绿"。
3. `wails3 build` 在本机与 CI 上成功产出可执行文件。
4. NSIS 安装包可以完成"安装 → 启动 → 卸载"，且卸载后用户数据目录按预期处理。
5. macOS / iOS / Linux 相关代码与 CI 矩阵移除完毕。**（已完成，见下方）**
6. 界面上的"通用跨平台"表述与残留的上游素材占位清理完毕。

### 阶段 1 验证记录（2026-09-28）

本机装上 MinGW-w64 并启用 `CGO_ENABLED=1` 后完成了此前无法执行的验证：

| 检查 | 结果 |
| --- | --- |
| `gofmt -l .` | 无输出 |
| `go vet ./...` | 无输出（修复了 1 处上游遗留的 unsafeptr 告警） |
| `go build ./...` | 通过（含 DuckDB 的 CGO 与 Wails） |
| `go test ./... -count=1` | 29 个含测试的包全部 `ok`，0 个 FAIL |
| `cd updater && go test ./... -count=1` | 通过 |

重要结论：**上游那 114 个测试文件第一次被真实执行，结果是全绿的。**
之前"可能已经失效"的担心不成立，平台裁剪也已有编译与测试双重保障。

注意：`go build` 需要 `frontend/dist` 存在（`main.go` 有 `//go:embed all:frontend/dist`）。
未构建前端时可在本地建一个占位目录临时绕过，该目录已在 `.gitignore` 中。

已完成的部分：

- [x] 删除 64 个 Windows 构建不参与的 Go 文件（darwin / linux 实现与对应测试）
- [x] 删除 `build/darwin`、`build/ios`、`build/linux` 与 `lib/{linuxamd64,linuxarm64,macarm64}`
- [x] 删除 `scripts/build.sh`、`scripts/patch-wails-linux-tray.sh`
- [x] `release.yml` / `autobuild.yml` 移除 macOS 与 Linux 构建作业，产物断言收敛为 4 个 Windows 产物
- [x] `Taskfile.yml` 移除 darwin / linux 分支，`.gitignore` 清理失效路径
- [x] `docs/workflow.md` 移除 Linux 渲染验证与 macOS 透明窗口两节

Wine / Proton / CrossOver 工具链清理（2026-09-28 完成）：

- [x] 删除 `internal/utils/protonutils/` 整包
- [x] 删除 `internal/service/compattools/` 整包
- [x] 删除 `internal/service/compatibility_tools.go`
      （`GameCompatibilityToolsInfo`、`GetGameCompatibilityTools`、`OpenGameCompatibilityTool`）
- [x] `integration_service.go` 移除 `LocalProtonTool` / `localProtonToolsFromUtils` /
      `GetLocalProtonTools`
- [x] `appconf.AppConfig` 移除 `WineRunnerPath`、`WinePrefix`、`WinetricksPath`、
      `ProtontricksPath`、`CrossOverRunnerPath`、`CrossOverBottle` 六个字段，
      删除 `MigrateLegacyCompatibilityConfig` 与 `wine_detect_*.go`
- [x] 删除 `launcher/strategy.go` 中已无调用者的 `newStrategyError`（Wine 缺配置错误路径的遗留）
- [x] 前端移除对应入口：`GameLaunchPanel` 的 Proton 工具发现与兼容层快捷工具面板、
      `GameSettingsPanel` 的 Wine / CrossOver / winetricks / protontricks 设置块、
      `useAppRuntimeEffects` 的 `wine_runner` 事件分支、`bindings/integration.ts` 的包装函数
- [x] 四语言文案清理 36 个孤儿键，四份文件键结构保持一致

明确保留（有意为之，不是遗漏）：

- `models.Game` 上的游戏级 `wine_runner` / `wine_args` / `wine_prefix`，以及对应的
  数据库列与 `cloudsync` 快照字段。它们是**数据契约**的一部分：导入 Playnite /
  PotatoVN / Vnite 等来源时可能带上这些值，云同步与备份也依赖这些字段做往返。
  删除它们属于数据语义变更，需要按 `docs/mobile-yukihub-migration.md` 的要求两端评审，
  不属于本轮范围。
- Steam 相关能力（`GetGameSteamCompatibility`、`SetGameSteamCompatibilityTool`、
  `OpenGameSteamProtonPrefix`、`RestartSteamClient`）在 Windows 上仍然有效，全部保留。

剩余的部分：

- [x] 执行 `wails3 generate bindings -clean=true -ts` 重新生成绑定 —— **已完成**。
      处理 601 个包 / 24 个服务 / 240 个方法 / 11 个枚举 / 96 个模型；
      已删除的 Wine、Proton 符号全部清除，`appconf/models.ts` 减少 30 行，
      `pnpm run typecheck` 通过。
      （上一轮记为"上游依赖有问题"是**误判**：真实原因是本机 Go 模块缓存被
      "半截解压"污染——zip 完整、解压目录只写了一半，因此报错会在不同包之间跳。
      清理后 `go install wails3@v3.0.0-beta.24` 仅 47 秒完成。
      排查脚本见 `~/.workbuddy/tools/gocheck/check_modcache.py`。）
- [ ] 清除共享代码中的死分支：`gamehelper/dialog.go`、`portable_setup_service.go`
      （含 AppImage 集成）、`cli/start.go`、`cli/protocolcmd/protocol.go`、
      `config_game_library.go`、`import_service.go`、`launcher/detector.go` 中的
      `runtime.GOOS` 非 Windows 分支
- [ ] 复核游戏级 `wine_*` 字段在 Windows-only 语境下的存废（见上方"明确保留"说明）

## 阶段 2：领域模型统一与双向数据迁移

目标：桌面版与 Android 版之间的数据可以双向流动，且语义明确。

验收标准：

1. 统一的游戏领域模型落地：三语标题（中文 / 原名 / 罗马字）、别名、NSFW 标记、
   五态游玩状态、标签、封面来源、元数据来源。
2. Android 版备份（schema 5 JSON）导入桌面版：万级样例 0 丢失、0 重复。
3. 桌面版数据导出为 Android 版可识别的备份格式（可被 Android 版导入）。
4. 游玩记录的合并语义有测试覆盖：
   - 总时长取最大值而非覆盖
   - `playtime_reset_at` 之后的历史会话不再计入
   - 会话 UUID 幂等，重复导入不产生重复记录
   - 时区与毫秒/秒单位换算正确
5. 路径模型差异有明确处理：Android 的 SAF 树 URI 与 Windows 绝对路径之间的映射与
   不可达降级策略。
6. 冲突裁决只有一套实现（不允许桌面版与 Android 版各有一套合并算法长期并存）。

设计细节见 [mobile-yukihub-migration.md](mobile-yukihub-migration.md)。

## 阶段 3：产品层重建

目标：界面与交互是 YukiHub，而不是"改了名字的 LunaBox"。

验收标准：

1. 首页 / 库 / 游戏详情 / 统计 / 设置五个主界面完成 YukiHub 化改造。
2. 上游素材（应用图标、界面插画、启动图、截图）全部替换为 YukiHub 素材。
3. 元数据来源合并为单一入口（Bangumi、VNDB、月幕、Hikarinagi、Steam 等），
   优先级与缓存策略统一。
4. 游玩启动与时长统计在 Windows 上按真实进程行为工作：
   - 启动、退出、崩溃、Switch 用户、开机自启场景均有验证
   - 后台/前台时长语义与 Android 版规则对齐
5. 同步协议落地：至少支持一种用户自持的同步后端（WebDAV 优先）。
6. 应用内"关于与开源许可"面板完成，展示版本、上游署名与 AGPL 声明。

## 阶段 4：Android 版差异化能力迁移

目标：把桌面版做成"手机版的桌面延伸"，而不是一个通用管理器。

按优先级：

1. **游玩记录与数据同步协议**（已在阶段 2 完成）
2. **多源元数据与本地缓存离线可用**
3. **AI 游玩报告**（复用上游已有的 AI 服务与防剧透配置）
4. **OCR 与多引擎翻译工作流**
   - 离线 PP-OCRv6 模型可复用（`assets/ppocrv6/`）
   - 需重做截图与悬浮层：Windows 用窗口截图而非 Android 无障碍/MediaProjection
5. **社区与好友功能**（REST 契约可复用，界面重做）
6. **音乐厅 / 大屏模式**（产品设计可参考 Android 版）

明确排除：

- Android 内置游戏引擎（Kirikiri、ONS、Tyrano、Artemis、PSP）
- 模拟器启动适配（Winlator、GameHub、盖世）
- Shizuku、SAF 镜像存档、触控光标注入、`.nomedia` 管理

## 暂缓：离线 3D 展厅

Android 版的 Three.js 离线展厅（`assets/exhibition/`，约 7200 行纯 Web）可近乎零改动地
由 WebView2 承载，但**本项目当前阶段不做**。

将来启动该项时的前置工作：

- 把 `ExhibitionBridge` 的 JS 桥接改写为 Wails 绑定
- 更换为 YukiHub 的展厅美术与数据接口
- 验证 WebGL 在 WebView2 下的兼容性与低端显卡降级策略

## 阶段 5：发布链路

- 自建更新服务（或改为不做应用内更新），替换上游 S3 / Cloudflare 方案
- 自备代码签名证书（上游依赖的 SignPath 开源免费签名资格不适用于本项目）
- 安装包、便携版、增量补丁、回滚的干净机验证
- 发布检查清单见 [AGPL-COMPLIANCE.md](AGPL-COMPLIANCE.md)

## 持续事项

- 每次发布前执行第三方依赖许可证审计
- `THIRD_PARTY_LICENSES.md` 与 `NOTICE` 随依赖变化更新
- 与上游保持"硬分叉不回灌"的定位；如需变更，先写 ADR
