import type { models } from "../../../src/bindings/models";
import { useTranslation } from "react-i18next";
import { formatDuration, formatLocalDateTime } from "../../utils/time";
import { GameCoverImage } from "../ui/GameCoverImage";

/** 首页英雄卡展示所需的快照（比 models.Game 多带本次/累计时长） */
export interface HomeHeroSnapshot {
  game: models.Game;
  isPlaying: boolean;
  lastPlayedAt: unknown;
  totalPlayedDur: number;
}

interface HomeHeroCardProps {
  activeGameId: string | null;
  games: models.Game[];
  isVisible: boolean;
  onContinuePlay: () => void;
  /** 鼠标进出轮播：进入暂停自动播放，离开恢复（由父级控制轮播计时器） */
  onHoverChange?: (hovered: boolean) => void;
  onOpenDetail: (gameId: string) => void;
  onSelectGame: (gameId: string) => void;
  showCover: boolean;
  snapshot: HomeHeroSnapshot | null;
  timeZone?: string;
}

/** 轮播点超过这个数量就不再渲染，改由下方快速启动滑轨承担选择 */
const MAX_CAROUSEL_DOTS = 12;

export function HomeHeroCard({
  activeGameId,
  games,
  isVisible,
  onContinuePlay,
  onHoverChange,
  onOpenDetail,
  onSelectGame,
  showCover,
  snapshot,
  timeZone,
}: HomeHeroCardProps) {
  const { t } = useTranslation();
  const game = snapshot?.game ?? null;
  const coverSrc = game?.cover_url || game?.cover_source_url || "";
  const hasCover = showCover && Boolean(coverSrc);
  const showDots = games.length > 1 && games.length <= MAX_CAROUSEL_DOTS;
  const contentMotionClass = isVisible
    ? "translate-y-0 opacity-100"
    : "translate-y-3 opacity-0";

  return (
    <div
      className="flex min-w-0 flex-1 flex-col gap-3"
      onMouseEnter={() => onHoverChange?.(true)}
      onMouseLeave={() => onHoverChange?.(false)}
    >
      <div className="relative flex min-h-[14rem] flex-1 flex-col overflow-hidden rounded-2xl border border-white/45 bg-white/30 shadow-lg shadow-black/10 backdrop-blur-xl dark:border-white/12 dark:bg-white/8 dark:shadow-black/30">
        {game ? (
          <div
            className={`absolute inset-0 transition-opacity duration-500 ease-out ${isVisible ? "opacity-100" : "opacity-0"}`}
          >
            {hasCover ? (
              <GameCoverImage
                src={coverSrc}
                fallbackSrc={game.cover_source_url}
                alt={game.name}
                isNSFW={game.is_nsfw}
                revealNSFWOnHover
                className="absolute inset-0"
                imageClassName="h-full w-full object-cover"
              />
            ) : (
              <div className="yh-hero-gradient absolute inset-0" />
            )}
          </div>
        ) : (
          <div className="yh-hero-gradient absolute inset-0" />
        )}

        {/* 手机版 bg_home_hero_overlay：底部深紫，向上渐隐，保证白字可读 */}
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-[#2B2158]/88 via-black/25 to-black/5" />

        {game && (
          <div
            className={`relative mt-auto flex items-end gap-4 p-5 transition-all duration-500 ease-out ${contentMotionClass}`}
          >
            <div className="min-w-0 flex-1">
              <button
                type="button"
                onClick={() => onOpenDetail(game.id)}
                className="block max-w-full truncate text-left text-2xl font-bold text-white drop-shadow-md transition-colors hover:text-white/85 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60"
              >
                {game.name}
              </button>

              {snapshot?.isPlaying ? (
                <p className="mt-1.5 text-sm font-medium text-white/90 drop-shadow">
                  {t("home.playingNow")}
                </p>
              ) : (
                <div className="mt-1.5 space-y-0.5">
                  <p className="truncate text-sm text-white/85 drop-shadow">
                    {t("home.lastPlayed", {
                      time: formatLocalDateTime(
                        snapshot?.lastPlayedAt,
                        timeZone,
                      ),
                    })}
                  </p>
                  {snapshot && snapshot.totalPlayedDur > 0 && (
                    <p className="truncate text-xs text-white/70 drop-shadow">
                      {t("home.totalPlayTime")}
                      {formatDuration(snapshot.totalPlayedDur, t)}
                    </p>
                  )}
                </div>
              )}
            </div>

            {snapshot?.isPlaying ? (
              <span className="inline-flex h-11 shrink-0 items-center gap-2 rounded-full border border-white/40 bg-success-500/90 px-6 text-sm font-bold text-white shadow-lg shadow-black/25">
                <span className="i-mdi-gamepad-variant animate-pulse text-lg" />
                {t("home.gaming")}
              </span>
            ) : (
              <button
                type="button"
                onClick={onContinuePlay}
                className="yh-primary-pill inline-flex h-11 shrink-0 items-center gap-2 rounded-full px-6 text-sm font-bold text-white shadow-lg shadow-black/30 transition-all hover:brightness-110 active:scale-95"
              >
                <span className="i-mdi-play text-lg" />
                {t("home.continueGame")}
              </button>
            )}
          </div>
        )}
      </div>

      {showDots && (
        <div className="flex items-center justify-center gap-1.5">
          {games.map(g => (
            <button
              key={g.id}
              type="button"
              aria-label={t("home.selectGame", { name: g.name })}
              aria-current={g.id === activeGameId}
              onClick={() => onSelectGame(g.id)}
              className={`h-1.5 rounded-full transition-all duration-300 ${
                g.id === activeGameId
                  ? "w-6 bg-white dark:bg-white"
                  : "w-1.5 bg-brand-900/25 hover:bg-brand-900/45 dark:bg-white/35 dark:hover:bg-white/60"
              }`}
            />
          ))}
        </div>
      )}
    </div>
  );
}
