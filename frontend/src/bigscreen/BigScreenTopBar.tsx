import { memo, useEffect, useState } from "react";

import { useTranslation } from "react-i18next";

interface BigScreenTopBarProps {
  /** 是否已连接手柄（决定状态文案与图标颜色） */
  gamepadConnected: boolean;
  /** 打开主菜单（☰ / Tab / START） */
  onOpenMenu: () => void;
}

/** 时钟刷新间隔：分钟粒度的显示不需要每秒重绘 */
const CLOCK_INTERVAL_MS = 20_000;

function formatClock(date: Date) {
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${hours}:${minutes}`;
}

/**
 * 大屏顶栏，对齐手机端 `bsTopBar`：左侧时间，右侧手柄连接状态与 ☰ 主菜单入口。
 *
 * 桌面端没有电量 / 网络 / 触摸模式这些概念，只保留对"客厅大屏"真正有意义的两项。
 * 顶栏不参与焦点引擎（菜单本身有独立浮层），但鼠标可以直接点 ☰。
 */
export const BigScreenTopBar = memo(
  ({ gamepadConnected, onOpenMenu }: BigScreenTopBarProps) => {
    const { t } = useTranslation();
    const [clock, setClock] = useState(() => formatClock(new Date()));

    useEffect(() => {
      const timer = window.setInterval(() => {
        setClock(formatClock(new Date()));
      }, CLOCK_INTERVAL_MS);
      return () => window.clearInterval(timer);
    }, []);

    return (
      <div className="pointer-events-none flex shrink-0 items-center justify-between px-8 pt-4 text-xs text-brand-400">
        <div className="flex items-center gap-4">
          <span className="font-medium tabular-nums text-brand-200">
            {clock}
          </span>
        </div>

        <div className="pointer-events-auto flex items-center gap-4">
          <span
            className={`inline-flex items-center gap-2 rounded-full border px-3 py-1 ${
              gamepadConnected
                ? "border-emerald-500/40 text-emerald-300"
                : "border-white/10 text-brand-500"
            }`}
          >
            <span
              className="i-mdi-gamepad-variant-outline text-sm"
              aria-hidden="true"
            />
            {gamepadConnected
              ? t("bigScreen.gamepadConnected")
              : t("bigScreen.gamepadDisconnected")}
          </span>

          <button
            type="button"
            onClick={onOpenMenu}
            title={t("bigScreen.mainMenu")}
            aria-label={t("bigScreen.mainMenu")}
            className="inline-flex h-9 items-center gap-2 rounded-full border border-white/12 bg-white/6 px-4 text-sm text-brand-200 transition-colors duration-150 hover:bg-white/12 hover:text-white"
          >
            <span className="i-mdi-menu text-base" aria-hidden="true" />
            {t("bigScreen.menu")}
          </button>
        </div>
      </div>
    );
  },
);
