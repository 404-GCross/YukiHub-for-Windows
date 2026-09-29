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
3. `wails3 build` 在本机与 CI 上成功产出可执行文件。**（本机已验证通过）**
   - 注：初次验证只跑到 `wails3 build`，**没跑完整的 `pnpm run build`**。
     后来构建安装包时才发现 `AddGameModal.tsx` 引用了已被重命名的品牌图片
     （`luna1/luna2.webp` → `brand-1/brand-2.webp`，去品牌化时改名却没同步引用），
     导致 `vite build` 报 `Could not resolve`。**CI 的 `pnpm run build`
     步骤同样会失败**，属必现问题。已修复（提交 `53e08d3`），
     并用脚本扫描确认全部 36 个相对资源引用中仅这 2 处失效。
4. NSIS 安装包可以完成"安装 → 启动 → 卸载"，且卸载后用户数据目录按预期处理。
   **（进行中）**
   - 产物：`build/bin/YukiHub-0.1.0-windows-amd64-setup.exe`（37.9 MB）
   - 本机无法直接运行 `scripts/build.bat`（安全策略拦截 `wmic.exe`，
     且 `pnpm install` 触发安全删除 shim 超时），已用等价的分步脚本完成构建，
     流程与坑记录在 `docs/fork-setup.md`
   - [x] 安装：用户在 D 盘实测通过
   - [x] 启动：实测发现 2 个缺陷，均已修复（提交 `35070b7`）
         · 未配置更新源时弹出 `failed to fetch update info from all sources: %!w(<nil>)`
           —— 清空默认更新地址时漏了"无源可用"分支，把"没有源"误报成"所有源失败"
         · 标题栏与侧边栏显示 "LunaBox" —— 渲染的是上游文字 logo 图片，
           文本替换覆盖不到二进制资源
   - [ ] 用重新构建的安装包复测启动
   - [ ] 卸载流程，以及卸载后 `%APPDATA%\YukiHub` 与 `%LOCALAPPDATA%\YukiHub` 的处理
5. macOS / iOS / Linux 相关代码与 CI 矩阵移除完毕。**（已完成，见下方）**
6. 界面上的"通用跨平台"表述与残留的上游素材占位清理完毕。**（已完成）**
   - [x] `PortableSetupPanel`（5 处三元 + 1 处条件渲染）、`GameSettingsPanel`、
         `TopBar`、`routes/__root.tsx`、`routes/game.tsx`、`routes/settings.tsx`
         的平台分支；四语言删除 6 个 macOS 专属孤儿键
   - [x] `GameLaunchPanel` 约 20 处 `isDarwin` / `isLinux` 及配套的 Wine runner
         选择 UI——连同 Go 侧 Linux 专属的 Steam Proton 兼容层整链删除
         （提交 12e1337，982 行重写为 360 行，四语言再清 49 个孤儿键）
   - [x] `UpdateDialog` 的 3 个平台变量与恒不显示的下载按钮区块
   - [x] 替换上游品牌素材（文本替换覆盖不到的二进制资源）：
         `appicon.png` / `appicon-dark.png` 与 `build/windows/icon.ico`
         已换成手机版 YukiHub 图标（提交 17629df）；
         文字 logo（`topbar-title*.png`）已删除，改为代码渲染 "YukiHub"；
         侧边栏 logo 与托盘图标改为雪花标识。
   - [ ] 添加游戏弹窗插画 `brand-1.webp` / `brand-2.webp`（上游素材，
         不含品牌文字，优先级低，待替换或删除）
   - [ ] 复查是否还有其它上游素材占位残留

### 阶段 1 验证记录（2026-09-28）

本机装上 MinGW-w64 并启用 `CGO_ENABLED=1` 后完成了此前无法执行的验证：

| 检查 | 结果 |
| --- | --- |
| `gofmt -l .` | 无输出 |
| `go vet ./...` | 无输出（修复了 1 处上游遗留的 unsafeptr 告警） |
| `go build ./...` | 通过（含 DuckDB 的 CGO 与 Wails） |
| `go test ./... -count=1` | 28 个含测试的包全部 `ok`，0 个 FAIL |
| `cd updater && go test ./... -count=1` | 通过 |
| `wails3 build`（离线） | 通过，产出 `bin/YukiHub.exe`（144.7 MB，PE 头有效） |

`wails3 build` 的执行链路：读取 `build/config.yml` → 重新生成绑定（601 包 /
240 方法）→ 生成 Windows 资源 `syso` → `go build` → 落盘 `bin/YukiHub.exe`。
体积 144.7 MB 主要来自 DuckDB 静态链接（amd64 走静态链接，不需要随包分发
`duckdb.dll`）与嵌入的前端资源；该路径已在 `.gitignore` 中。

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
- [x] 清除共享代码中的死分支 —— **已完成**（23 个文件，净减 410 行）。
      收敛 `runtime.GOOS` 恒真判断、删除恒假分支，并连带删除
      `IsMacAppBundlePath`（含 6 处调用点）、`repairStaleAppImageProtocolRegistration`
      与孤儿包 `internal/utils/tricksutils`。验证：gofmt / vet / build / test 全绿。
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

### 进展

- [x] 契约字段落地（迁移 177，导入方向）：`legacy_local_id`、`source_device_id`、
      `playtime_reset_at`、`hidden` 四个列与 `models.Game` 字段，贯穿
      建表 / INSERT / SELECT / UPDATE / 云同步快照 / 测试 helper；导入器正确填充，
      `playtime_reset_at` 为 0 时落 NULL（提交 e520b1e）
- [x] 修正契约文档的单位错误：`play_sessions.duration` 与 `games.total_play_time`
      **均为毫秒**（原文误写为"duration 是秒、两者单位不同"），已按手机版
      `GameRepository` 源码更正并补记 Android 侧的清零过滤行为
- [x] 统一领域模型（三语标题 / 别名 / NSFW / 五态 / 标签 / 封面来源 / 元数据来源）复核
      （2026-09-28，见下方"领域模型复核结论"）
- [x] 导出方向：`internal/service/exporter/yukihub.go` 产出 schema 5 快照
      （`Build()` + gzip `Export()`）。单位毫秒换算、清零过滤、每游戏 30 条会话上限、
      6→5 态映射、`favorite` / `hidden` / `nsfw` / `local_id` 回填；
      `root_uri` 恒空（理由见迁移文档），无标题条目跳过。
      测试：单测 8 项 + 集成 3 项（含**导出→导入往返**，用真实 importer 消费快照）
- [x] 合并语义测试：**总时长取最大值**（`merge_sessions` 动作：会话并集去重 +
      聚合补偿差额 → 最终 = max(桌面已录, 快照 total)，测试覆盖）、
      会话 UUID 幂等（skip 动作重导幂等，测试覆盖；数据库层无 UUID 唯一约束，
      两层近似保证已记录在迁移文档）。导入器此前丢弃 `samePathAction`
      （无合并路径）、标题匹配与预览不一致（预览判重、导入重复），均已修复
- [x] SAF 树 URI 与 Windows 绝对路径的映射与不可达降级：导出侧不写 Windows 路径
      （`root_uri` 恒空 → 对端按标题匹配）。遗留 `legacy_root_uri` 回填见迁移文档
- [x] 万级样例导入性能与 0 丢失验证（见下方"阶段 2 验证记录"）

**原阻塞项已决策**（见 [ADR-0002](../decisions/0002-android-authoritative-and-sync-backend.md)）：

- 数据权威源 = **Android 端**，schema 5 快照是权威契约
- 同步后端 = **本项目自有服务器**（Android 版已在用的 `/api/sync/*`），不接上游托管服务
- 冲突裁决收敛为**一套**（以 Android 侧哈希比对为准），
  上游 `cloudsync` 的墓碑/脏表长期要替换 —— **该验收项实际落到阶段 6**，
  阶段 2 只完成契约与文件级迁移
- 元数据缓存沿用 Android 版 `VnMetadata` 字段

结论：文件级导入导出（验收 2、3）**不依赖服务端，是本阶段主线**。

### 阶段 2 验证记录（2026-09-28）

万级样例验证落在 `internal/service/test/yukihub_scale_test.go`。
快照由测试现场生成：10,000 款游戏（每款一条唯一 vndb 元数据、2 条标签、
`i%3` 决定 2/1/0 条明细会话、`i%4` 秒的时长差额触发聚合补偿会话），
合计 17,501 条会话、20,000 条标签、10,000 条元数据源。
关键在于**走真实 Committer 落库**（`newTestImporterDependencies`）后再查库核对，
而不是只检查内存里的 `ImportItem`——后者不会暴露 staging 落库环节的字段丢失。

| 用例 | 覆盖点 | 结果 |
| --- | --- | --- |
| `TestYukiHubImportScaleNoLossNoDuplicates` | 10k 导入 0 丢失、0 重复 | 通过（0.76s） |
| `TestYukiHubImportScaleIsIdempotent` | 同一快照重复导入幂等 | 通过 |
| `TestYukiHubImportPersistsMobileContractFields` | 四个契约列落库 | 通过 |

`TestYukiHubImportScaleNoLossNoDuplicates` 逐项核对：games=10,000、play_sessions=17,501、
game_tags=20,000、game_metadata_sources=10,000；游戏名与 `legacy_local_id`
各自去重后仍为 10,000（0 丢失、0 重复）；`hidden` / `nsfw` / `playtime_reset_at`
非空计数与期望一致；并抽查首条游戏确认 `legacy_local_id` / `hidden` / `nsfw` /
`playtime_reset_at` 落到正确列上。耗时 0.76s（导入侧日志显示 games 落库 122ms、
标签 121ms、会话 84ms、整体提交 512ms），远低于 90s 护栏。

`TestYukiHubImportScaleIsIdempotent` 确认重复导入 success=0 / skipped=10,000 / failed=0，
`games` 与 `play_sessions` 行数不变。

**过程中修复的缺陷**：契约字段（`legacy_local_id`、`source_device_id`、
`playtime_reset_at`、`hidden`）此前只被填进 `models.Game`，但导入器的
staging 表（`temp_import_games` / `temp_update_import_games`）与
INSERT / UPDATE 语句都不含这些列，落库时被**静默丢弃**——这会让身份键
（`legacy_local_id`）与清零语义、隐藏标记在桌面端全部失效。
已补齐四列（`internal/service/importer/persistence.go`），
并对 `playtime_reset_at` 增加 `nullableTime` 处理，使 0 落 NULL 而非 1970。
这正是"只测内存对象测不出来"的一类缺陷。

### 领域模型复核结论（2026-09-28）

验收项 1 的七个子项逐条核对（映射细节见
[mobile-yukihub-migration.md](mobile-yukihub-migration.md) 的"字段映射"）：

| 子项 | 落点 | 结论 |
| --- | --- | --- |
| 三语标题 | `name` + `aliases`（原名 / 罗马字并入别名数组） | 信息不丢，但**不做结构化区分**（设计如此） |
| 别名 | `games.aliases`（JSON 数组） | 完整 |
| NSFW | `games.is_nsfw` | 完整（导入取快照 `nsfw`，导出回填） |
| 五态游玩状态 | `games.status`（桌面 6 态，含"想玩"） | 完整，双向映射已容忍历史写法 |
| 标签 | `game_tags` 表 | 完整（快照 `tags` 文本拆分去重 + 元数据 `tagsText` 合并） |
| 封面来源 | `cover_url` / `cover_source_url` / `source_type` | 完整；本地封面按契约不迁移 |
| 元数据来源 | `source_type` / `source_id` / `game_metadata_sources` | 导入完整；导出侧 `metadata_cache` 待两端结构统一（见迁移文档） |

复核中发现并修复一处**真实缺陷**：`favorite` 在导入方向被完全丢弃
（导出侧读 `system:favorites`，导入侧不碰分类表），导致往返丢失收藏。
已补齐（`ImportItem.Favorite` → `game_categories`），语义为"只加不删"。
详见迁移文档"现状核对 ⑦"。

遗留（已在迁移文档记录，非本轮范围）：
- 三语标题不做结构化区分，`original_title` 导出时取第一个别名（对 VNDB / Bangumi
  等主源恰好是原名，属可用启发式）。
- `metadata_cache` 导出待两端元数据缓存结构统一后再补。

## 阶段 3：产品层重建

目标：界面与交互是 YukiHub，而不是"改了名字的 LunaBox"。

验收标准：

1. 首页 / 库 / 游戏详情 / 统计 / 设置五个主界面完成 YukiHub 化改造。**（已完成，见进展）**
2. 上游素材（应用图标、界面插画、启动图、截图）全部替换为 YukiHub 素材。**（已完成，见进展）**
3. 元数据来源合并为单一入口（Bangumi、VNDB、月幕、Hikarinagi、Steam 等），
   优先级与缓存策略统一。**（后端已合并，见进展；前端本已是单一注册表）**
4. 游玩启动与时长统计在 Windows 上按真实进程行为工作：
   - 启动、退出、崩溃、Switch 用户、开机自启场景均有验证
   - 后台/前台时长语义与 Android 版规则对齐
   **（静态部分：启动策略接线已补测；端到端真机验证待做，见进展）**
5. 同步协议落地：至少支持一种用户自持的同步后端（WebDAV 优先）。
   **（上游能力已具备且为默认项，端到端真机验证待做，见进展）**
6. 应用内"关于与开源许可"面板完成，展示版本、上游署名与 AGPL 声明。**（已完成）**

### 进展

- [x] 设计令牌对齐手机版（2026-09-28）：从手机版 030p 源码的
      `app/src/main/res/values/colors.xml` 提取原始调色板，落到
      `frontend/uno.config.ts`
      - `brand` 中性色板整体改为冷调深蓝：`brand-900 = #0B1020`（`yh_bg`）、
        `brand-800 = #171E33`（`yh_card`）、`brand-700 = #2D3658`（`yh_line`）、
        `brand-400 = #9AA4BF`（`yh_text_muted`）。暗色模式的主背景与卡片
        因此直接变成手机版那套深蓝，这是"像不像 YukiHub"的决定性一步
      - `primary` 改为手机版 `yh_primary` 柔和蓝：`primary-300 = #8AB4FF`
        （手机版原值，供暗色模式强调文字），`500/600` 压深一档
        （`#6E96E8` / `#5A7CC9`）以保住亮色模式"白字蓝底"按钮的对比度
      - 新增 `secondary`（`yh_secondary` 樱粉 `#FF8AB3`）与 `yh` 原始令牌命名空间
      - 新增两条签名渐变 rule：`yh-primary-gradient`（主按钮，
        取自 `bg_button_primary.xml`）与 `yh-hero-gradient`（首页横幅，
        取自 `bg_home_gradient.xml`）
      - 注意：手机版是**暗色优先**（`values-night/colors.xml` 为空），
        而桌面版默认主题是 `light`（`appconf/config.go`），
        故亮色模式按同色系做了浅色版，不是手机版的直接搬运
- [x] 侧边栏视觉对齐（2026-09-28）：导航项选中态改为手机版 `bg_sidebar_item` 的做法
      ——半透明蓝底（`primary-500/12`）+ 左侧强调条（`border-l-2`），
      取代上游的"整块实心灰底"；侧栏底色改用 `dark:bg-yh-sidebar`（`#10172A`）
- [x] 首页 YukiHub 化（2026-09-28）：按手机版首页信息架构重做，拆为四个组件
      ——`HomeHeroCard`（横幅渐变 + 继续游玩主胶囊）、
      `HomeQuickLaunchRail`（横向快速启动滑轨）、
      `HomeTodayStatsCard`（今日数据）、`HomeHeatmapCard`（热力图）；
      新增 `yh-glass` / `yh-glass-inner` / `yh-glass-chip` / `yh-chip` /
      `yh-hero-gradient-light` 等令牌，亮色模式补同色系浅色版；
      清理旧首页组件（`HomeGameRailPanel` 等）与失效 hook
- [x] 游戏库 YukiHub 化（2026-09-28）：对齐手机版"筛选芯片 + 网格 + 右侧详情"
      - 新增 `LibraryStatusChipRow`：状态筛选从抽屉里提到页面顶部，
        做成手机版 `bg_chip` 风格的横向胶囊行，选中态用 `yh-primary-pill`
      - 新增 `LibraryDetailPanel`：对应手机版 `dialog_game_detail`，
        桌面端改为**常驻右侧栏**（`lg` 以上显示，`sticky` 跟随滚动），
        含封面 + 状态徽标、名称/简介、厂商/评分/发售/最近游玩信息行、
        启动游戏与"完整详情"两个动作
      - `GameCard` 增加状态徽标、评分芯片、封面底部排序字段覆盖条与
        悬浮启动按钮；`onActivate` 回调贯通 `VirtualGameGrid` → 库页面，
        非多选态点击卡片即展开右侧详情；多选态自动收起详情面板
      - 状态徽标样式集中到 `consts/gameStatusBadge.ts`，卡片与详情面板共用
      - 顺带修复 `HomeQuickLaunchRail` 引用了不存在的 `common.details` 文案，
        并清理 10 个死键（`home.welcome` / `home.noPlayRecord` /
        `home.recentPlayed` / `home.expandCoverPicker` / `home.collapseCoverPicker` /
        `stats.library.*`），`i18n:check` 恢复通过
- [x] 游戏详情页 YukiHub 化（2026-09-28）：详情页头部、状态切换芯片行、
      标签页签改用 YukiHub 令牌——状态芯片复用 `consts/gameStatusBadge.ts` 的
      `GAME_STATUS_BADGE_STYLES`，选中态用 `yh-primary-pill`、未选中态用
      `yh-chip`；正文卡片容器改用 `yh-glass`，与库页/首页视觉统一
- [x] 统计页 YukiHub 化（2026-09-28）：页面头部改为 `yh-glass` 玻璃条
      （圆形渐变图标 + 标题），概览指标卡、排行榜/标签分布/时段分布/趋势图
      五块内容全部改用 `yh-glass` 承载，取代上游的实心 `brand-50`/`brand-800` 面板
- [x] 设置页 YukiHub 化（2026-09-28）：页面头部改为 `yh-glass` 玻璃条
      （新增 `settings.subtitle` 四语文案），`CollapsibleSection` 折叠卡由
      `glass-settings-section` + 实心底色改为 `yh-glass`，折叠头图标改用
      `primary-500/300`、悬停态改为白色半透明；底部 GitHub 按钮改为
      `yh-primary-pill` 同款圆角胶囊。设置项面板本身为透明布局，无需改动
- [x] 修复"全站图标空白"（2026-09-29）：页面上大量图标显示为空白方块
      - 根因：pnpm 严格 `node_modules` 布局下，`presetIcons()` 的 node loader
        位于 `.pnpm/@iconify+utils@.../`，无法解析到
        `frontend/node_modules/@iconify-json/mdi`（`@iconify-json` 不在
        `.pnpm/node_modules` 提升列表里），图标集探测**静默失败**——
        生产构建 CSS 中 `.i-mdi-*` 规则数为 **0**，即全站图标从未生成过
      - 修法：`uno.config.ts` 显式注册 mdi 图标集
        （`presetIcons({ collections: { mdi: () => mdiIcons } })`）。
        注意值**必须是函数**：`@iconify/utils` 的 `loadIcon` 只会对函数求值后
        按集合查找，直接传集合对象会被误当成 loader 调用而返回空
      - 顺带修正 3 个在 MDI 中并不存在的图标名：
        `application-search-outline` → `magnify-scan`、
        `content-save-clock-outline` → `content-save-outline`、
        `database-star-outline` → `database-outline`
      - 校验：源码 186 个唯一 `i-mdi-*` 图标现已全部生成，缺失 0
- [x] 元数据来源合并为单一入口（2026-09-29）：验收项 3 的后端部分
      - 问题：`GameService` 与 `ImportService` 各持一份
        `getConfiguredMetadataSearchSources`，两份实现逐行几乎相同
        （同样的 switch、同样的 getter 构造、同样的服务判空），
        任何新增来源或选项改动都要改两处，属"两套来源清单长期分叉"的隐患
      - 修法：抽出唯一构建器
        `internal/service/metadata_search_sources.go` 的
        `buildConfiguredMetadataSearchSources(deps)`，
        连同 `metadataSearchSource` 类型一并迁入；两个服务改为传入依赖的薄封装
        （`GameService` 直接调用，`ImportService.metadataSearchSources()` 委托）
      - 优先级：直接沿用 `gamehelper.ConfiguredMetadataSources` 返回的用户配置顺序，
        构建器**不做任何重排**，界面上勾选的顺序即实际尝试顺序
      - getter / 缓存策略：统一由 `gamehelper.MetadataGetterOptions` 生成
        （代理、tag 上限、ErogameScape 基址、各源封面来源），两处不再各持一份；
        各来源自身的 token 缓存（Hikarinagi token、Steam tag catalog）保持来源内聚
      - 前端侧本已是单一注册表 `frontend/src/utils/metadataSources.ts`
        （`ALL_METADATA_SOURCES` / 图标 / URL 解析），无重复清单，无需改动
      - 测试：新增 `metadata_search_sources_test.go` 锁定两条契约——
        配置顺序不被重排、未注入服务的来源被跳过，以及
        `ImportService` 清单与共享构建器逐项一致
      - 验证：`gofmt` / `go vet` / `go build ./...` 无输出，
        `go test ./... -count=1` 全绿（退出码 0）
- [x] 补齐 Windows 启动策略的测试覆盖（2026-09-29）：验收项 4 的静态部分
      - 背景：Magpie（超分）与 Locale Emulator（转区）是 **Windows 独有能力**，
        后端策略（`launcher/strategy_windows.go`）与前端 UI
        （`GameLaunchPanel` 每游戏开关 + `GameSettingsPanel` 路径配置）都已具备，
        但"字段 → 启动计划"的接线此前没有任何测试
      - 新增 5 条用例（`strategy_windows_test.go`）：
        游戏字段开启 Magpie 进入计划、单次启动开关可正反向覆盖 Magpie、
        未配置 LE 路径时**回落到原生启动**（不把不存在的 exe 写进计划）、
        单次开关可临时启用转区、转区与超分可叠加（LE 计划仍保留 Magpie 标记）
      - 验证：`gofmt` 无输出；`go test ./internal/service/launcher/... -count=1`
        11 条用例全通过；`go vet` / `go build ./...` / `go test ./... -count=1` 全绿
      - 时长语义核对（静态，2026-09-29）：`config.record_active_time_only`
        **默认 `false`**，即按墙钟计时（整段会话时长），与 Android 版
        "启动建会话、退出结算时长"的语义一致；开启后切换为
        `utils/timerutils/active_time_tracker.go` 的窗口焦点活跃计时，
        作为桌面端增强项，且已有 12 条单测覆盖
      - 仍待真机验证（不在本轮范围）：启动/退出/崩溃/切换用户/开机自启的
        端到端行为
- [x] 核对验收项 5（WebDAV 自持同步）现状（2026-09-29）：**无需新实现**
      - 结论：上游已内建 WebDAV 云备份后端，且**已是默认 provider**
        （`appconf` 默认 `cloud_backup_provider = "webdav"`），全链路已接线：
        `appconf`（URL/用户名/密码）→ `cloudprovider/factory.go`
        （`NewCloudProvider` / `TestConnection` / `IsConfigured` 三处均已登记
        `ProviderWebDAV`）→ `backup_service.go`（`TestWebDAVConnection`）→
        前端 `CloudBackupSettingsPanel`（配置表单 + 连接测试）与
        `cloudSync.ts` / `useCloudSync.ts`，四语言文案齐备
      - 实现质量：`webdav/webdav_provider.go` 有地址合法性校验、代理支持、
        429/Retry-After 重试、BasicAuth、PROPFIND 区分文件与集合、
        父目录缺失时补建后重试；`webdav_provider_test.go` 覆盖了
        重试 + 认证 + 代理 + User-Agent
      - 因此验收项 5 的缺口不在实现，而在**真机端到端验证**
        （对真实 WebDAV 服务端跑通 上传 → 列表 → 下载 → 恢复）
      - 验证：`go test ./internal/service/cloudprovider/... -count=1` 通过

手机版参考源码的提取方式（归档在 `参考文件/YukiHub手机版030p...7z`，
用 7z 按需抽取，不要整包解压）：

```powershell
$7z = "$env:APPDATA\TRAE SOLO CN\ModularData\ai-agent\vm\tools\bin\7z.exe"
$arc = "参考文件\YukiHub手机版030p离线展厅成型，进度条跳转.7z"
& $7z x $arc -o".tmp_ref" -y -r "*res/values/colors.xml" "*res/layout/activity_main.xml"
```

关键结论：手机版主界面（`activity_main.xml`）本身就是**三栏**结构
——左侧 76dp 竖栏（头像/导航/底部统计胶囊）+ 中间网格 + 右侧 152dp 详情面板。
也就是说它的信息架构已经接近桌面形态，桌面版要做的是**放大并适配鼠标键盘**，
而不是重新设计一套信息架构。

## 阶段 4：Android 版差异化能力迁移

目标：把桌面版做成"手机版的桌面延伸"，而不是一个通用管理器。

按优先级：

1. **游玩记录与数据同步协议**（已在阶段 2 完成）
2. **多源元数据与本地缓存离线可用**
3. **AI 游玩报告**（复用上游已有的 AI 服务与防剧透配置）→ **已移至阶段 6**（依赖服务端）
4. ~~**OCR 与多引擎翻译工作流**~~ → **已移除（2026-09-29）**
   - 理由：截图 OCR + 悬浮翻译层是**手机独有**的能力形态（Android 无障碍 /
     MediaProjection 取词）；Windows 侧可用的翻译工具（Textractor、Translator++、
     各类屏幕翻译）已足够成熟，桌面端再内建一套 OCR 只是重复造轮子
   - 仓库现状：本项目从未移植过该能力（无 OCR 代码、无 `assets/ppocrv6/` 素材），
     此前仅作为计划项列出，现已删除，不再是待办
5. **社区与好友功能**（REST 契约可复用，界面重做）→ **已移至阶段 6**（依赖服务端）
6. **大屏模式**（对齐手机端的大屏形态；产品设计可参考 Android 版，桌面端 UI 应能做得更好）
   → **保留要做，但不急**，随项目进度稳步推进
   - 注：**音乐厅 = 离线 3D 展厅**（手机端 `assets/exhibition/`）是**同一项**，
     且 **2026-09-29 决定不做**，详见下方"不做：离线 3D 展厅（音乐厅）"。
     此前条目里"音乐厅 / 大屏模式"并列写法有误导，已拆分。

进展：

- [x] 多源元数据与本地缓存离线可用（2026-09-29）：验收项 2 的缓存 / 离线部分
      - 问题：桌面端把刮削结果散落在 `games` / `game_tags`，没有保留来源侧的原始负载；
        Android 的 `metadata_cache`（`VnMetadata` JSON）两个方向都未被搬运——导入只解析
        身份与少量字段后丢弃 blob，导出完全不产出 `metadata_cache`。结果是
        「Android → 桌面端 → Android」静默丢失桌面端没有对应列的字段（截图、罗马音标题、
        封面分级），且离线时没有可复用的本地副本
      - 修法：`game_metadata_sources` 新增 `cache_json TEXT`（`migration178`，历史行为空串），
        负载沿用 Android 的 `VnMetadata` 结构，两端原样往返
        （`models/yukihub` 的 `MetadataCache.JSON` ↔ `cache_json`）
      - 写入点：导入时原样写入 `entry.json`；刮削时在 `applyRemoteMetadataResult` 统一写入
        （该函数同时服务单条更新与批量刷新两条路径）；导入 upsert 采用"空负载不覆盖已有缓存"，
        避免快照缺少 `metadata_cache` 元素时把对端已有缓存抹成空
      - 导出：`loadMetadataCache` 只导出带 `legacy_local_id`（可被 Android 关联）且标题非空的
        条目，桌面端自建条目跳过，避免产出孤儿缓存
      - 测试：`TestYukiHubImporterPreviewAndImport`（导入原样保留）、
        `TestApplyRemoteMetadataCachesSourcePayload`（刮削写入的负载字段）、
        `TestYukiHubMetadataCacheRoundTrip`（导出 → 导入 → 再导出逐字节一致 +
        无 `legacy_local_id` 条目被跳过）
      - 验证：`gofmt` 无输出；`go vet ./...` 退出码 0；`go test ./... -count=1` 全绿
      - 契约文档已同步：`docs/mobile-yukihub-migration.md`（`metadata_cache` 由"暂不导出"
        改为已导出，待决策问题 4 标记为已决策）

明确排除（**手机独有形态，桌面端不做**）：

- Android 内置游戏引擎（Kirikiri、ONS、Tyrano、Artemis、PSP）
- 模拟器启动适配（Winlator、GameHub、盖世）
- 截图 OCR 与悬浮翻译层（理由见上）
- Shizuku、SAF 镜像存档、触控光标注入、`.nomedia` 管理

明确保留（**Windows 独有能力，属于"功能以电脑侧为准"，不得随平台裁剪误删**）：

- **Magpie 超分**：`config.magpie_path` / `default_use_magpie`、
  `game.use_magpie`；启动时以托盘模式拉起（`start_service.go`），
  策略在 `launcher/strategy_windows.go`
- **Locale Emulator 转区**：`config.locale_emulator_path` /
  `default_use_locale_emulator`、`game.use_locale_emulator`；
  未配置路径时回落到原生启动
- **以管理员身份运行**：`game.launch_mode = admin`
- **Steam 直启**：按注册表定位安装目录，`steam://rungameid/<id>`
- **进程树/窗口焦点活跃时长统计**：`utils/timerutils/active_time_tracker.go`

## 不做：离线 3D 展厅（音乐厅）

Android 版的 Three.js 离线展厅（`assets/exhibition/`，约 7200 行纯 Web）可近乎零改动地
由 WebView2 承载，但 **2026-09-29 决定不做**——它就是产品里的"音乐厅"，
两者是同一项功能，本次一并确定不迁移。
（此前写作"暂缓"，措辞已更正为"不做"。）

若将来重启该项，前置工作是：

- 把 `ExhibitionBridge` 的 JS 桥接改写为 Wails 绑定
- 更换为 YukiHub 的展厅美术与数据接口
- 验证 WebGL 在 WebView2 下的兼容性与低端显卡降级策略

## 阶段 5：发布链路

- 自建更新服务（或改为不做应用内更新），替换上游 S3 / Cloudflare 方案
- 自备代码签名证书（上游依赖的 SignPath 开源免费签名资格不适用于本项目）
- 安装包、便携版、增量补丁、回滚的干净机验证
- 发布检查清单见 [AGPL-COMPLIANCE.md](AGPL-COMPLIANCE.md)

## 阶段 6：服务端相关（已按 ADR-0002 排后）

目标：把依赖服务端的能力接上本项目自有的服务器，并把冲突裁决收敛为一套实现。

依据 [ADR-0002](../decisions/0002-android-authoritative-and-sync-backend.md)：
权威源为 Android 端，同步后端为本项目自有服务器（不接上游托管服务）。
以下各项**均依赖服务端**，故整体排在阶段 1–5 之后。

- 服务端确认接受桌面端客户端（版本、User-Agent、协议版本），确定桌面端 client 标识
- 同步协议对接：桌面端接入 `/api/sync/*`（Android 版已在用的自有服务）
- 冲突裁决收敛：移除上游 `cloudsync` 的墓碑/脏表机制，改用 Android 侧的哈希比对
  （**不得两端各有一份合并算法长期并存**）
- AI 游玩报告（依赖服务端；上游的 AI 服务与防剧透配置可复用）
- 社区与好友功能（REST 契约可复用，界面重做）
- 在线账号与跨设备身份（`source_device_id` 的价值在此阶段才真正体现）

注：阶段 3 的 **WebDAV 用户自持同步保留原位**——它是用户自己的存储，
与"我们的服务器"不冲突，不属于本节。

## 本地开发与预览

本机的 Go / wails3 / pnpm / MinGW 都是便携版（在 `%USERPROFILE%\.workbuddy\tools` 下），
**没有加入系统 PATH**。为避免每次手工拼环境变量，新增了可双击的预览脚本：

- `scripts/preview.bat`：双击即可 —— 临时注入工具链环境 → 打包前端 → `wails3 build`
  → 关闭已在运行的旧实例 → 启动 `bin\YukiHub.exe`
- `scripts/preview.ps1`：实际逻辑（bat 只是调用壳），支持：
  - `-Check`：只检查工具链是否可用，不构建不启动
  - `-RunOnly`：跳过构建，直接启动已有的 `bin\YukiHub.exe`
  - `-SkipFrontend`：跳过前端打包（前端未改动时更快）
- 两个关键坑：
  1. 前端资源通过 `//go:embed all:frontend/dist` **打进二进制**，所以改完前端
     必须重新 `wails3 build`，否则直接跑 exe 看到的仍是旧界面；
     日常调 UI 建议用 `wails3 dev`（Vite 热更新 + Go 改动自动重编译）
  2. 系统里的 `pnpm` 是坏的（corepack 缓存缺失），脚本会通过 `COREPACK_HOME`
     指向 `.workbuddy\tools\corepack` 并优先使用 `corepack-bin\pnpm.cmd`

## 持续事项

- 每次发布前执行第三方依赖许可证审计
- `THIRD_PARTY_LICENSES.md` 与 `NOTICE` 随依赖变化更新
- 与上游保持"硬分叉不回灌"的定位；如需变更，先写 ADR
