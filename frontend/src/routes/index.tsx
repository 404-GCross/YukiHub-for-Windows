import type { CSSProperties } from "react";
import type { models } from "../../src/bindings/models";
import type { HomeHeroSnapshot } from "../components/home/HomeHeroCard";
import type { HomeRailGame } from "../components/home/HomeQuickLaunchRail";
import { createRoute, useNavigate } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { GetGlobalPeriodStats } from "../../bindings/yukihub/internal/service/statsservice";
import { enums, vo } from "../../src/bindings/models";
import { SnowflakeMark } from "../components/branding/SnowflakeMark";
import { HomeHeatmapCard } from "../components/home/HomeHeatmapCard";
import { HomeHeroCard } from "../components/home/HomeHeroCard";
import { HomeQuickLaunchRail } from "../components/home/HomeQuickLaunchRail";
import { HomeTodayStatsCard } from "../components/home/HomeTodayStatsCard";
import { ProxyImage } from "../components/ui/ProxyImage";
import { useCrossfadeBackground } from "../hooks/useCrossfadeBackground";
import { useSnapshotVisibilityTransition } from "../hooks/useSnapshotVisibilityTransition";
import { isGameRuntimeVisible, useAppStore } from "../store";
import { clearFailedImageSources } from "../utils/imageProxy";
import { Route as rootRoute } from "./__root";

const DEFAULT_HOME_GAME_CAROUSEL_INTERVAL_SEC = 6;
const MIN_HOME_GAME_CAROUSEL_INTERVAL_SEC = 4;
/**
 * 手动点选某个游戏后暂停自动轮播的时长。
 *
 * 点一下轮播点就把自动播放永久关掉是不对的（原先 setIsCarouselPaused(true)
 * 没有任何恢复路径，点过一次之后轮播就再也不会自己走了），这里改成
 * 「暂停一会儿，让用户看清刚选的那张，然后继续」。
 */
const HOME_GAME_CAROUSEL_RESUME_DELAY_MS = 15000;
const BACKGROUND_CROSSFADE_MS = 1200;
const HERO_FADE_OUT_MS = 280;
const HERO_FADE_IN_DELAY_MS = 90;
const HOME_BACKGROUND_BLUR_PX = 16;
const HOME_BACKGROUND_IMAGE_STYLE = {
  filter: `blur(${HOME_BACKGROUND_BLUR_PX}px)`,
  WebkitFilter: `blur(${HOME_BACKGROUND_BLUR_PX}px)`,
} satisfies CSSProperties;

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: HomePage,
});

function HomePage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const homeData = useAppStore(state => state.homeData);
  const fetchHomeData = useAppStore(state => state.fetchHomeData);
  const isLoading = useAppStore(state => state.isLoading);
  const config = useAppStore(state => state.config);
  const startGame = useAppStore(state => state.startGame);
  const hasVisibleGameRuntime = useAppStore(state =>
    Object.values(state.gameRuntimes).some(isGameRuntimeVisible),
  );
  const [activeGameId, setActiveGameId] = useState<string | null>(null);
  const [isCarouselPaused, setIsCarouselPaused] = useState(false);
  const [isCarouselHovered, setIsCarouselHovered] = useState(false);
  const carouselResumeTimerRef = useRef<number | null>(null);
  const [libraryPreviewStats, setLibraryPreviewStats]
    = useState<vo.PeriodStats | null>(null);
  const [heatmapStats, setHeatmapStats] = useState<vo.PeriodStats | null>(null);
  const [isHeatmapLoading, setIsHeatmapLoading] = useState(false);
  const [heatmapLoadFailed, setHeatmapLoadFailed] = useState(false);

  const loadLibraryPreviewStats = useCallback(async () => {
    try {
      const data = await GetGlobalPeriodStats(
        new vo.PeriodStatsRequest({
          dimension: enums.Period.All,
          start_date: "",
          end_date: "",
        }),
      );
      setLibraryPreviewStats(data);
    }
    catch (error) {
      console.error("Failed to fetch library preview stats:", error);
    }
  }, []);

  const loadHeatmapStats = useCallback(async () => {
    setIsHeatmapLoading(true);
    setHeatmapLoadFailed(false);
    try {
      const data = await GetGlobalPeriodStats(
        new vo.PeriodStatsRequest({
          dimension: enums.Period.Year,
          start_date: "",
          end_date: "",
        }),
      );
      setHeatmapStats(data);
    }
    catch (error) {
      console.error("Failed to fetch home heatmap stats:", error);
      setHeatmapLoadFailed(true);
    }
    finally {
      setIsHeatmapLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchHomeData();
    void loadLibraryPreviewStats();
    void loadHeatmapStats();
  }, [fetchHomeData, loadLibraryPreviewStats, loadHeatmapStats]);

  const carouselItems = useMemo(() => {
    const items = [...(homeData?.recent_played || [])];
    const lastPlayed = homeData?.last_played;

    if (
      lastPlayed?.game.id
      && !items.some(item => item.game.id === lastPlayed.game.id)
    ) {
      items.unshift(lastPlayed);
    }

    return items;
  }, [homeData?.last_played, homeData?.recent_played]);

  const carouselGames = useMemo(
    () => carouselItems.map(item => item.game),
    [carouselItems],
  );
  const railGames = useMemo<HomeRailGame[]>(
    () =>
      carouselItems.map(item => ({
        game: item.game,
        isPlaying: Boolean(item.is_playing),
        totalPlayedDur: Number(item.total_played_dur || 0),
      })),
    [carouselItems],
  );
  const isHomeGameCarouselEnabled
    = config?.home_game_carousel_enabled !== false;
  const homeGameCarouselIntervalMs
    = Math.max(
      MIN_HOME_GAME_CAROUSEL_INTERVAL_SEC,
      Number(
        config?.home_game_carousel_interval_sec
        || DEFAULT_HOME_GAME_CAROUSEL_INTERVAL_SEC,
      ),
    ) * 1000;

  useEffect(() => {
    setActiveGameId(current =>
      current && carouselItems.some(item => item.game.id === current)
        ? current
        : (carouselItems[0]?.game.id ?? null),
    );
  }, [carouselItems]);

  // 悬停暂停：鼠标停在轮播上就别切走；离开立刻恢复。
  // 手动点选暂停：延时恢复，避免「点过一次就永远不动了」。
  const isCarouselAutoPlayBlocked = isCarouselPaused || isCarouselHovered;

  useEffect(() => {
    if (
      carouselGames.length <= 1
      || !isHomeGameCarouselEnabled
      || isCarouselAutoPlayBlocked
      || hasVisibleGameRuntime
    ) {
      return;
    }

    const timer = window.setInterval(() => {
      setActiveGameId((current) => {
        const currentIndex = carouselGames.findIndex(
          game => game.id === current,
        );
        const nextIndex
          = currentIndex >= 0 ? (currentIndex + 1) % carouselGames.length : 0;
        return carouselGames[nextIndex]?.id ?? current;
      });
    }, homeGameCarouselIntervalMs);

    return () => window.clearInterval(timer);
  }, [
    carouselGames,
    hasVisibleGameRuntime,
    homeGameCarouselIntervalMs,
    isCarouselAutoPlayBlocked,
    isHomeGameCarouselEnabled,
  ]);

  useEffect(
    () => () => {
      if (carouselResumeTimerRef.current !== null) {
        window.clearTimeout(carouselResumeTimerRef.current);
      }
    },
    [],
  );

  const selectedCarouselItem = useMemo(() => {
    if (carouselItems.length === 0) {
      return null;
    }

    return (
      carouselItems.find(item => item.game.id === activeGameId)
      ?? carouselItems[0]
    );
  }, [activeGameId, carouselItems]);

  const selectedGame = selectedCarouselItem?.game ?? null;

  const selectedGameHasRuntime = useAppStore((state) => {
    if (!selectedGame?.id) {
      return false;
    }

    return Boolean(state.gameRuntimes[selectedGame.id]);
  });

  const selectedGameCoverSrc
    = selectedGame?.cover_url || selectedGame?.cover_source_url || "";
  const { isBackgroundCrossfading, previousBackgroundUrl }
    = useCrossfadeBackground(selectedGameCoverSrc, {
      durationMs: BACKGROUND_CROSSFADE_MS,
    });

  const isSelectedGamePlaying = Boolean(
    selectedGame?.id
    && (selectedCarouselItem?.is_playing || selectedGameHasRuntime),
  );
  const currentHeroSnapshot = useMemo<HomeHeroSnapshot | null>(() => {
    if (!selectedGame || !selectedCarouselItem) {
      return null;
    }

    return {
      game: selectedGame,
      isPlaying: isSelectedGamePlaying,
      lastPlayedAt: selectedCarouselItem.last_played_at,
      totalPlayedDur: Number(selectedCarouselItem.total_played_dur || 0),
    };
  }, [isSelectedGamePlaying, selectedCarouselItem, selectedGame]);
  const currentHeroId = currentHeroSnapshot?.game.id ?? null;
  const { displayedSnapshot: displayedHeroSnapshot, isVisible: isHeroVisible }
    = useSnapshotVisibilityTransition<HomeHeroSnapshot>(
      currentHeroSnapshot,
      currentHeroId,
      {
        fadeInDelayMs: HERO_FADE_IN_DELAY_MS,
        fadeOutMs: HERO_FADE_OUT_MS,
      },
    );

  const showGameBackground
    = !config?.background_enabled || !config?.background_hide_game_cover;
  const showHeroCover
    = !config?.background_enabled || !config?.background_hide_game_hero_cover;

  const openGameDetail = useCallback(
    (gameId?: string) => {
      if (!gameId) {
        return;
      }
      navigate({ to: "/game/$gameId", params: { gameId } });
    },
    [navigate],
  );

  const launchGame = useCallback(
    async (game: models.Game) => {
      if (!game.id) {
        return;
      }

      try {
        const success = await startGame(game);
        if (success) {
          setActiveGameId(game.id);
        }
        else {
          toast.error(
            t("gameCard.startFailedNotLaunched", { name: game.name }),
          );
        }
      }
      catch (err) {
        console.error("Failed to launch game:", err);
        toast.error(t("home.toast.launchFailed"));
      }
    },
    [startGame, t],
  );

  const handleContinuePlay = useCallback(async () => {
    if (!selectedGame) {
      return;
    }
    await launchGame(selectedGame);
  }, [launchGame, selectedGame]);

  /**
   * 手动切换轮播：暂停自动播放一小段时间再恢复。
   *
   * 之前这里只 setIsCarouselPaused(true) 且没有任何地方置回 false ——
   * 只要点过一次轮播点或快速启动栏，自动轮播就永久停住，用户看到的就是
   * 「首页的轮播图不能自己滑动」。
   */
  const pauseCarouselBriefly = useCallback(() => {
    setIsCarouselPaused(true);
    if (carouselResumeTimerRef.current !== null) {
      window.clearTimeout(carouselResumeTimerRef.current);
    }
    carouselResumeTimerRef.current = window.setTimeout(() => {
      carouselResumeTimerRef.current = null;
      setIsCarouselPaused(false);
    }, HOME_GAME_CAROUSEL_RESUME_DELAY_MS);
  }, []);

  const handleSelectGame = useCallback(
    (gameId: string) => {
      pauseCarouselBriefly();
      setActiveGameId(gameId);
    },
    [pauseCarouselBriefly],
  );

  const handleCarouselHoverChange = useCallback((hovered: boolean) => {
    setIsCarouselHovered(hovered);
  }, []);

  const handleRefresh = useCallback(() => {
    // 手动刷新时清掉封面失败记忆，否则刚修好的地址会被旧记忆挡住。
    clearFailedImageSources();
    void fetchHomeData({ showLoading: false, syncRuntime: false });
    void loadLibraryPreviewStats();
    void loadHeatmapStats();
  }, [fetchHomeData, loadHeatmapStats, loadLibraryPreviewStats]);

  if (isLoading) {
    return null;
  }

  if (!homeData) {
    return (
      <div className="relative flex h-full flex-col items-center justify-center gap-4">
        <p className="text-brand-500 dark:text-brand-400">{t("home.noData")}</p>
        <button
          type="button"
          onClick={() => void fetchHomeData()}
          className="yh-primary-pill rounded-full px-5 py-2 text-sm font-bold text-white"
        >
          {t("home.retry")}
        </button>
      </div>
    );
  }

  const lastPlayed = homeData.last_played;
  if (!lastPlayed || !selectedGame) {
    return (
      <div className="relative min-h-full">
        <div className="yh-hero-gradient-light dark:yh-hero-gradient pointer-events-none absolute inset-0" />
        <div className="relative flex min-h-full flex-col items-center justify-center gap-3 p-6 text-center">
          <SnowflakeMark className="h-16 w-16 text-brand-400/70 dark:text-white/25" />
          <h1 className="text-2xl font-bold text-brand-900 dark:text-white">
            {t("home.title")}
          </h1>
          <p className="max-w-md text-sm text-brand-700 dark:text-white/80">
            {t("home.noPlayRecordHint")}
          </p>
          <button
            type="button"
            onClick={() => navigate({ to: "/library" })}
            className="yh-primary-pill mt-2 inline-flex items-center gap-2 rounded-full px-6 py-3 text-sm font-bold text-white shadow-lg shadow-black/20 transition-all hover:brightness-110 active:scale-95"
          >
            <span className="i-mdi-gamepad-variant text-lg" />
            {t("home.browseLibrary")}
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="relative min-h-full">
      {/* 背景层：模糊封面（受「隐藏首页游戏封面」控制） + 手机版签名渐变 */}
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        {showGameBackground && selectedGameCoverSrc && (
          <div
            className="absolute"
            style={{ inset: -3 * HOME_BACKGROUND_BLUR_PX }}
          >
            <ProxyImage
              src={selectedGameCoverSrc}
              fallbackSrc={selectedGame.cover_source_url}
              alt=""
              isNSFW={selectedGame.is_nsfw}
              className="absolute inset-0 h-full w-full object-cover"
              style={HOME_BACKGROUND_IMAGE_STYLE}
            />
            {previousBackgroundUrl
              && previousBackgroundUrl !== selectedGameCoverSrc && (
              <ProxyImage
                src={previousBackgroundUrl}
                alt=""
                isNSFW={selectedGame.is_nsfw}
                className={`absolute inset-0 h-full w-full object-cover transition-opacity duration-[1200ms] ease-in-out ${
                  isBackgroundCrossfading ? "opacity-100" : "opacity-0"
                }`}
                style={HOME_BACKGROUND_IMAGE_STYLE}
              />
            )}
          </div>
        )}
        <div className="yh-hero-gradient-light dark:yh-hero-gradient absolute inset-0 opacity-95" />
        <div className="absolute inset-0 bg-gradient-to-t from-black/25 via-transparent to-transparent" />
      </div>

      <div className="relative flex min-h-full flex-col gap-4 p-5">
        <header className="flex items-center gap-3">
          <SnowflakeMark className="h-6 w-6 shrink-0 text-primary-600 dark:text-primary-300" />
          <div className="min-w-0 flex-1">
            <h1 className="truncate text-base font-bold leading-tight text-brand-900 dark:text-white">
              YukiHub
            </h1>
            <p className="truncate text-xs text-brand-600 dark:text-white/80">
              {t("home.welcomeBack")}
            </p>
          </div>
          <button
            type="button"
            onClick={handleRefresh}
            aria-label={t("home.refresh")}
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-white/50 bg-white/40 text-brand-700 transition-colors hover:bg-white/70 dark:border-white/12 dark:bg-white/8 dark:text-white/85 dark:hover:bg-white/16"
          >
            <span className="i-mdi-refresh text-lg" />
          </button>
        </header>

        <div className="flex min-h-0 flex-1 flex-col gap-4 lg:flex-row">
          <section className="flex min-w-0 flex-[1.6] flex-col gap-4">
            <HomeHeroCard
              activeGameId={selectedGame.id}
              games={carouselGames}
              isVisible={isHeroVisible}
              onContinuePlay={() => void handleContinuePlay()}
              onHoverChange={handleCarouselHoverChange}
              onOpenDetail={openGameDetail}
              onSelectGame={handleSelectGame}
              showCover={showHeroCover}
              snapshot={displayedHeroSnapshot}
              timeZone={config?.time_zone}
            />
            <HomeQuickLaunchRail
              activeGameId={selectedGame.id}
              games={railGames}
              onLaunchGame={game => void launchGame(game)}
              onOpenDetail={openGameDetail}
              onSelectGame={handleSelectGame}
              onViewAll={() => navigate({ to: "/library" })}
            />
          </section>

          <aside className="flex min-w-0 flex-[1.1] flex-col gap-4">
            <HomeTodayStatsCard
              completedGames={
                libraryPreviewStats
                  ? Number(libraryPreviewStats.all_completed_games_count || 0)
                  : null
              }
              libraryGames={
                libraryPreviewStats
                  ? Number(libraryPreviewStats.library_games_count || 0)
                  : null
              }
              todayPlayTimeSec={Number(homeData.today_play_time_sec || 0)}
              weeklyPlayTimeSec={Number(homeData.weekly_play_time_sec || 0)}
            />
            <HomeHeatmapCard
              cells={heatmapStats?.heatmap ?? []}
              hasFailed={heatmapLoadFailed}
              isLoading={isHeatmapLoading || !heatmapStats}
              onRetry={() => void loadHeatmapStats()}
              onViewStats={() => navigate({ to: "/stats" })}
            />
          </aside>
        </div>
      </div>
    </div>
  );
}
