import type { BigScreenCategoryId } from "./categories";
import { memo } from "react";

import { useTranslation } from "react-i18next";
import { BIG_SCREEN_CATEGORIES } from "./categories";
import {
  BIG_SCREEN_RAIL_COLLAPSED_WIDTH,
  BIG_SCREEN_RAIL_EXPANDED_WIDTH,
} from "./constants";

interface BigScreenRailProps {
  activeCategory: BigScreenCategoryId;
  /** 焦点或鼠标进入侧栏时展开，离开后收回到图标条 */
  expanded: boolean;
  focused: boolean;
  focusedIndex: number;
  onFocusIndexChange: (index: number) => void;
  onSelect: (category: BigScreenCategoryId) => void;
}

/** 大屏左侧分类栏，对齐手机端 `bsRail`（收起只留图标，展开显示名称）。 */
export const BigScreenRail = memo(
  ({
    activeCategory,
    expanded,
    focused,
    focusedIndex,
    onFocusIndexChange,
    onSelect,
  }: BigScreenRailProps) => {
    const { t } = useTranslation();

    return (
      <nav
        className="flex h-full shrink-0 flex-col gap-1.5 overflow-hidden border-r border-white/8 bg-brand-900/60 py-6 backdrop-blur-sm transition-[width] duration-[220ms] ease-out"
        style={{
          width: expanded
            ? BIG_SCREEN_RAIL_EXPANDED_WIDTH
            : BIG_SCREEN_RAIL_COLLAPSED_WIDTH,
        }}
      >
        {BIG_SCREEN_CATEGORIES.map((category, index) => {
          const isActive = category.id === activeCategory;
          const isFocused = focused && index === focusedIndex;
          const label = t(category.labelKey);

          return (
            <button
              key={category.id}
              type="button"
              title={label}
              aria-current={isActive ? "true" : undefined}
              className={`mx-2 flex h-11 shrink-0 items-center gap-3 rounded-xl px-3 text-left transition-colors duration-150 ${
                isActive
                  ? "text-secondary-500"
                  : "text-brand-400 hover:text-white"
              } ${
                isFocused
                  ? "bg-secondary-500/16 ring-2 ring-secondary-500"
                  : "hover:bg-white/6"
              }`}
              onClick={() => onSelect(category.id)}
              onMouseEnter={() => onFocusIndexChange(index)}
            >
              <span
                className={`${category.icon} shrink-0 text-xl`}
                aria-hidden="true"
              />
              {expanded && (
                <span className="truncate text-sm font-medium">{label}</span>
              )}
            </button>
          );
        })}
      </nav>
    );
  },
);
