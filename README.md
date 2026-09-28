# YukiHub for Windows

YukiHub 的 Windows 桌面版：Galgame / 视觉小说库管理、启动与游玩记录工具。

> **开发状态：早期开发中。** 当前仓库是以上游 LunaBox 为基线的硬分叉，
> 正在进行去品牌化、工程加固与产品层重建。**尚无可发布的安装包**，
> 界面与功能仍是上游形态。进度见 [docs/ROADMAP.md](docs/ROADMAP.md)。

## 这是什么

YukiHub 有两个端：

| 端 | 仓库 | 许可证 |
| --- | --- | --- |
| Android 手机版 | https://github.com/xm486/YukiHub | GPL-3.0 |
| Windows 桌面版（本仓库） | 见仓库地址 | AGPL-3.0 |

桌面版的目标不是把手机版移植过来，而是与手机版共享同一套数据语义与使用习惯：
游戏库、游玩记录、资料刮削、数据同步、备份恢复。两端之间的数据可以互相导入导出。

## 规划中的能力

已从上游基线继承并可直接使用：

- Windows 游戏进程识别与退出监听，自动统计游玩时长
- Locale Emulator 启动（日文游戏转区）与 Magpie 缩放器联动
- 多来源资料刮削（Bangumi、VNDB、月幕 Gal、Hikarinagi、Steam 等）
- 批量目录扫描导入、拖入导入，以及从 Playnite / PotatoVN / Vnite / ReinaManager 迁移
- 存档与数据库备份（本地 + 云）、多设备同步
- 系统托盘、开机自启、URL 协议唤醒、代理、后台静音
- 命令行长接口与 MCP 服务
- NSIS 安装器与应用内增量更新

计划从 Android 版迁入：

- 与手机版一致的游玩记录与同步协议（见 [迁移设计](docs/mobile-yukihub-migration.md)）
- AI 游玩报告
- OCR + 多引擎翻译工作流

暂缓：

- 离线 3D 展厅（手机版已有实现，桌面版后续再评估）

明确不做：

- 内置 Galgame 引擎与模拟器启动（Windows 上直接运行原生程序即可）

## 从源码构建

> 本项目仅支持 Windows。macOS / iOS / Linux 的平台代码与构建资源已经移除，
> 不要尝试在其它平台构建。

环境要求：

- Go（版本见 `go.mod`，当前 1.27.1）
- Node.js 24 与 pnpm 9
- Wails v3 CLI，版本需与 `go.mod` 中 `github.com/wailsapp/wails/v3` **完全一致**

```bash
# 1. 前端依赖
cd frontend && pnpm install && cd ..

# 2. 生成 Wails 绑定（后端 service 方法签名变更后必须重新执行）
wails3 generate bindings -clean=true -ts

# 3. 开发模式运行
wails3 dev -config ./build/config.yml -port 9245

# 4. 构建
wails3 build
```

提交前自检：

```bash
gofmt -l .          # 应无输出
go vet ./...
go test ./... -count=1
```

## 分叉说明与待配置项

本仓库是 LunaBox v1.13.0 的硬分叉。代码层面的品牌替换已完成，但**仓库地址、第三方服务凭据、
代码签名与更新服务**必须由 YukiHub 自行配置后才能发布。完整清单见
[docs/fork-setup.md](docs/fork-setup.md)。

上游版权与修改记录见 [docs/upstream-lunabox.md](docs/upstream-lunabox.md)。

## 开源许可

本项目采用 **GNU Affero General Public License v3.0（AGPL-3.0）**，全文见 [LICENSE](LICENSE)。

- 本项目是修改版本，基于 LunaBox 开发，**并非上游官方发行版**，上游不提供担保或支持。
- 上游项目版权归 LunaBox contributors 所有，同样采用 AGPL-3.0。
- 版权与修改声明见 [NOTICE](NOTICE)，合规义务说明见 [docs/AGPL-COMPLIANCE.md](docs/AGPL-COMPLIANCE.md)。
- 第三方组件许可证见 [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md)。

## 免责声明

本项目仅用于管理和启动你**有权使用**的游戏、应用或资源。

本项目不提供游戏本体、破解资源或任何绕过授权的能力，也不为违规用途提供支持。

## 参与贡献

- 提交前请确保 `gofmt`、`go vet`、`go test` 均通过。
- 后端改动请遵循 [docs/backend.md](docs/backend.md) 的分层与依赖注入约束。
- 涉及数据结构变更时，请同时更新 [迁移设计](docs/mobile-yukihub-migration.md)，
  因为 Android 版需要与桌面版保持数据语义一致。
- 大型技术决策请先写 ADR，放在 `docs/decisions/` 下。
