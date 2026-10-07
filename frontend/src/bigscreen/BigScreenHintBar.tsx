import type { BigScreenKeyStyle } from "./keyStyles";
import type { BigScreenInputDevice } from "./useGamepad";
import { memo } from "react";
import { useTranslation } from "react-i18next";

import { KEYBOARD_GLYPHS, resolveBigScreenKeyGlyphs } from "./keyStyles";

/** 提示条模式，与 `bigscreen_hint_mode` 一致 */
export type BigScreenHintMode = "always" | "auto" | "off";

interface BigScreenHintBarProps {
  /** 布局类名（间距 / 内边距由调用方决定） */
  className?: string;
  /** 提示条模式：auto 4s 后淡出 / always 常显 / off 隐藏 */
  hintMode?: BigScreenHintMode;
  /** 最近一次使用的输入设备：手柄显示 A/B/X/Y，键盘显示按键 */
  inputDevice: BigScreenInputDevice;
  /** 按键图标风格（Xbox / PlayStation），与 `bigscreen_key_style` 一致 */
  keyStyle?: BigScreenKeyStyle;
  /** main：货架与外层的操作；details：详情层内的操作 */
  variant: "details" | "main";
}

interface HintItem {
  badge: string;
  label: string;
}

/**
 * 大屏底栏按键提示，对齐手机端底栏按键提示条：
 * 按当前输入设备切换手柄图标与键盘按键，手柄字形随按键风格（Xbox / PS）变化。
 */
export const BigScreenHintBar = memo(
  ({
    className,
    hintMode = "auto",
    inputDevice,
    keyStyle = "xbox",
    variant,
  }: BigScreenHintBarProps) => {
    const { t } = useTranslation();
    if (hintMode === "off") {
      return null;
    }

    const isGamepad = inputDevice === "gamepad";
    const keys = resolveBigScreenKeyGlyphs(keyStyle);
    let items: HintItem[] = [];

    if (variant === "main" && isGamepad) {
      items = [
        { badge: "✚", label: t("bigScreen.hintMove") },
        { badge: keys.confirm, label: t("bigScreen.hintConfirm") },
        { badge: keys.third, label: t("bigScreen.hintFavorite") },
        { badge: keys.fourth, label: t("bigScreen.hintDetails") },
        { badge: "←", label: t("bigScreen.hintCategoryRail") },
        { badge: `${keys.lb}/${keys.rb}`, label: t("bigScreen.hintCategory") },
        { badge: keys.menu, label: t("bigScreen.hintMenu") },
        { badge: keys.back, label: t("bigScreen.hintExit") },
      ];
    }
    else if (variant === "main") {
      items = [
        { badge: KEYBOARD_GLYPHS.move, label: t("bigScreen.hintMove") },
        { badge: KEYBOARD_GLYPHS.confirm, label: t("bigScreen.hintConfirm") },
        { badge: KEYBOARD_GLYPHS.favorite, label: t("bigScreen.hintFavorite") },
        { badge: KEYBOARD_GLYPHS.details, label: t("bigScreen.hintDetails") },
        { badge: KEYBOARD_GLYPHS.menu, label: t("bigScreen.hintMenu") },
        { badge: KEYBOARD_GLYPHS.back, label: t("bigScreen.hintExit") },
      ];
    }
    else if (isGamepad) {
      items = [
        { badge: "✚", label: t("bigScreen.hintSwitchButton") },
        { badge: keys.confirm, label: t("bigScreen.hintConfirm") },
        { badge: keys.third, label: t("bigScreen.hintFavorite") },
        { badge: keys.back, label: t("bigScreen.hintBack") },
      ];
    }
    else {
      items = [
        { badge: KEYBOARD_GLYPHS.move, label: t("bigScreen.hintSwitchButton") },
        { badge: KEYBOARD_GLYPHS.confirm, label: t("bigScreen.hintConfirm") },
        { badge: KEYBOARD_GLYPHS.favorite, label: t("bigScreen.hintFavorite") },
        { badge: KEYBOARD_GLYPHS.back, label: t("bigScreen.hintBack") },
      ];
    }

    return (
      <div
        className={`flex flex-wrap items-center gap-6 text-xs text-brand-400 ${
          hintMode === "auto" ? "animate-bigscreen-hint-dim" : ""
        } ${className ?? ""}`}
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
