import type { BigScreenInputDevice } from "./useGamepad";
import { memo } from "react";
import { useTranslation } from "react-i18next";

interface BigScreenHintBarProps {
  /** 布局类名（间距 / 内边距由调用方决定） */
  className?: string;
  /** 最近一次使用的输入设备：手柄显示 A/B/X/Y，键盘显示按键 */
  inputDevice: BigScreenInputDevice;
  /** main：货架与外层的操作；details：详情层内的操作 */
  variant: "details" | "main";
}

interface HintItem {
  badge: string;
  label: string;
}

/**
 * 大屏底栏按键提示，对齐手机端底栏按键提示条：按当前输入设备切换手柄图标与键盘按键。
 * 两个变体（外层货架 / 详情层内）共用同一套样式，避免两处各写一遍。
 */
export const BigScreenHintBar = memo(
  ({ className, inputDevice, variant }: BigScreenHintBarProps) => {
    const { t } = useTranslation();
    const isGamepad = inputDevice === "gamepad";
    let items: HintItem[] = [];

    if (variant === "main" && isGamepad) {
      items = [
        { badge: "✚", label: t("bigScreen.hintMove") },
        { badge: "A", label: t("bigScreen.hintConfirm") },
        { badge: "X", label: t("bigScreen.hintFavorite") },
        { badge: "Y", label: t("bigScreen.hintDetails") },
        { badge: "LB/RB", label: t("bigScreen.hintCategory") },
        { badge: "B", label: t("bigScreen.hintExit") },
      ];
    }
    else if (variant === "main") {
      items = [
        { badge: "← →", label: t("bigScreen.hintMove") },
        { badge: "Enter", label: t("bigScreen.hintConfirm") },
        { badge: "Esc", label: t("bigScreen.hintExit") },
      ];
    }
    else if (isGamepad) {
      items = [
        { badge: "✚", label: t("bigScreen.hintSwitchButton") },
        { badge: "A", label: t("bigScreen.hintConfirm") },
        { badge: "B", label: t("bigScreen.hintBack") },
      ];
    }
    else {
      items = [
        { badge: "← →", label: t("bigScreen.hintSwitchButton") },
        { badge: "Enter", label: t("bigScreen.hintConfirm") },
        { badge: "Esc", label: t("bigScreen.hintBack") },
      ];
    }

    return (
      <div
        className={`flex flex-wrap items-center gap-6 text-xs text-brand-400 ${
          className ?? ""
        }`}
      >
        {items.map(item => (
          <span key={item.label} className="inline-flex items-center gap-2">
            <kbd
              className={`rounded border border-brand-700 px-1.5 py-0.5 text-center font-sans ${
                isGamepad ? "min-w-6" : ""
              }`}
            >
              {item.badge}
            </kbd>
            {item.label}
          </span>
        ))}
      </div>
    );
  },
);
