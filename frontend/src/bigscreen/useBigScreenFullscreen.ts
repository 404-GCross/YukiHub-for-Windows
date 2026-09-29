import { Window } from "@wailsio/runtime";
import { useEffect, useRef } from "react";

/**
 * 进入大屏模式时把窗口切成全屏，离开时还原。
 *
 * M0.1 决策：不扩展 Go 侧的 `wailsruntime.Runtime`（它只暴露 Show/Restore 与对话框），
 * 直接用 `@wailsio/runtime` 的 `Window` 全屏 API。理由：前端全仓已有这种直连用法
 * （`useAppRuntimeEffects`、`StartupWindow`），且无需重新生成绑定。
 *
 * 若进入大屏前窗口本来就不是全屏，退出时才还原；本来已是全屏（例如用户自己按了
 * 全屏键）则保持不动。
 */
export function useBigScreenFullscreen(enabled = true) {
  const shouldRestoreRef = useRef(false);

  useEffect(() => {
    if (!enabled) {
      return;
    }

    let active = true;
    shouldRestoreRef.current = false;

    void (async () => {
      try {
        const isFullscreen = await Window.IsFullscreen();
        if (!active || isFullscreen) {
          return;
        }
        shouldRestoreRef.current = true;
        await Window.Fullscreen();
      }
      catch (error) {
        console.error("Failed to enter big screen fullscreen:", error);
      }
    })();

    return () => {
      active = false;
      if (!shouldRestoreRef.current) {
        return;
      }
      shouldRestoreRef.current = false;
      void Window.UnFullscreen().catch((error: unknown) => {
        console.error("Failed to leave big screen fullscreen:", error);
      });
    };
  }, [enabled]);
}
