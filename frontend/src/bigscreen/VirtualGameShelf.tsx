import type { models } from "../../src/bindings/models";
import { useVirtualizer } from "@tanstack/react-virtual";
import { memo, useEffect, useRef } from "react";

import { GameCard } from "../components/card/GameCard";
import { BIG_SCREEN_CARD_GAP, resolveBigScreenCardWidth } from "./constants";

interface VirtualGameShelfProps {
  /** 焦点是否落在这个区域（用于决定要不要画焦点环） */
  focused: boolean;
  focusedIndex: number;
  games: models.Game[];
  onActivate: (game: models.Game) => void;
  onFocusIndexChange: (index: number) => void;
  onViewDetails: (game: models.Game) => void;
  /** 货架可视高度（px） */
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
    focused,
    focusedIndex,
    games,
    onActivate,
    onFocusIndexChange,
    onViewDetails,
    rowHeight,
  }: VirtualGameShelfProps) => {
    const scrollRef = useRef<HTMLDivElement | null>(null);
    const cardWidth = resolveBigScreenCardWidth(rowHeight);

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
                style={{ height: rowHeight, width: cardWidth }}
              >
                <div
                  className="relative transition-transform duration-[140ms] ease-out"
                  style={{
                    transform: isFocused ? "scale(1.045)" : undefined,
                    width: cardWidth,
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
            );
          })}
        </div>
      </div>
    );
  },
);
