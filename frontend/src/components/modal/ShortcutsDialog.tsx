import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import {
  GLOBAL_SHORTCUTS,
  SHORTCUT_DIALOG_EVENT,
} from "../../consts/shortcuts";
import { ModalPortal } from "../ui/ModalPortal";

/**
 * 快捷键速查弹窗。
 *
 * 用自定义事件打开，而不是往上提状态：快捷键在任何页面都能触发，
 * 若走 props 就得把 open 状态一路挂到根路由再传下来。
 */
export function ShortcutsDialog() {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = useState(false);

  useEffect(() => {
    const open = () => setIsOpen(true);
    window.addEventListener(SHORTCUT_DIALOG_EVENT, open);
    return () => window.removeEventListener(SHORTCUT_DIALOG_EVENT, open);
  }, []);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    const close = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsOpen(false);
      }
    };
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, [isOpen]);

  if (!isOpen) {
    return null;
  }

  const parts = [t("shortcuts.modifier"), t("shortcuts.shift")];

  return (
    <ModalPortal>
      <div
        className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
        onClick={() => setIsOpen(false)}
      >
        <div
          className="w-full max-w-md rounded-xl border border-brand-200 bg-white p-6 shadow-xl dark:border-brand-700 dark:bg-brand-800"
          onClick={event => event.stopPropagation()}
        >
          <div className="mb-4 flex items-center justify-between">
            <h3 className="text-xl font-bold text-brand-900 dark:text-white">
              {t("shortcuts.title")}
            </h3>
            <button
              type="button"
              onClick={() => setIsOpen(false)}
              aria-label={t("common.close")}
              className="rounded-lg p-1 text-2xl text-brand-500 hover:bg-brand-100 hover:text-brand-700 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200"
            >
              <span className="i-mdi-close" />
            </button>
          </div>

          <ul className="flex flex-col gap-2">
            {GLOBAL_SHORTCUTS.map(shortcut => (
              <li
                key={shortcut.id}
                className="flex items-center justify-between gap-4"
              >
                <span className="text-sm text-brand-700 dark:text-brand-200">
                  {t(`shortcuts.${shortcut.id}`)}
                </span>
                <span className="flex shrink-0 items-center gap-1">
                  {parts.map(part => (
                    <kbd
                      key={part}
                      className="rounded border border-brand-200 bg-brand-50 px-1.5 py-0.5 text-[11px] font-semibold text-brand-700 dark:border-brand-600 dark:bg-brand-900 dark:text-brand-200"
                    >
                      {part}
                    </kbd>
                  ))}
                  <kbd className="rounded border border-brand-200 bg-brand-50 px-1.5 py-0.5 text-[11px] font-semibold text-brand-700 dark:border-brand-600 dark:bg-brand-900 dark:text-brand-200">
                    {shortcut.key === "/" ? "/" : shortcut.key.toUpperCase()}
                  </kbd>
                </span>
              </li>
            ))}
          </ul>

          <p className="mt-4 text-xs text-brand-500 dark:text-brand-400">
            {t("shortcuts.hint")}
          </p>
        </div>
      </div>
    </ModalPortal>
  );
}
