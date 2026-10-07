import type { BigScreenCategoryId } from "./categories";
import { memo } from "react";

import { useTranslation } from "react-i18next";
import { BIG_SCREEN_CATEGORIES } from "./categories";
import {
  BIG_SCREEN_RAIL_COLLAPSED_WIDTH,
  BIG_SCREEN_RAIL_EXPANDED_WIDTH,
  resolveBigScreenEnterDelay,
} from "./constants";

interface BigScreenRailProps {
  activeCategory: BigScreenCategoryId;
  /** 各分类条目数（展开时显示，对齐手机端侧栏的计数徽标） */
  counts?: Partial<Record<BigScreenCategoryId, number>>;
  /** 入场错峰动画：只在首次进入大屏时开启 */
  entryAnimation?: boolean;
  /** 焦点或鼠标进入侧栏时展开，离开后收回到图标条 */
  expanded: boolean;
  focused: boolean;
  focusedIndex: number;
  onFocusIndexChange: (index: number) => void;
  onSelect: (category: BigScreenCategoryId) => void;
}

/** 大屏左侧分类栏，对齐手机端 `bsRail`（收起只留图标，展开显示名称与计数）。 */
export const BigScreenRail = memo(
  ({
    activeCategory,
    counts,
    entryAnimation = false,
    expanded,
    focused,
    focusedIndex,
    onFocusIndexChange,
    onSelect,
  }: BigScreenRailProps) => {
    const { t } = useTranslation();

    return (
      // 绝对定位 + z-30：展开时**浮在内容之上**，而不是把货架往右推。
      // 对齐手机端 `bsRail`（固定 72dp + `elevation=10dp`，展开靠叠放层实现），
      // 这样焦点进出侧栏、鼠标划过侧栏都不会让整排卡片左右跳。
      <nav
        className="absolute inset-y-0 left-0 z-30 flex flex-col gap-1 overflow-hidden border-r border-white/8 bg-brand-900/60 py-5 backdrop-blur-sm transition-[width] duration-[220ms] ease-out"
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
          const count = counts?.[category.id] ?? 0;

          return (
            <button
              key={category.id}
              type="button"
              title={label}
              aria-current={isActive ? "true" : undefined}
              className={`mx-1.5 flex h-10 shrink-0 items-center gap-2.5 rounded-lg px-2.5 text-left transition-colors duration-150 ${
                entryAnimation ? "animate-bigscreen-enter" : ""
              } ${
                isActive
                  ? "text-secondary-500"
                  : "text-brand-400 hover:text-white"
              } ${
                isFocused
                  ? "bg-secondary-500/16 ring-2 ring-secondary-500"
                  : "hover:bg-white/6"
              }`}
              style={{
                animationDelay: entryAnimation
                  ? `${resolveBigScreenEnterDelay(index)}ms`
                  : undefined,
              }}
              onClick={() => onSelect(category.id)}
              onMouseEnter={() => onFocusIndexChange(index)}
            >
              <span
                className={`${category.icon} shrink-0 text-lg`}
                aria-hidden="true"
              />
              {expanded && (
                <>
                  <span className="min-w-0 flex-1 truncate text-sm font-medium">
                    {label}
                  </span>
                  {count > 0 && (
                    <span className="shrink-0 text-xs tabular-nums text-brand-500">
                      {count}
                    </span>
                  )}
                </>
              )}
            </button>
          );
        })}
      </nav>
    );
  },
);
