import type { models } from "../../../src/bindings/models";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { formatDurationCompact } from "../../utils/time";
import { GameCoverImage } from "../ui/GameCoverImage";

export interface HomeRailGame {
  game: models.Game;
  isPlaying: boolean;
  totalPlayedDur: number;
}

interface HomeQuickLaunchRailProps {
  activeGameId: string | null;
  games: HomeRailGame[];
  onLaunchGame: (game: models.Game) => void;
  onOpenDetail: (gameId: string) => void;
  onSelectGame: (gameId: string) => void;
  onViewAll: () => void;
}

const SCROLL_STEP_PX = 320;

export function HomeQuickLaunchRail({
  activeGameId,
  games,
  onLaunchGame,
  onOpenDetail,
  onSelectGame,
  onViewAll,
}: HomeQuickLaunchRailProps) {
  const { t } = useTranslation();
  const scrollerRef = useRef<HTMLDivElement | null>(null);
  const [canScrollPrev, setCanScrollPrev] = useState(false);
  const [canScrollNext, setCanScrollNext] = useState(false);

  const syncScrollState = useCallback(() => {
    const node = scrollerRef.current;
    if (!node) {
      return;
    }
    const maxScrollLeft = node.scrollWidth - node.clientWidth;
    setCanScrollPrev(node.scrollLeft > 1);
    setCanScrollNext(maxScrollLeft > 1 && node.scrollLeft < maxScrollLeft - 1);
  }, []);

  useEffect(() => {
    const node = scrollerRef.current;
    if (!node) {
      return;
    }

    syncScrollState();
    node.addEventListener("scroll", syncScrollState, { passive: true });

    const observer = new ResizeObserver(syncScrollState);
    observer.observe(node);

    return () => {
      node.removeEventListener("scroll", syncScrollState);
      observer.disconnect();
    };
  }, [games.length, syncScrollState]);

  const scrollRail = useCallback((direction: -1 | 1) => {
    scrollerRef.current?.scrollBy({
      left: direction * SCROLL_STEP_PX,
      behavior: "smooth",
    });
  }, []);

  // 鼠标滚轮在横向滑轨上直接映射为横向滚动，符合桌面直觉
  const handleWheel = useCallback((event: React.WheelEvent<HTMLDivElement>) => {
    const node = scrollerRef.current;
    if (!node || node.scrollWidth <= node.clientWidth) {
      return;
    }
    const delta
      = Math.abs(event.deltaX) > Math.abs(event.deltaY)
        ? event.deltaX
        : event.deltaY;
    if (delta === 0) {
      return;
    }
    node.scrollLeft += delta;
  }, []);

  if (games.length === 0) {
    return (
      <section className="yh-glass flex flex-col gap-1 px-5 py-6">
        <h3 className="text-sm font-bold text-brand-900 dark:text-white">
          {t("home.quickLaunch")}
        </h3>
        <p className="text-xs text-brand-600 dark:text-white/70">
          {t("home.noRecentPlayedHint")}
        </p>
      </section>
    );
  }

  return (
    <section className="flex min-w-0 flex-col gap-2">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-sm font-bold text-brand-900 drop-shadow-sm dark:text-white">
          {t("home.quickLaunch")}
        </h3>
        <div className="flex items-center gap-1">
          <button
            type="button"
            aria-label={t("home.scrollRecentPrev")}
            onClick={() => scrollRail(-1)}
            disabled={!canScrollPrev}
            className="flex h-7 w-7 items-center justify-center rounded-full border border-white/50 bg-white/40 text-brand-700 transition-colors hover:bg-white/70 disabled:opacity-35 disabled:hover:bg-white/40 dark:border-white/12 dark:bg-white/8 dark:text-white/85 dark:hover:bg-white/16"
          >
            <span className="i-mdi-chevron-left text-lg" />
          </button>
          <button
            type="button"
            aria-label={t("home.scrollRecentNext")}
            onClick={() => scrollRail(1)}
            disabled={!canScrollNext}
            className="flex h-7 w-7 items-center justify-center rounded-full border border-white/50 bg-white/40 text-brand-700 transition-colors hover:bg-white/70 disabled:opacity-35 disabled:hover:bg-white/40 dark:border-white/12 dark:bg-white/8 dark:text-white/85 dark:hover:bg-white/16"
          >
            <span className="i-mdi-chevron-right text-lg" />
          </button>
          <button
            type="button"
            onClick={onViewAll}
            className="ml-1 rounded-full px-2 py-1 text-xs font-medium text-brand-700 transition-colors hover:bg-white/50 dark:text-white/80 dark:hover:bg-white/10"
          >
            {t("home.viewAll")}
          </button>
        </div>
      </div>

      <div
        ref={scrollerRef}
        onWheel={handleWheel}
        className="scrollbar-hide flex gap-3 overflow-x-auto pb-1"
      >
        {games.map(({ game, isPlaying, totalPlayedDur }) => {
          const isActive = game.id === activeGameId;
          const coverSrc = game.cover_url || game.cover_source_url || "";

          return (
            <button
              key={game.id}
              type="button"
              onClick={() => onSelectGame(game.id)}
              aria-current={isActive}
              className={`group flex w-[8.5rem] shrink-0 flex-col gap-2 rounded-xl border p-2 text-left transition-all duration-200 hover:-translate-y-1 ${
                isActive
                  ? "border-primary-400/80 bg-white/70 shadow-lg shadow-black/10 dark:border-primary-300/70 dark:bg-white/14"
                  : "border-white/40 bg-white/40 hover:border-white/70 hover:bg-white/60 dark:border-white/10 dark:bg-white/6 dark:hover:border-white/25 dark:hover:bg-white/12"
              }`}
            >
              <span className="relative block aspect-[3/4] w-full overflow-hidden rounded-lg bg-brand-900/20">
                {coverSrc ? (
                  <GameCoverImage
                    src={coverSrc}
                    fallbackSrc={game.cover_source_url}
                    alt={game.name}
                    isNSFW={game.is_nsfw}
                    className="h-full w-full"
                    imageClassName="h-full w-full object-cover"
                    loading="lazy"
                    decoding="async"
                  />
                ) : (
                  <span className="flex h-full w-full items-center justify-center text-brand-400 dark:text-white/40">
                    <span className="i-mdi-image-off text-2xl" />
                  </span>
                )}

                {isPlaying && (
                  <span className="absolute left-1.5 top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-success-500 text-white shadow">
                    <span className="i-mdi-gamepad-variant text-xs" />
                  </span>
                )}

                {/* 悬停浮层：直接启动 / 查看详情，与游戏库卡片保持一致 */}
                <span className="absolute inset-0 flex items-center justify-center gap-2 bg-black/55 opacity-0 transition-opacity duration-200 group-hover:opacity-100 group-focus-within:opacity-100">
                  <span
                    role="button"
                    tabIndex={0}
                    aria-label={t("gameCard.startGame")}
                    onClick={(event) => {
                      event.stopPropagation();
                      onLaunchGame(game);
                    }}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        event.stopPropagation();
                        onLaunchGame(game);
                      }
                    }}
                    className="flex h-8 w-8 items-center justify-center rounded-full bg-neutral-600 text-white transition-transform hover:scale-110 active:scale-95"
                  >
                    <span className="i-mdi-play text-lg" />
                  </span>
                  <span
                    role="button"
                    tabIndex={0}
                    aria-label={t("common.details")}
                    onClick={(event) => {
                      event.stopPropagation();
                      onOpenDetail(game.id);
                    }}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        event.stopPropagation();
                        onOpenDetail(game.id);
                      }
                    }}
                    className="flex h-8 w-8 items-center justify-center rounded-full bg-white/25 text-white transition-transform hover:scale-110 hover:bg-white/35 active:scale-95"
                  >
                    <span className="i-mdi-information-variant text-lg" />
                  </span>
                </span>
              </span>

              <span className="line-clamp-2 min-h-8 text-xs font-semibold leading-tight text-brand-900 dark:text-white">
                {game.name}
              </span>
              <span className="truncate text-[11px] text-brand-600 dark:text-white/65">
                {t("home.playTimeShort", {
                  duration: formatDurationCompact(totalPlayedDur, t),
                })}
              </span>
            </button>
          );
        })}
      </div>
    </section>
  );
}
