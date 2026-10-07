import { memo, useEffect, useRef } from "react";

/**
 * 大屏通用浮层菜单，对齐手机端 `BigScreenPanel`。
 *
 * 手机端用它承载三种浮层：游戏操作菜单（S4）、主菜单（S5）与各种选择器（S8）。
 * 结构与行为要点：
 * - 从右侧滑入，自带遮罩：遮罩**消费点击**，避免点到下面的卡片/侧栏/顶栏
 *   （手机端 M16 用户反馈"你是不是就做了层透明布"）；
 * - 条目支持图标 + 主文案 + 副文案 + 分隔线；
 * - 打开时**吞掉所有其它输入**，避免误操作到下层。
 *
 * 焦点下标由调用方持有（和货架 / 侧栏一样走大屏的意图分发），
 * 组件只负责渲染与滚动跟随，这样上下键不会出现两套状态。
 */

export interface BigScreenPanelItem {
  /** 图标（UnoCSS mdi 类名）；留空则不渲染图标位 */
  icon?: string;
  key: string;
  label: string;
  /** 副文案（手机端 Item.sub） */
  sub?: string;
  /** 分隔线：为 true 时忽略其余字段 */
  separator?: boolean;
}

interface BigScreenPanelProps {
  /** 底部提示文案（按键说明由调用方按当前输入设备给出） */
  hint: string;
  items: BigScreenPanelItem[];
  /** 当前焦点项在 `items` 里的下标（只会是可选中的条目） */
  focusedIndex: number;
  onActivate: (index: number) => void;
  onClose: () => void;
  onFocusIndexChange: (index: number) => void;
  title: string;
}

/** 第一个可选中的条目下标；没有可选项时返回 -1 */
export function firstFocusableIndex(items: BigScreenPanelItem[]): number {
  return items.findIndex(item => !item.separator);
}

/**
 * 在可选中的条目之间按 delta 循环移动（跳过分隔线）。
 * 只有一个可选条目时原地不动，避免"按了没反应但音效在响"。
 */
export function stepFocusableIndex(
  items: BigScreenPanelItem[],
  from: number,
  delta: number,
): number {
  const selectable = items
    .map((item, index) => (item.separator ? -1 : index))
    .filter(index => index >= 0);
  if (selectable.length === 0) {
    return -1;
  }
  const currentPosition = selectable.indexOf(from);
  const nextPosition
    = currentPosition < 0
      ? 0
      : (currentPosition + delta + selectable.length) % selectable.length;
  return selectable[nextPosition];
}

/** 「确认」时取当前项；焦点落在分隔线或越界时返回 null */
export function focusedPanelItem(
  items: BigScreenPanelItem[],
  focusedIndex: number,
): BigScreenPanelItem | null {
  const item = items[focusedIndex];
  return !item || item.separator ? null : item;
}

export const BigScreenPanel = memo(
  ({
    hint,
    items,
    focusedIndex,
    onActivate,
    onClose,
    onFocusIndexChange,
    title,
  }: BigScreenPanelProps) => {
    const rowRefs = useRef<Array<HTMLButtonElement | null>>([]);

    // 焦点滚进可视区：菜单条目超屏时靠它保证"看得见选了哪条"
    useEffect(() => {
      rowRefs.current[focusedIndex]?.scrollIntoView({
        block: "nearest",
        behavior: "smooth",
      });
    }, [focusedIndex]);

    return (
      <div className="absolute inset-0 z-40">
        {/* 遮罩：点空白 = 关闭，同时拦住落到下层的点击 */}
        <div
          className="absolute inset-0 animate-bigscreen-scrim-in bg-black/60"
          onClick={onClose}
          aria-hidden="true"
        />

        <div
          role="menu"
          aria-label={title}
          className="absolute right-10 top-1/2 flex max-h-[86vh] w-[min(360px,30vw)] -translate-y-1/2 animate-bigscreen-panel-in flex-col overflow-hidden rounded-2xl border border-white/12 bg-brand-900/94 shadow-2xl backdrop-blur-xl"
        >
          <div className="px-6 pb-2 pt-5 text-[11px] font-medium uppercase tracking-[0.18em] text-brand-500">
            {title}
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto px-3 pb-1">
            {items.map((item, index) => {
              if (item.separator) {
                return (
                  <div
                    key={item.key}
                    className="mx-3 my-2 h-px bg-white/12"
                    aria-hidden="true"
                  />
                );
              }

              const isFocused = index === focusedIndex;
              return (
                <button
                  key={item.key}
                  ref={(node) => {
                    rowRefs.current[index] = node;
                  }}
                  type="button"
                  role="menuitem"
                  className={`mx-1 my-0.5 flex min-h-11 w-[calc(100%-0.5rem)] items-center gap-2.5 rounded-lg px-3 py-2 text-left transition-all duration-150 ${
                    isFocused
                      ? "scale-[1.02] bg-white/12 ring-2 ring-secondary-500"
                      : "hover:bg-white/6"
                  }`}
                  onClick={() => onActivate(index)}
                  onMouseEnter={() => onFocusIndexChange(index)}
                >
                  {item.icon && (
                    <span
                      className={`${item.icon} shrink-0 text-lg ${
                        isFocused ? "text-secondary-500" : "text-brand-500"
                      }`}
                      aria-hidden="true"
                    />
                  )}
                  <span className="min-w-0 flex-1">
                    <span
                      className={`block truncate text-sm font-medium ${
                        isFocused ? "text-white" : "text-brand-200"
                      }`}
                    >
                      {item.label}
                    </span>
                    {item.sub && (
                      <span
                        className={`mt-0.5 block truncate text-xs ${
                          isFocused ? "text-brand-300" : "text-brand-500"
                        }`}
                      >
                        {item.sub}
                      </span>
                    )}
                  </span>
                </button>
              );
            })}
          </div>

          <div className="shrink-0 border-t border-white/8 px-6 py-3 text-xs text-brand-500">
            {hint}
          </div>
        </div>
      </div>
    );
  },
);
