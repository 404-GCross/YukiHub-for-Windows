# 上游项目与修改记录（LunaBox）

本文件用于满足 GNU AGPL v3 第 5(a) 条的要求：修改版本必须显著声明"已修改"，并给出相关日期。
同时保留上游项目的版权声明与出处。

## 上游项目

| 项目 | 说明 |
| --- | --- |
| 名称 | LunaBox |
| 仓库 | https://github.com/Saramanda9988/LunaBox |
| 版本 | v1.13.0 |
| 版权 | Copyright (c) LunaBox contributors |
| 许可证 | GNU Affero General Public License v3.0 |
| 许可证全文 | 本仓库根目录 `LICENSE`（与上游一致，未作修改） |

LunaBox 是一款开源 Galgame 库管理与启动工具，Windows 为其主支持平台。
本项目的 Windows 端已实现能力（进程识别与退出监听、游玩时长统计、Locale Emulator 启动、
Magpie 联动、托盘、URL 协议唤醒、系统代理、后台静音、NSIS 安装器、应用内增量更新与
Authenticode 校验等）源自该上游项目。

上游历史变更日志完整保存在 `CHANGELOG.upstream.md`（同样未作修改）。

## 本项目的定位

YukiHub for Windows 是 YukiHub 项目的 Windows 桌面版本。
它与 Android 版 YukiHub（https://github.com/xm486/YukiHub ，GPL-3.0）属于同一产品家族，
但桌面版并非 Android 版的移植，而是以上游 LunaBox 为基线重新构建产品层。

## 修改记录

### 2026-09-27 — 建立硬分叉基线并完成去品牌化

基线提交：`chore: import LunaBox v1.13.0 as fork baseline`

在此基础上完成：

1. **去品牌化**
   - Go 模块名 `lunabox` → `yukihub`（含 `updater` 子模块 `lunabox/updater` → `yukihub/updater`）
   - 应用标识 `io.github.saramanda9988.lunabox` → `com.yukihub.desktop`
   - URL 协议 `lunabox://` → `yukihub://`
   - 数据目录 / 数据库 `LunaBox`、`lunabox.db` → `YukiHub`、`yukihub.db`
   - 更新器命令 `lunabox-updater` / `lunabox-update-builder`
     → `yukihub-updater` / `yukihub-update-builder`
   - 前端工作区包 `@lunabox/desktop-shell-*` → `@yukihub/desktop-shell-*`，
     Wails 生成绑定路径 `frontend/bindings/lunabox/` → `frontend/bindings/yukihub/`
   - 构建期环境变量 `LUNABOX_*` → `YUKIHUB_*`
   - 安装器 NSIS 脚本、Windows 版本资源、应用图标路径等同步调整
2. **身份与凭据**
   - User-Agent 由 `Saramanda9988/LunaBox/...` 改为 `xm486/YukiHub/...`
   - 移除源码中硬编码的上游 Hikarinagi OAuth Client ID，改为由构建期注入；
     所有第三方服务凭据必须由 YukiHub 自行申请
   - 移除应用内更新检查中指向上游更新服务的默认地址
3. **默认行为调整**
   - 默认云备份后端由上游绑定的托管服务改为 WebDAV（用户自持存储）；
     托管后端仍作为可选项保留
4. **工程与合规**
   - 修复 CI 中 "只编译不执行测试" 的写法（原为 `go test -run '^$'`），
     Go 测试改为真实执行，并新增 `gofmt` 与 `go vet` 门禁
   - 新增 `NOTICE`、`THIRD_PARTY_LICENSES.md`、`third_party/` 许可证文本
     （上游仓库中没有任何第三方许可证清单）
   - 新增本文件与 `docs/AGPL-COMPLIANCE.md`
   - 全仓库 Go 代码重新执行 `gofmt`（模块改名会影响导入排序，属于必要调整）
   - 版本号从上游的 `1.13.0` 改为 YukiHub 自己的 `0.1.0`

### 2026-10-06 — 恢复 Linux（amd64）支持

- 从本仓库历史（平台移除前的提交 `02158b5^`、`8a55b8c^`、`fb87c68^`）恢复
  Linux 平台实现：进程识别（/proc）、启动策略（原生 / Wine / Proton / Steam）、
  Steam 集成（兼容工具 VDF / Proton prefix / 客户端重启）、URL 协议注册、
  Wine/Proton 辅助工具、7zz 捆绑与 deb / rpm / AppImage 打包链
- 回移上游 v1.13.0 之后对 Linux 进程识别的修复（忽略 Steam runtime helper）
- 恢复 `AppConfig` 的 Wine / CrossOver 字段与迁移逻辑；数据契约未变更
- 移除 CLI 后的适配：Linux 打包不再包含 `yukihubcli`
- 决策与范围见 `docs/decisions/0004-restore-linux-support.md`

### 尚未修改、计划修改

- 产品界面与交互仍为上游形态，尚未替换为 YukiHub 的视觉与信息架构。
- 游戏领域模型仍为上游语义，尚未合并 Android 版的视觉小说条目语义
  （三语标题、NSFW、`play_status` 五态等）。
- macOS / iOS / Linux 平台代码与 CI 矩阵尚未移除（本项目仅面向 Windows）。
- 应用图标、启动图、界面内品牌插画仍为上游占位资源，需要替换为 YukiHub 素材。
- 应用内尚无独立的"开源许可"展示界面。

## 与上游的关系

本项目定位为**硬分叉，不与上游保持同步**，也不向上游回灌代码。
理由与取舍见 `docs/decisions/0001-fork-lunabox-as-windows-baseline.md`。

上游项目不对本产品提供任何担保、支持或背书。
