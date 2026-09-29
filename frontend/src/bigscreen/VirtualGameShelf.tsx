import type { models } from "../../src/bindings/models";
import { useVirtualizer } from "@tanstack/react-virtual";
import { memo, useEffect, useRef } from "react";

import { GameCard } from "../components/card/GameCard";
import { BIG_SCREEN_CARD_GAP, resolveBigScreenEnterDelay } from "./constants";

interface VirtualGameShelfProps {
  /** 卡片宽度（px），由 `resolveBigScreenShelfMetrics` 统一算好传入 */
  cardWidth: number;
  /** 入场错峰动画：只在首次进入大屏时开启，滚动新挂载的卡片不再重放 */
  entryAnimation?: boolean;
  /** 焦点是否落在这个区域（用于决定要不要画焦点环） */
  focused: boolean;
  focusedIndex: number;
  games: models.Game[];
  onActivate: (game: models.Game) => void;
  onFocusIndexChange: (index: number) => void;
  onViewDetails: (game: models.Game) => void;
  /** 卡片行高度（px，含焦点缩放的上下余量），卡片在其中垂直居中 */
  rowHeight: number;
}

/**
 * 大屏模式的横向单排货架。
 *
 * 现有的 `VirtualGameGrid` 是纵向虚拟化，横向货架在 `@tanstack/react-virtual`
 * 上不共用同一套行列计算，所以单独实现；卡片本体仍复用 `GameCard`。
 */
export const VirtualGameShelf = memo(
  ({
    cardWidth,
    entryAnimation = false,
    focused,
    focusedIndex,
    games,
    onActivate,
    onFocusIndexChange,
    onViewDetails,
    rowHeight,
  }: VirtualGameShelfProps) => {
    const scrollRef = useRef<HTMLDivElement | null>(null);

    const virtualizer = useVirtualizer({
      count: games.length,
      estimateSize: () => cardWidth + BIG_SCREEN_CARD_GAP,
      getScrollElement: () => scrollRef.current,
      horizontal: true,
      overscan: 4,
    });

    useEffect(() => {
      virtualizer.measure();
    }, [cardWidth, virtualizer]);

    useEffect(() => {
      if (focusedIndex < 0 || focusedIndex >= games.length) {
        return;
      }
      virtualizer.scrollToIndex(focusedIndex, { align: "auto" });
    }, [focusedIndex, games.length, virtualizer]);

    return (
      <div
        ref={scrollRef}
        className="scrollbar-hide h-full w-full overflow-x-auto overflow-y-hidden"
      >
        <div
          className="relative h-full"
          style={{ width: virtualizer.getTotalSize() }}
        >
          {virtualizer.getVirtualItems().map((virtualItem) => {
            const game = games[virtualItem.index];
            if (!game) {
              return null;
            }

            const isFocused = focused && virtualItem.index === focusedIndex;
            return (
              <div
                key={virtualItem.key}
                className="absolute left-0 top-0 flex items-center"
                style={{
                  height: rowHeight,
                  // 横向虚拟化：偏移必须由 virtualItem.start 给出，
                  // 少了这一行所有卡片都会叠在 left:0 上，一屏只看得到一张
                  transform: `translateX(${virtualItem.start}px)`,
                  width: cardWidth,
                }}
              >
                {/*
                  定位（translateX）与入场动画必须分在两层：动画的关键帧也写
                  `transform`，且 fill-mode 是 both，同层会把定位覆盖掉
                  （动画结束后依然生效，卡片会一直叠在原点）。
                */}
                <div
                  className={`w-full ${entryAnimation ? "animate-bigscreen-enter" : ""}`}
                  style={{
                    animationDelay: entryAnimation
                      ? `${resolveBigScreenEnterDelay(virtualItem.index)}ms`
                      : undefined,
                  }}
                >
                  <div
                    className="relative transition-transform duration-[140ms] ease-out"
                    style={{
                      transform: isFocused ? "scale(1.045)" : undefined,
                      zIndex: isFocused ? 10 : undefined,
                    }}
                    onMouseEnter={() => onFocusIndexChange(virtualItem.index)}
                  >
                    <GameCard
                      game={game}
                      cardLayout="portrait"
                      onActivate={onActivate}
                      onViewDetails={onViewDetails}
                    />
                    {isFocused && (
                      <div
                        className="pointer-events-none absolute inset-0 z-20 rounded-xl ring-3 ring-secondary-500"
                        aria-hidden="true"
                      />
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    );
  },
);
