# 变更流程与自检清单

## Agent 变更流程（推荐顺序）

当被要求"实现一个功能/修复一个问题"时，按此顺序执行：

1. **定位落点**：优先在现有 route/service/utils 中找最贴近的位置（参考 [anchors.md](anchors.md)）
2. **复用模式**：沿用同目录已有写法（migration 幂等、service 注入、UI glass/dark）
3. **实现最小改动**：避免新增大层级抽象
4. **本地验证**：尽量运行已有 build task
5. **自检**：对照下方清单

---

## 自检清单（交付前）

### 前端

- [ ] 新增/修改的组件在 `light` 与 `dark` 下都可读
- [ ] 背景开启时 `data-glass="true"` 下观感不崩
- [ ] 没有把局部样式硬塞进 `style.css`（除非是全局不可避免项）
- [ ] 新增组件遵守 HeadlessUI/Radix 封装约束，没有直接引入大型 UI 框架
- [ ] 页面最外层盒子没有设置颜色与不透明度
- [ ] 应用级副作用是否已优先抽到 `frontend/src/hooks/`，而不是把复杂 `useEffect` 堆在 `App.tsx`

### 后端

- [ ] 涉及 schema 变更时同时更新 `InitSchema` + 新 migration
- [ ] Migration 幂等且事务安全，空库也能跑通
- [ ] 没有引入 macOS 平台的系统调用；Linux 分支必须落在 `_linux.go` 或带显式
      `runtime.GOOS == "linux"` 判断
- [ ] 底层操作优先 Wails/Go 标准库
- [ ] 若新增/修改 `AppConfig` 字段，已同步检查 `LoadConfig/SaveConfig` 与 `ConfigService.UpdateAppConfig(...)`
- [ ] 若修改退出逻辑，已核对 `OnBeforeClose` / 前端退出流 / `OnShutdown` 的职责边界，没有把交互流程塞进 `OnShutdown`

### 通用

- [ ] 新增工具函数前已搜索并复用现有 `frontend/src/utils` 或 `internal/utils`
- [ ] 没有顺手重构或格式化不相关代码

---

### Linux 渲染验证

开发、绑定生成和打包流程都会调用 `scripts/patch-wails-linux-tray.sh`。该脚本除了
托盘修复，还会让 amd64 GTK4 构建保留 WebKit 的默认 GPU 渲染路径，并在创建 WebView
时，通过 WebKitGTK 2.42 起提供的公开 feature API 关闭
`PreferPageRenderingUpdatesNear60FPS`。直接使用 `go build` 前需要手动运行该脚本；
设置 `YUKIHUB_WEBKIT_PREFER_60FPS=1` 可以保留引擎默认偏好以便对比。

Wails beta.24 的 `application_linux.go` 在包初始化时检测 NVIDIA，并自动设置
`WEBKIT_DISABLE_DMABUF_RENDERER=1`。YukiHub 补丁在 amd64 上跳过这项自动禁用，
用户显式设置的环境变量仍然有效；arm64 保留原有兼容措施。若 NVIDIA 环境出现白屏
或驱动问题，可用 `WEBKIT_DISABLE_DMABUF_RENDERER=1` 恢复上游兼容行为。补丁需要
重新构建并重启应用后生效，不能改变已经启动的 WebKit 子进程。

补丁还针对 NVIDIA amd64 + 运行时 WebKitGTK 2.52 默认设置
`WEBKIT_SKIA_GPU_PAINTING_THREADS=0`，规避驱动与 Skia 资源释放路径的崩溃；
其他 WebKit 版本不自动设置该变量。完整原理与实测记录见
`scripts/patch-wails-linux-tray.sh` 的注释与仓库历史文档。
