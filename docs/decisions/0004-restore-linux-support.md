# ADR-0004：在本仓库恢复 Linux（amd64）支持

- 状态：已接受
- 日期：2026-10-06
- 相关：ADR-0001（以 LunaBox 建立 Windows 基线）、[ROADMAP](../ROADMAP.md)

## 背景

本仓库最初将上游 LunaBox 的 Linux 平台实现、Wine/Proton 工具链与打包资源
整体移除，仅保留 Windows（提交 `02158b5`、`8a55b8c` 等）。被移除的代码仍在
git 历史中完整保留，上游 LunaBox 至今（v1.13.x）仍在发布 `linux-amd64` 的
deb / rpm / AppImage，且相关实现与移除前基本一致。

现在需要在**本仓库**恢复 Linux 支持，范围限定 **amd64**，产物对齐上游。

## 决策

1. **承载方式**：在本仓库内恢复，不另建仓库、不建长期分支。仓库名保留，
   文档与产品描述改为「Windows / Linux（amd64）」。
2. **恢复方式**：从平台移除前的提交（`02158b5^`、`8a55b8c^`、`fb87c68^`）
   取回实现，再适配本仓库后续对共享代码的改动；不做整体 revert
   （避免把 macOS 分支一并带回来）。
3. **仅 amd64**：CI 与发布只产出 `linux/amd64`。代码内保留的上游 arm64
   分支（WebKit 安全模式等）不清理、不验证。
4. **不恢复 macOS**：共享代码 MUST NOT 新增 macOS 分支。
5. **不恢复 CLI**：`cmd/yukihubcli` 已作为产品决策移除，Linux 打包不再包含 CLI。
6. **Wails 补丁纳入构建链**：`scripts/patch-wails-linux-tray.sh` 负责 Linux 托盘
   与 WebKitGTK 渲染兼容，开发、绑定生成、打包与 CI 都必须调用。

## 影响

- `AGENTS.md` 的平台约束从「仅 Windows」改为「Windows + Linux（amd64），无 macOS」。
- CI 新增 Linux 作业；发布产物从 4 个 Windows 产物扩展为 7 个（+3 Linux）。
- **数据契约不变**：`wine_runner` / `wine_args` / `wine_prefix` 本就在移动端
  契约与数据库中保留，Linux 恢复只是重新使用它们。
- 应用内更新（`updater`）仍仅支持 Windows，与上游基线一致。

## 遗留与风险

- 完整 Linux 构建（DuckDB 的 CGO + GTK4 / WebKitGTK 6.0）需要在装有构建依赖的
  Linux 环境或 CI 上验证；`wails3 generate bindings` 需在正式构建机上重跑一次。
- `scripts/patch-wails-linux-tray.sh` 与 Wails 版本强耦合，升级 Wails 时必须同步更新。
