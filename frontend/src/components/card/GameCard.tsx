import type { TFunction } from "i18next";
import type { models } from "../../../src/bindings/models";
import { useNavigate } from "@tanstack/react-router";
import { memo, useCallback } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums } from "../../../src/bindings/models";
import { GAME_STATUS_BADGE_STYLES } from "../../consts/gameStatusBadge";
import { useAppStore } from "../../store";
import { formatLocalDate } from "../../utils/time";
import { GameCoverImage } from "../ui/GameCoverImage";

export type GameCardLayout = "portrait" | "landscape";

function HighlightText({ text, query }: { text: string; query: string }) {
  if (!query || !text) {
    return <>{text}</>;
  }
  const q = query.toLowerCase();
  const idx = text.toLowerCase().indexOf(q);
  if (idx === -1) {
    return <>{text}</>;
  }
  return (
    <>
      {text.slice(0, idx)}
      <mark className="bg-yellow-300/80 dark:bg-yellow-500/50 text-inherit rounded-[2px] px-[1px]">
        {text.slice(idx, idx + query.length)}
      </mark>
      {text.slice(idx + query.length)}
    </>
  );
}

// ─────────────────────────────────────────────────────────────────────────────

function formatSortFieldValue(
  game: models.Game,
  sortBy: enums.GameListSortBy | null | undefined,
  t: TFunction,
): string | null {
  if (
    !sortBy
    || sortBy === enums.GameListSortBy.GameListSortByName
    || sortBy === enums.GameListSortBy.GameListSortByCompany
  ) {
    return null;
  }
  switch (sortBy) {
    case enums.GameListSortBy.GameListSortByLastPlayedAt:
      return game.last_played_at
        ? formatLocalDate(game.last_played_at)
        : t("common.never");
    case enums.GameListSortBy.GameListSortByCreatedAt:
      return formatLocalDate(game.created_at);
    case enums.GameListSortBy.GameListSortByRating:
      return `${(game.rating ?? 0).toFixed(1)}/10.0`;
    case enums.GameListSortBy.GameListSortByReleaseDate:
      return game.release_date || t("common.unknownDate");
    default:
      return null;
  }
}

interface GameCardProps {
  game: models.Game;
  selectionMode?: boolean;
  selected?: boolean;
  onSelectChange?: (selected: boolean) => void;
  /** 当前搜索词，用于高亮游戏名和开发商 */
  searchQuery?: string;
  /** 当前排序维度；名称和厂商已在卡片底部展示，其余字段可在封面底部展示 */
  displaySortField?: enums.GameListSortBy | null;
  cardLayout?: GameCardLayout;
  /** 非多选模式下点击卡片本体时触发（PC 版用于展开右侧详情面板） */
  onActivate?: (game: models.Game) => void;
  /** 覆盖「查看详情」的默认行为（大屏模式下改为就地打开详情层） */
  onViewDetails?: (game: models.Game) => void;
}

function GameCardComponent({
  game,
  selectionMode = false,
  selected = false,
  onSelectChange,
  searchQuery = "",
  displaySortField = null,
  cardLayout = "portrait",
  onActivate,
  onViewDetails,
}: GameCardProps) {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const startGame = useAppStore(state => state.startGame);
  const gameRuntime = useAppStore(state =>
    game.id ? state.gameRuntimes[game.id] : undefined,
  );
  const isCurrentGameRunning = Boolean(gameRuntime);
  const isCurrentGameEnding = gameRuntime?.state === "ending";

  const handleToggleSelect = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      onSelectChange?.(!selected);
    },
    [onSelectChange, selected],
  );

  const handleStartGame = useCallback(
    async (e: React.MouseEvent) => {
      e.stopPropagation();
      if (isCurrentGameRunning) {
        return;
      }
      if (game.id) {
        try {
          const started = await startGame(game);
          // if (started) {
          //   toast.success(t("gameCard.startSuccess", { name: game.name }));
          // }
          // else {
          //   toast.error(
          //     t("gameCard.startFailedNotLaunched", { name: game.name }),
          //   );
          // }
          if (!started) {
            toast.error(
              t("gameCard.startFailedNotLaunched", { name: game.name }),
            );
          }
        }
        catch (error) {
          console.error("Failed to start game:", error);
          toast.error(t("gameCard.startFailedLog", { name: game.name }));
        }
      }
    },
    [game, isCurrentGameRunning, startGame, t],
  );

  const handleViewDetails = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      if (onViewDetails) {
        onViewDetails(game);
        return;
      }
      navigate({ to: `/game/${game.id}` });
    },
    [game, navigate, onViewDetails],
  );

  const handleCardClick = useCallback(() => {
    if (selectionMode) {
      onSelectChange?.(!selected);
      return;
    }
    onActivate?.(game);
  }, [game, onActivate, onSelectChange, selected, selectionMode]);

  const statusBadge = GAME_STATUS_BADGE_STYLES[game.status];
  const companyDisplay = game.company || t("common.unknownDeveloper");
  const sortFieldText = formatSortFieldValue(game, displaySortField, t);
  const isLandscape = cardLayout === "landscape";

  return (
    <div
      data-drag-selection-id={selectionMode ? game.id : undefined}
      className={`group relative flex w-full flex-col overflow-hidden rounded-xl border border-primary-200/70 bg-white shadow-sm transition-all duration-200 hover:-translate-y-1 hover:border-primary-300 hover:shadow-lg dark:border-primary-300/30 dark:bg-gradient-to-b dark:from-[#1B2A47] dark:via-[#152039] dark:to-[#0E1729] dark:hover:border-primary-300/55 dark:hover:shadow-black/40 data-glass:border-white/22 data-glass:bg-none data-glass:bg-transparent data-glass:hover:border-white/35 data-glass:dark:border-white/12 data-glass:dark:bg-transparent data-glass:dark:hover:border-white/22 native-webkit:paint-containment ${selectionMode ? "cursor-pointer [touch-action:none]" : ""} ${selectionMode && selected ? "ring-2 ring-inset ring-primary-400 dark:ring-primary-300" : ""}`}
      onClick={handleCardClick}
    >
      {selectionMode && (
        <button
          type="button"
          onClick={handleToggleSelect}
          className={`absolute left-2 top-2 z-10 flex h-6 w-6 items-center justify-center rounded-full border
                      ${
        selected
          ? "border-transparent bg-gradient-to-b from-primary-500 to-primary-700 text-white"
          : "bg-white/90 text-transparent border-brand-300 dark:bg-brand-800/90 dark:border-brand-600"
        }
                      shadow-sm`}
        >
          <div className="i-mdi-check text-sm" />
        </button>
      )}
      <div
        className={`relative w-full overflow-hidden bg-brand-200 dark:bg-brand-900/60 ${
          isLandscape ? "aspect-video" : "aspect-[3/3.6]"
        }`}
      >
        {game.cover_url || game.cover_source_url ? (
          <GameCoverImage
            src={game.cover_url || game.cover_source_url}
            fallbackSrc={game.cover_source_url}
            alt={game.name}
            isNSFW={game.is_nsfw}
            className="h-full w-full"
            imageClassName="h-full w-full object-cover object-center"
            decoding="async"
            loading="lazy"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-brand-400">
            <div className="i-mdi-image-off text-4xl" />
          </div>
        )}

        {/* 手机版状态徽标（item_game_card.xml 的 tvStatusBadge） */}
        {statusBadge && (
          <span
            className={`absolute right-1.5 top-1.5 inline-flex items-center gap-1 rounded-full border border-white/25 px-2 py-0.5 text-[10px] font-bold leading-none text-white shadow-sm backdrop-blur-sm ${statusBadge.className}`}
          >
            <span
              className={`${statusBadge.icon} text-xs`}
              aria-hidden="true"
            />
            {t(statusBadge.labelKey)}
          </span>
        )}

        {/* 手机版封面芯片：评分 */}
        {!selectionMode && game.rating > 0 && (
          <span className="absolute left-1.5 top-1.5 inline-flex items-center gap-0.5 rounded-full border border-white/20 bg-black/45 px-1.5 py-0.5 text-[10px] font-bold leading-none text-white backdrop-blur-sm">
            <span
              className="i-mdi-star text-[10px] text-yellow-300"
              aria-hidden="true"
            />
            {game.rating.toFixed(1)}
          </span>
        )}

        {/* 当前排序字段值（封面底部覆盖条） */}
        {sortFieldText && (
          <div className="pointer-events-none absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/75 via-black/45 to-transparent px-2 pt-5 pb-1.5">
            <p className="truncate text-xs font-semibold text-white drop-shadow-sm">
              {sortFieldText}
            </p>
          </div>
        )}

        {!selectionMode && (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-black/50 opacity-0 transition-opacity duration-200 group-hover:opacity-100">
            <button
              type="button"
              onClick={handleStartGame}
              disabled={isCurrentGameRunning}
              aria-label={t("gameCard.startGame")}
              className="yh-primary-pill flex h-8 w-8 items-center justify-center rounded-full text-white shadow-lg transition-transform hover:scale-110 active:scale-95 disabled:cursor-not-allowed disabled:opacity-65 disabled:hover:scale-100"
            >
              <div
                className={
                  isCurrentGameEnding
                    ? "i-mdi-loading animate-spin text-lg"
                    : isCurrentGameRunning
                      ? "i-mdi-gamepad-variant text-lg"
                      : "i-mdi-play text-lg"
                }
              />
            </button>
            <button
              type="button"
              onClick={handleViewDetails}
              className="flex h-8 w-8 items-center justify-center rounded-full bg-white/20 text-white transition-transform hover:scale-110 hover:bg-white/30 active:scale-95"
            >
              <div className="i-mdi-information-variant text-lg" />
            </button>
          </div>
        )}
      </div>

      <div
        className={`data-glass:bg-white/8 data-glass:backdrop-blur-12 data-glass:backdrop-saturate-180 data-glass:dark:bg-black/12 native-webkit:backdrop-filter-off ${isLandscape ? "px-3 pt-2 pb-3" : "px-2 pt-1 pb-2"}`}
      >
        <h3 className="truncate text-sm font-bold text-brand-900 dark:text-white leading-tight">
          <HighlightText text={game.name} query={searchQuery} />
        </h3>
        <p className="truncate text-xs text-brand-500 dark:text-brand-400 leading-tight">
          <HighlightText text={companyDisplay} query={searchQuery} />
        </p>
      </div>
    </div>
  );
}

export const GameCard = memo(GameCardComponent);
