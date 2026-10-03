import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

/**
 * `KeyboardEvent.code` → 后端 accelerator 的键名。
 *
 * 用 code 而不是 key：Shift 会改变 key（` 变 ~、1 变 !），用 code 才能稳定
 * 对应到「用户按的那个物理键」。键名必须和后端允许的集合一致
 * （Go 侧 internal/service/overlay_shortcut.go 的 overlayShortcutKeyLabels），
 * 否则保存时才报「不支持的按键」。
 */
const CODE_TO_KEY: Record<string, string> = {
  Backquote: "`",
  Minus: "-",
  Equal: "=",
  BracketLeft: "[",
  BracketRight: "]",
  Backslash: "\\",
  Semicolon: ";",
  Quote: "'",
  Comma: ",",
  Period: ".",
  Slash: "/",
  Space: "space",
  Tab: "tab",
  Enter: "return",
  Backspace: "backspace",
  Delete: "delete",
  Home: "home",
  End: "end",
  PageUp: "page up",
  PageDown: "page down",
  NumLock: "numlock",
  ArrowLeft: "left",
  ArrowRight: "right",
  ArrowUp: "up",
  ArrowDown: "down",
};

/** 修饰键的展示名（顺序与后端规范化后的顺序一致）。 */
const MODIFIER_LABELS: Array<{ key: string; label: string }> = [
  { key: "ctrl", label: "Ctrl" },
  { key: "alt", label: "Alt" },
  { key: "shift", label: "Shift" },
  { key: "cmd", label: "Win" },
];

/** 纯修饰键本身，按下它们不算「选定了按键」。 */
const PURE_MODIFIER_CODES = new Set([
  "ShiftLeft",
  "ShiftRight",
  "ControlLeft",
  "ControlRight",
  "AltLeft",
  "AltRight",
  "MetaLeft",
  "MetaRight",
]);

function keyNameFromCode(code: string): string | null {
  if (/^Key[A-Z]$/.test(code)) {
    return code.slice(3).toLowerCase();
  }
  if (/^Digit\d$/.test(code)) {
    return code.slice(5);
  }
  if (/^F(?:[1-9]|1\d|2[0-4])$/.test(code)) {
    return code.toLowerCase();
  }
  return CODE_TO_KEY[code] ?? null;
}

function pressedModifiers(event: KeyboardEvent): string[] {
  const modifiers: string[] = [];
  if (event.ctrlKey) {
    modifiers.push("ctrl");
  }
  if (event.altKey) {
    modifiers.push("alt");
  }
  if (event.shiftKey) {
    modifiers.push("shift");
  }
  if (event.metaKey) {
    modifiers.push("cmd");
  }
  return modifiers;
}

function modifierPreview(modifiers: string[]): string {
  return modifiers
    .map(
      key => MODIFIER_LABELS.find(item => item.key === key)?.label ?? key,
    )
    .join(" + ");
}

interface ShortcutRecorderProps {
  /** 当前组合的展示文本，如 “Shift + ~”；为空表示还没拿到 */
  display: string;
  /** 正在录制时按住的部分，如 “Ctrl + Shift” */
  onRecord: (accelerator: string) => void;
  disabled?: boolean;
}

/**
 * 快捷键录制控件：点一下进入录制，按下的组合会被翻译成后端的 accelerator 形式。
 *
 * 为什么要自己录而不是给个输入框：快捷键的写法（Ctrl / Win / ~ / PageUp）
 * 让用户手打必然出错，而且打错了要到保存时才报「不支持的按键」。
 */
export function ShortcutRecorder({
  display,
  onRecord,
  disabled = false,
}: ShortcutRecorderProps) {
  const { t } = useTranslation();
  const [recording, setRecording] = useState(false);
  const [holding, setHolding] = useState("");
  const [hint, setHint] = useState<string | null>(null);
  const containerRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    if (!recording) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      // 录制期间吃掉所有按键：不然会触发应用自身的快捷键（Ctrl+Shift+H 之类）
      event.preventDefault();
      event.stopPropagation();

      // 什么都不按的 Esc 用来取消；Ctrl+Esc 之类仍算组合键
      if (event.key === "Escape" && pressedModifiers(event).length === 0) {
        setRecording(false);
        setHolding("");
        setHint(null);
        return;
      }

      const modifiers = pressedModifiers(event);
      setHolding(modifierPreview(modifiers));

      if (PURE_MODIFIER_CODES.has(event.code)) {
        // 还只按着修饰键，等真正的那个键
        return;
      }

      const key = keyNameFromCode(event.code);
      if (!key) {
        setHint(t("settings.basic.overlayShortcutUnsupportedKey"));
        return;
      }
      if (modifiers.length === 0) {
        setHint(t("settings.basic.overlayShortcutNeedsModifier"));
        return;
      }

      setRecording(false);
      setHolding("");
      setHint(null);
      onRecord([...modifiers, key].join("+"));
    };

    const handleKeyUp = (event: KeyboardEvent) => {
      // 松开修饰键后把预览收回去，免得显示一个已经松开的组合
      setHolding(modifierPreview(pressedModifiers(event)));
    };

    window.addEventListener("keydown", handleKeyDown, true);
    window.addEventListener("keyup", handleKeyUp, true);
    return () => {
      window.removeEventListener("keydown", handleKeyDown, true);
      window.removeEventListener("keyup", handleKeyUp, true);
    };
  }, [recording, onRecord, t]);

  const handleBlur = useCallback(() => {
    setRecording(false);
    setHolding("");
    setHint(null);
  }, []);

  const label = recording
    ? holding
      ? `${holding} + …`
      : t("settings.basic.overlayShortcutRecording")
    : display;

  return (
    <div className="space-y-1">
      <div className="flex items-center gap-2">
        <button
          ref={containerRef}
          type="button"
          disabled={disabled}
          onClick={() => {
            setHint(null);
            setRecording(current => !current);
          }}
          onBlur={handleBlur}
          className={`min-w-[160px] rounded-lg border px-3 py-2 text-left text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
            recording
              ? "border-brand-400 bg-brand-50 text-brand-700 dark:border-brand-400 dark:bg-brand-800 dark:text-brand-100"
              : "border-brand-200 bg-white text-brand-700 hover:border-brand-300 dark:border-brand-700 dark:bg-brand-800 dark:text-brand-200"
          }`}
        >
          <span className="font-mono">{label}</span>
        </button>
      </div>
      {hint ? (
        <p className="text-xs text-amber-600 dark:text-amber-400">{hint}</p>
      ) : null}
    </div>
  );
}
