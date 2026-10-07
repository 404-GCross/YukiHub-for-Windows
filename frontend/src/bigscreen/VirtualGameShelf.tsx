import type { models } from "../../src/bindings/models";
import { useVirtualizer } from "@tanstack/react-virtual";
import { memo, useEffect, useRef } from "react";

import { BigScreenCard } from "./BigScreenCard";
import { BIG_SCREEN_CARD_GAP, resolveBigScreenEnterDelay } from "./constants";

interface VirtualGameShelfProps {
  /** 卡片宽度（px），由 `resolveBigScreenShelfMetrics` 统一算好传入 */
  cardWidth: number;
  /** 封面高度（px） */
  coverHeight: number;
  /** 入场错峰动画：只在首次进入大屏时开启，滚动新挂载的卡片不再重放 */
  entryAnimation?: boolean;
  /** 焦点缩放幅度（%），0 表示只描边 */
  focusScale: number;
  /** 焦点是否落在这个区域（用于决定要不要画焦点环） */
  focused: boolean;
  focusedIndex: number;
  /** 已收藏的游戏 id，用来给卡片画收藏角标 */
  favoriteIds: Set<string>;
  games: models.Game[];
  onActivate: (game: models.Game) => void;
  onDetails: (game: models.Game) => void;
  onFocusIndexChange: (index: number) => void;
  /** 卡片行高度（px，含焦点缩放的上下余量），卡片在其中垂直居中 */
  rowHeight: number;
  /** 卡片下方是否再显示游戏名 */
  showTitles: boolean;
}

/**
 * 大屏模式的横向单排货架。
 *
 * 现有的 `VirtualGameGrid` 是纵向虚拟化，横向货架在 `@tanstack/react-virtual`
 * 上不共用同一套行列计算，所以单独实现；卡片本体用大屏专用的 `BigScreenCard`。
 */
export const VirtualGameShelf = memo(
  ({
    cardWidth,
    coverHeight,
    entryAnimation = false,
    focusScale,
    focused,
    focusedIndex,
    favoriteIds,
    games,
    onActivate,
    onDetails,
    onFocusIndexChange,
    rowHeight,
    showTitles,
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
      // 高度必须是「行高」而不是 h-full：写成 h-full 时这个 flex 子项会
      // flex-shrink 吃掉整个剩余空间，父级的 justify-end 就失效了，
      // 卡片会停在区域顶部而不是贴底。
      //
      // px-2 是给焦点缩放留的余量：容器是 overflow-x-auto，第一张卡放大后
      // 会往左溢出几像素，没有这段内边距就会被裁掉（表现为「卡片左边看不见」）。
      // 外层用 -mx-2 抵消，保证卡片左沿与标题左沿仍然对齐。
      //
      // pt-4 是垂直方向的同款余量（overflow-y 必然退化成 hidden）。
      <div
        ref={scrollRef}
        className="scrollbar-hide w-full overflow-x-auto overflow-y-hidden px-2 pt-4"
        style={{ height: rowHeight }}
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
                className="absolute left-0 top-0 flex items-start"
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
                  <BigScreenCard
                    coverHeight={coverHeight}
                    focused={isFocused}
                    focusScale={focusScale}
                    game={game}
                    isFavorite={favoriteIds.has(game.id)}
                    onActivate={() => onActivate(game)}
                    onDetails={() => onDetails(game)}
                    onFocus={() => onFocusIndexChange(virtualItem.index)}
                    showTitle={showTitles}
                  />
                </div>
              </div>
            );
          })}
        </div>
      </div>
    );
  },
);
