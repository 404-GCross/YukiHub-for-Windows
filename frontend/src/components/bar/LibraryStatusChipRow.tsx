import type { GameStatusFilter } from "../../consts/options";
import { useTranslation } from "react-i18next";
import { GAME_STATUS_BADGE_STYLES } from "../../consts/gameStatusBadge";
import { statusOptions } from "../../consts/options";

interface LibraryStatusChipRowProps {
  statusFilter: GameStatusFilter;
  onStatusFilterChange: (value: GameStatusFilter) => void;
}

/** 手机版游戏库顶部状态筛选芯片行（PC 版横向平铺，对齐 bg_chip）。 */
export function LibraryStatusChipRow({
  statusFilter,
  onStatusFilterChange,
}: LibraryStatusChipRowProps) {
  const { t } = useTranslation();

  return (
    <div className="-mx-1 flex items-center gap-2 overflow-x-auto px-1 py-1">
      {statusOptions.map((option) => {
        const active = (statusFilter || "") === option.value;
        const badge = option.value
          ? GAME_STATUS_BADGE_STYLES[option.value]
          : undefined;
        return (
          <button
            key={`status-chip-${option.value || "all"}`}
            type="button"
            aria-pressed={active}
            onClick={() => onStatusFilterChange(option.value)}
            className={
              active
                ? "yh-primary-pill inline-flex shrink-0 items-center gap-1.5 rounded-full px-3.5 py-1.5 text-xs font-bold text-white shadow-md transition-transform active:scale-95"
                : "yh-chip shrink-0 cursor-pointer px-3.5 py-1.5 text-xs font-bold transition-colors hover:border-primary-300 hover:bg-white dark:hover:bg-[#22314A]"
            }
          >
            {badge && (
              <span className={`${badge.icon} text-sm`} aria-hidden="true" />
            )}
            {t(option.label)}
          </button>
        );
      })}
    </div>
  );
}
