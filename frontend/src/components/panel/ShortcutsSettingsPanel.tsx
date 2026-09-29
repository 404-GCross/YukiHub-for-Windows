import { useTranslation } from "react-i18next";
import {
  GLOBAL_SHORTCUTS,
  SHORTCUT_DIALOG_EVENT,
} from "../../consts/shortcuts";
import { BetterButton } from "../ui/better/BetterButton";

/** 快捷键速查（只读）：快捷键本身不可配置，这里负责让它能被发现。 */
export function ShortcutsSettingsPanel() {
  const { t } = useTranslation();

  return (
    <div className="space-y-4">
      <p className="text-xs text-brand-500 dark:text-brand-400">
        {t("shortcuts.hint")}
      </p>

      <ul className="flex flex-col gap-2">
        {GLOBAL_SHORTCUTS.map(shortcut => (
          <li
            key={shortcut.id}
            className="flex items-center justify-between gap-4"
          >
            <span className="text-sm text-brand-700 dark:text-brand-200">
              {t(`shortcuts.${shortcut.id}`)}
            </span>
            <span className="flex shrink-0 items-center gap-1 text-[11px] font-semibold text-brand-600 dark:text-brand-300">
              <kbd className="rounded border border-brand-200 bg-brand-50 px-1.5 py-0.5 dark:border-brand-600 dark:bg-brand-900">
                {t("shortcuts.modifier")}
              </kbd>
              <span aria-hidden="true">+</span>
              <kbd className="rounded border border-brand-200 bg-brand-50 px-1.5 py-0.5 dark:border-brand-600 dark:bg-brand-900">
                {t("shortcuts.shift")}
              </kbd>
              <span aria-hidden="true">+</span>
              <kbd className="rounded border border-brand-200 bg-brand-50 px-1.5 py-0.5 dark:border-brand-600 dark:bg-brand-900">
                {shortcut.key === "/" ? "/" : shortcut.key.toUpperCase()}
              </kbd>
            </span>
          </li>
        ))}
      </ul>

      <BetterButton
        variant="secondary"
        onClick={() =>
          window.dispatchEvent(new CustomEvent(SHORTCUT_DIALOG_EVENT))}
      >
        <span className="i-mdi-keyboard-outline text-base" aria-hidden="true" />
        {t("shortcuts.openCheatsheet")}
      </BetterButton>
    </div>
  );
}
