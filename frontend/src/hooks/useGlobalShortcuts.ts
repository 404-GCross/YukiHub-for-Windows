import { useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";

import { GLOBAL_SHORTCUTS, SHORTCUT_DIALOG_EVENT } from "../consts/shortcuts";

function isEditableTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) {
    return false;
  }

  return (
    target.isContentEditable
    || target.tagName === "INPUT"
    || target.tagName === "TEXTAREA"
    || target.tagName === "SELECT"
  );
}

/**
 * 全局快捷键：Ctrl/Cmd + Shift + 单键。
 *
 * 与 `useAppZoom` 的 Ctrl+±/0 分开，互不干扰；在输入框里一律不触发，
 * 免得打字被导航打断。挂在整个路由树的根上，大屏模式下同样可用
 * （大屏自己只处理方向键等无修饰键的输入）。
 */
export function useGlobalShortcuts() {
  const navigate = useNavigate();

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const modifier = event.ctrlKey || event.metaKey;
      if (!modifier || !event.shiftKey || event.altKey) {
        return;
      }
      if (isEditableTarget(event.target)) {
        return;
      }

      const key = event.key.toLowerCase();
      // 斜杠这类键在按住 Shift 后 event.key 会变成 `?`，只能按 code 匹配
      const matched = GLOBAL_SHORTCUTS.find(shortcut =>
        shortcut.code ? shortcut.code === event.code : shortcut.key === key,
      );
      if (!matched) {
        return;
      }

      event.preventDefault();
      if (matched.path) {
        void navigate({ to: matched.path });
        return;
      }
      window.dispatchEvent(new CustomEvent(SHORTCUT_DIALOG_EVENT));
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [navigate]);
}
