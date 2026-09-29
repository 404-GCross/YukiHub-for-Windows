import type { models } from "../../../src/bindings/models";
import { useTranslation } from "react-i18next";
import { GAME_STATUS_BADGE_STYLES } from "../../consts/gameStatusBadge";
import { useGamePlaytime } from "../../hooks/useGamePlaytime";
import { formatDuration, formatLocalDate } from "../../utils/time";
import { GameCoverImage } from "../ui/GameCoverImage";

interface LibraryDetailPanelProps {
  game: models.Game;
  isRunning: boolean;
  onClose: () => void;
  onOpenDetail: (gameId: string) => void;
  onStart: (game: models.Game) => void;
}

/** PC 版游戏库右侧详情面板：对应手机版点击卡片弹出的 dialog_game_detail。 */
export function LibraryDetailPanel({
  game,
  isRunning,
  onClose,
  onOpenDetail,
  onStart,
}: LibraryDetailPanelProps) {
  const { t } = useTranslation();
  // 列表请求已带批量时长并预填缓存；缓存未命中（例如命中本地列表缓存）时才单查
  const playTime = useGamePlaytime(game.id);
  const coverSrc = game.cover_url || game.cover_source_url || "";
  const statusBadge = GAME_STATUS_BADGE_STYLES[game.status];

  const infoRows = [
    {
      icon: "i-mdi-timer-outline",
      label: t("common.playTime"),
      value: playTime > 0 ? formatDuration(playTime, t) : t("common.never"),
    },
    {
      icon: "i-mdi-domain",
      label: t("common.company"),
      value: game.company || t("common.unknownDeveloper"),
    },
    {
      icon: "i-mdi-star-outline",
      label: t("common.rating"),
      value: game.rating > 0 ? `${game.rating.toFixed(1)}/10.0` : "--",
    },
    {
      icon: "i-mdi-calendar-blank-outline",
      label: t("common.releaseDate"),
      value: game.release_date || t("common.unknownDate"),
    },
    {
      icon: "i-mdi-history",
      label: t("common.lastPlayedAt"),
      value: game.last_played_at
        ? formatLocalDate(game.last_played_at)
        : t("common.never"),
    },
  ];

  return (
    <section className="yh-glass flex flex-col gap-4 p-4">
      <header className="flex items-center gap-2">
        <h2 className="min-w-0 flex-1 truncate text-sm font-bold text-brand-900 dark:text-white">
          {t("library.detailTitle")}
        </h2>
        <button
          type="button"
          onClick={onClose}
          aria-label={t("common.close")}
          className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-brand-600 transition-colors hover:bg-white/60 dark:text-white/70 dark:hover:bg-white/12"
        >
          <span className="i-mdi-close text-lg" />
        </button>
      </header>

      <div className="relative aspect-[3/4] w-full overflow-hidden rounded-xl border border-primary-200/70 bg-brand-200 dark:border-primary-300/30 dark:bg-brand-900/60">
        {coverSrc ? (
          <GameCoverImage
            src={coverSrc}
            fallbackSrc={game.cover_source_url}
            alt={game.name}
            isNSFW={game.is_nsfw}
            revealNSFWOnHover
            className="h-full w-full"
            imageClassName="h-full w-full object-cover"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-brand-400 dark:text-white/40">
            <span className="i-mdi-image-off text-3xl" />
          </div>
        )}

        {statusBadge && (
          <span
            className={`absolute right-2 top-2 inline-flex items-center gap-1 rounded-full border border-white/25 px-2 py-0.5 text-[10px] font-bold leading-none text-white shadow-sm backdrop-blur-sm ${statusBadge.className}`}
          >
            <span
              className={`${statusBadge.icon} text-xs`}
              aria-hidden="true"
            />
            {t(statusBadge.labelKey)}
          </span>
        )}
      </div>

      <div className="min-w-0">
        <h3 className="truncate text-lg font-bold text-brand-900 dark:text-white">
          {game.name}
        </h3>
        {game.summary && (
          <p className="mt-1 line-clamp-3 text-xs leading-relaxed text-brand-600 dark:text-white/70">
            {game.summary}
          </p>
        )}
      </div>

      <dl className="flex flex-col gap-2">
        {infoRows.map(row => (
          <div key={row.label} className="flex items-start gap-2">
            <span
              className={`${row.icon} mt-0.5 shrink-0 text-base text-primary-600 dark:text-primary-300`}
              aria-hidden="true"
            />
            <div className="min-w-0 flex-1">
              <dt className="text-[11px] text-brand-500 dark:text-white/60">
                {row.label}
              </dt>
              <dd className="truncate text-xs font-medium text-brand-800 dark:text-white/90">
                {row.value}
              </dd>
            </div>
          </div>
        ))}
      </dl>

      <div className="mt-auto flex flex-col gap-2 pt-2">
        <button
          type="button"
          onClick={() => onStart(game)}
          disabled={isRunning}
          className="yh-primary-pill inline-flex h-10 items-center justify-center gap-2 rounded-full text-sm font-bold text-white shadow-lg transition-all hover:brightness-110 active:scale-95 disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:brightness-100"
        >
          <span
            className={
              isRunning ? "i-mdi-gamepad-variant text-lg" : "i-mdi-play text-lg"
            }
          />
          {isRunning ? t("common.playing") : t("gameCard.startGame")}
        </button>
        <button
          type="button"
          onClick={() => onOpenDetail(game.id)}
          className="inline-flex h-10 items-center justify-center gap-2 rounded-full border border-primary-200/70 bg-white/70 text-sm font-semibold text-brand-700 transition-colors hover:bg-white dark:border-primary-300/40 dark:bg-[#1D2B3E]/70 dark:text-white/90 dark:hover:bg-[#1D2B3E]"
        >
          <span className="i-mdi-open-in-new text-base" />
          {t("library.viewFullDetail")}
        </button>
      </div>
    </section>
  );
}
