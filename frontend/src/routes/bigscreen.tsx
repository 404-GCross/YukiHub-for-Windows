import type { models } from "../../src/bindings/models";
import type { BigScreenDetailAction } from "../bigscreen/BigScreenDetailsLayer";
import type { BigScreenAction } from "../bigscreen/BigScreenInfoBar";
import type { BigScreenSound } from "../bigscreen/bigScreenSound";
import type { BigScreenCategoryId } from "../bigscreen/categories";
import type { FocusDirection, FocusZone } from "../bigscreen/focusEngine";
import type {
  BigScreenInputDevice,
  BigScreenIntent,
} from "../bigscreen/useGamepad";
import { createRoute, useNavigate } from "@tanstack/react-router";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  AddGameToCategory,
  RemoveGameFromCategory,
} from "../../bindings/yukihub/internal/service/categoryservice";
import { BigScreenAtmosphere } from "../bigscreen/BigScreenAtmosphere";
import { BigScreenBackground } from "../bigscreen/BigScreenBackground";
import { BigScreenDetailsLayer } from "../bigscreen/BigScreenDetailsLayer";
import { BigScreenHintBar } from "../bigscreen/BigScreenHintBar";
import { BigScreenInfoBar } from "../bigscreen/BigScreenInfoBar";
import { BigScreenRail } from "../bigscreen/BigScreenRail";
import { playBigScreenSound } from "../bigscreen/bigScreenSound";
import { BigScreenTrailerPlayer } from "../bigscreen/BigScreenTrailerPlayer";
import {
  BIG_SCREEN_CATEGORIES,
  fetchBigScreenGames,
  resolveFavoritesCategoryId,
} from "../bigscreen/categories";
import {
  BIG_SCREEN_ACTIONS_ZONE,
  BIG_SCREEN_DETAILS_ZONE,
  BIG_SCREEN_EXIT_PATH,
  BIG_SCREEN_FOCUS_ORDER,
  BIG_SCREEN_RAIL_ZONE,
  BIG_SCREEN_SHELF_ZONE,
  BIG_SCREEN_TRAILER_ZONE,
  resolveBigScreenEffectLevel,
} from "../bigscreen/constants";
import { useBigScreenFullscreen } from "../bigscreen/useBigScreenFullscreen";
import { useFocusEngine } from "../bigscreen/useFocusEngine";
import { useGameFavorite } from "../bigscreen/useGameFavorite";
import { useGamepad } from "../bigscreen/useGamepad";
import { useGameTags } from "../bigscreen/useGameTags";
import { useTrailerHover } from "../bigscreen/useTrailerHover";
import { VirtualGameShelf } from "../bigscreen/VirtualGameShelf";
import { useAppStore } from "../store";
import { Route as rootRoute } from "./__root";

const KEY_DIRECTIONS: Record<string, FocusDirection> = {
  ArrowDown: "down",
  ArrowLeft: "left",
  ArrowRight: "right",
  ArrowUp: "up",
};

const CATEGORY_COUNT = BIG_SCREEN_CATEGORIES.length;

/** 信息浮层的操作条目数：启动 / 收藏 / 详情 */
const ACTION_COUNT = 3;

/** 详情层的操作条目数：游玩 / 详细 / 观看 PV */
const DETAIL_ACTION_COUNT = 3;

/** 「观看 PV」在详情层操作里的下标，关闭播放器后把焦点还给它 */
const DETAIL_TRAILER_ACTION_INDEX = 2;

function normalizeCategoryId(value: string | undefined): BigScreenCategoryId {
  return BIG_SCREEN_CATEGORIES.some(category => category.id === value)
    ? (value as BigScreenCategoryId)
    : "recent";
}

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: "/bigscreen",
  component: BigScreenPage,
});

function BigScreenPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const startGame = useAppStore(state => state.startGame);
  const showHiddenGame = useAppStore(
    state => state.config?.bigscreen_show_hidden_game ?? false,
  );
  const defaultCategory = useAppStore(
    state => state.config?.bigscreen_default_category,
  );
  const effectLevel = useAppStore(state =>
    resolveBigScreenEffectLevel(state.config?.bigscreen_effect_level),
  );
  const soundEnabled = useAppStore(
    state => state.config?.bigscreen_sound_enabled ?? true,
  );

  const [activeCategory, setActiveCategory] = useState<BigScreenCategoryId>(
    () => normalizeCategoryId(defaultCategory),
  );
  const [games, setGames] = useState<models.Game[]>([]);
  const [reloadToken, setReloadToken] = useState(0);
  const [shelfIndex, setShelfIndex] = useState(0);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [trailerOpen, setTrailerOpen] = useState(false);
  const [shelfHeight, setShelfHeight] = useState(0);
  const [railHovered, setRailHovered] = useState(false);
  const [hintKey, setHintKey] = useState(0);
  // null = 用户还没用过任何输入设备，此时按手柄是否接入决定提示条形态
  const [inputDevice, setInputDevice] = useState<BigScreenInputDevice | null>(
    null,
  );
  const [entryAnimation, setEntryAnimation] = useState(true);
  const shelfAreaRef = useRef<HTMLDivElement | null>(null);
  const restoredCategoryRef = useRef<BigScreenCategoryId | null>(null);
  // 引擎在构造时就固定了 onBoundary，这里用 ref 把「之后才定义的处理函数」接进去
  const boundaryHandlerRef = useRef<
    (zoneId: string, direction: FocusDirection) => boolean
  >(() => false);

  useBigScreenFullscreen();

  // 入场错峰动画只播一次：时间窗结束后摘掉动画类，滚动时新挂载的卡片不再重放
  useEffect(() => {
    const timer = window.setTimeout(() => setEntryAnimation(false), 900);
    return () => window.clearTimeout(timer);
  }, []);

  const excludeHidden = !showHiddenGame;

  useEffect(() => {
    let active = true;
    void (async () => {
      try {
        const next = await fetchBigScreenGames(activeCategory, excludeHidden);
        if (active) {
          setGames(next);
        }
      }
      catch (error) {
        console.error("Failed to load big screen games:", error);
        if (active) {
          setGames([]);
        }
      }
    })();

    return () => {
      active = false;
    };
  }, [activeCategory, excludeHidden, reloadToken]);

  useLayoutEffect(() => {
    const element = shelfAreaRef.current;
    if (!element) {
      return;
    }

    const updateHeight = () => {
      // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect
      setShelfHeight(element.clientHeight);
    };
    updateHeight();

    const observer = new ResizeObserver(updateHeight);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  // 详情层 / 全屏播放器打开时把其余区域清空，让它成为唯一有内容的区域：
  // 否则上下键会顺着 verticalNeighbor 跳出浮层，firstPosition 也不会选中它。
  const zones = useMemo<FocusZone[]>(
    () =>
      trailerOpen
        ? [
            { id: BIG_SCREEN_RAIL_ZONE, rowLengths: [0] },
            { id: BIG_SCREEN_SHELF_ZONE, rowLengths: [0] },
            { id: BIG_SCREEN_ACTIONS_ZONE, rowLengths: [0] },
            { id: BIG_SCREEN_DETAILS_ZONE, rowLengths: [0] },
            { id: BIG_SCREEN_TRAILER_ZONE, rowLengths: [1] },
          ]
        : detailsOpen
          ? [
              { id: BIG_SCREEN_RAIL_ZONE, rowLengths: [0] },
              { id: BIG_SCREEN_SHELF_ZONE, rowLengths: [0] },
              { id: BIG_SCREEN_ACTIONS_ZONE, rowLengths: [0] },
              {
                id: BIG_SCREEN_DETAILS_ZONE,
                rowLengths: [DETAIL_ACTION_COUNT],
              },
            ]
          : [
              {
                id: BIG_SCREEN_RAIL_ZONE,
                rowLengths: BIG_SCREEN_CATEGORIES.map(() => 1),
              },
              { id: BIG_SCREEN_SHELF_ZONE, rowLengths: [games.length] },
              { id: BIG_SCREEN_ACTIONS_ZONE, rowLengths: [ACTION_COUNT] },
              { id: BIG_SCREEN_DETAILS_ZONE, rowLengths: [0] },
            ],
    [detailsOpen, games.length, trailerOpen],
  );

  const activeCategoryIndex = BIG_SCREEN_CATEGORIES.findIndex(
    category => category.id === activeCategory,
  );
  const safeShelfIndex = Math.min(shelfIndex, Math.max(0, games.length - 1));
  const focusedGame = games[safeShelfIndex];
  const { isFavorite, setFavorite } = useGameFavorite(focusedGame?.id);
  const tags = useGameTags(focusedGame?.id);
  // 特效档位为 off 时一并关掉背景预告片（语义是省电），浮层打开时也不该起播
  const backgroundTrailerActive = useTrailerHover(
    focusedGame?.id,
    focusedGame?.trailer_path,
    effectLevel !== "off" && !detailsOpen && !trailerOpen,
  );

  const { focus, move, position, restoreMemory, saveMemory } = useFocusEngine({
    onBoundary: (zoneId, direction) =>
      boundaryHandlerRef.current(zoneId, direction),
    verticalOrder: BIG_SCREEN_FOCUS_ORDER,
    zones,
  });

  const selectCategory = useCallback(
    (next: BigScreenCategoryId, moveRailFocus = false) => {
      if (next === activeCategory) {
        return;
      }
      // 按分类记忆货架焦点：离开前存当前分类，回来时再恢复
      saveMemory(activeCategory);
      setActiveCategory(next);
      if (moveRailFocus) {
        focus(
          BIG_SCREEN_RAIL_ZONE,
          BIG_SCREEN_CATEGORIES.findIndex(category => category.id === next),
        );
      }
    },
    [activeCategory, focus, saveMemory],
  );

  const stepCategory = useCallback(
    (delta: number, moveRailFocus: boolean) => {
      const nextIndex
        = (activeCategoryIndex + delta + CATEGORY_COUNT) % CATEGORY_COUNT;
      selectCategory(BIG_SCREEN_CATEGORIES[nextIndex].id, moveRailFocus);
    },
    [activeCategoryIndex, selectCategory],
  );

  const handleBoundary = useCallback(
    (zoneId: string, direction: FocusDirection) => {
      // 播放器是全屏浮层，方向键一律吞掉，不做任何区域间穿梭
      if (zoneId === BIG_SCREEN_TRAILER_ZONE) {
        return true;
      }

      if (zoneId === BIG_SCREEN_RAIL_ZONE) {
        // 侧栏是单列：左右切分类；向下交还给货架
        if (direction === "left") {
          stepCategory(-1, true);
          return true;
        }
        if (direction === "right") {
          stepCategory(1, true);
          return true;
        }
        if (direction === "down") {
          focus(BIG_SCREEN_SHELF_ZONE, safeShelfIndex);
          return true;
        }
        return true;
      }

      if (zoneId === BIG_SCREEN_SHELF_ZONE) {
        if (direction === "left" || direction === "up") {
          focus(BIG_SCREEN_RAIL_ZONE, activeCategoryIndex);
          return true;
        }
        if (direction === "right") {
          stepCategory(1, false);
          return true;
        }
      }

      return false;
    },
    [activeCategoryIndex, focus, safeShelfIndex, stepCategory],
  );

  useEffect(() => {
    boundaryHandlerRef.current = handleBoundary;
  }, [handleBoundary]);

  // 分类切换后回到该分类上次的焦点，没有记忆就从第一张开始（对齐手机端）。
  useEffect(() => {
    if (games.length === 0 || restoredCategoryRef.current === activeCategory) {
      return;
    }
    restoredCategoryRef.current = activeCategory;
    if (!restoreMemory(activeCategory)) {
      focus(BIG_SCREEN_SHELF_ZONE, 0);
    }
  }, [activeCategory, focus, games.length, restoreMemory]);

  // 焦点进入侧栏 / 按钮排后仍要保留「当前选中的游戏」，所以单独记一份货架下标。
  useEffect(() => {
    if (position.zoneId === BIG_SCREEN_SHELF_ZONE) {
      // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect
      setShelfIndex(position.index);
    }
  }, [position]);

  // 详情层开关时显式交接焦点：引擎的 setZones 只会把失效焦点退回区域首项，
  // 打开时进不了详情层按钮、关闭时会停在货架第一张，所以这里各补一次。
  const detailsOpenRef = useRef(detailsOpen);
  useEffect(() => {
    if (detailsOpenRef.current === detailsOpen) {
      return;
    }
    detailsOpenRef.current = detailsOpen;
    focus(
      detailsOpen ? BIG_SCREEN_DETAILS_ZONE : BIG_SCREEN_SHELF_ZONE,
      detailsOpen ? 0 : safeShelfIndex,
    );
  }, [detailsOpen, focus, safeShelfIndex]);

  // 播放器开关时同样显式交接焦点：打开进播放器，关闭把焦点还给「观看 PV」按钮
  const trailerOpenRef = useRef(trailerOpen);
  useEffect(() => {
    if (trailerOpenRef.current === trailerOpen) {
      return;
    }
    trailerOpenRef.current = trailerOpen;
    focus(
      trailerOpen ? BIG_SCREEN_TRAILER_ZONE : BIG_SCREEN_DETAILS_ZONE,
      trailerOpen ? 0 : DETAIL_TRAILER_ACTION_INDEX,
    );
  }, [focus, trailerOpen]);

  const handleStartGame = useCallback(
    (game: models.Game | undefined) => {
      if (!game?.id) {
        return;
      }
      void startGame(game);
    },
    [startGame],
  );

  const handleViewDetails = useCallback(
    (game: models.Game | undefined) => {
      if (!game?.id) {
        return;
      }
      void navigate({ to: `/game/${game.id}` });
    },
    [navigate],
  );

  /** 货架与信息浮层的「详情」都先展开大屏详情层，「详细」按钮才进完整详情页。 */
  const handleOpenDetails = useCallback((game: models.Game | undefined) => {
    if (!game?.id) {
      return;
    }
    setDetailsOpen(true);
  }, []);

  const handleToggleFavorite = useCallback(async () => {
    const gameId = focusedGame?.id;
    if (!gameId) {
      return;
    }

    try {
      const favoritesId = await resolveFavoritesCategoryId();
      if (!favoritesId) {
        toast.error(t("bigScreen.favoriteFailed"));
        return;
      }

      if (isFavorite) {
        await RemoveGameFromCategory(gameId, favoritesId);
        setFavorite(false);
        toast.success(t("bigScreen.favoriteRemoved"));
        // 在「收藏」分类里取消收藏需要即时刷新列表
        if (activeCategory === "favorites") {
          setReloadToken(token => token + 1);
        }
        return;
      }

      await AddGameToCategory(gameId, favoritesId);
      setFavorite(true);
      toast.success(t("bigScreen.favoriteAdded"));
    }
    catch (error) {
      console.error("Failed to toggle big screen favorite:", error);
      toast.error(t("bigScreen.favoriteFailed"));
    }
  }, [activeCategory, focusedGame?.id, isFavorite, setFavorite, t]);

  const actions = useMemo<BigScreenAction[]>(
    () => [
      {
        icon: "i-mdi-play",
        key: "start",
        label: t("gameCard.startGame"),
        run: () => handleStartGame(focusedGame),
      },
      {
        icon: "i-mdi-heart-outline",
        key: "favorite",
        label: isFavorite ? t("bigScreen.unfavorite") : t("bigScreen.favorite"),
        run: () => {
          void handleToggleFavorite();
        },
      },
      {
        icon: "i-mdi-information-variant",
        key: "details",
        label: t("common.details"),
        run: () => handleOpenDetails(focusedGame),
      },
    ],
    [
      focusedGame,
      handleOpenDetails,
      handleStartGame,
      handleToggleFavorite,
      isFavorite,
      t,
    ],
  );

  const detailActions = useMemo<BigScreenDetailAction[]>(
    () => [
      {
        icon: "i-mdi-play",
        key: "start",
        label: t("gameCard.startGame"),
        run: () => handleStartGame(focusedGame),
      },
      {
        icon: "i-mdi-information-outline",
        key: "details",
        label: t("common.details"),
        run: () => handleViewDetails(focusedGame),
      },
      {
        icon: "i-mdi-movie-open-outline",
        key: "trailer",
        label: t("bigScreen.trailer"),
        run: () => setTrailerOpen(true),
        disabled: !focusedGame?.trailer_path,
      },
    ],
    [focusedGame, handleStartGame, handleViewDetails, t],
  );

  const exitBigScreen = useCallback(() => {
    void navigate({ to: BIG_SCREEN_EXIT_PATH });
  }, [navigate]);

  const playSound = useCallback(
    (kind: BigScreenSound) => {
      if (soundEnabled) {
        playBigScreenSound(kind);
      }
    },
    [soundEnabled],
  );

  const markInputDevice = useCallback((device: BigScreenInputDevice) => {
    setInputDevice(current => (current === device ? current : device));
  }, []);

  /** 确认：详情层里跑详情层按钮，外层按当前聚焦的区域分别处理 */
  const activateFocused = useCallback(() => {
    // 播放器只吃返回键，确认不做任何事
    if (position.zoneId === BIG_SCREEN_TRAILER_ZONE) {
      return;
    }
    if (position.zoneId === BIG_SCREEN_DETAILS_ZONE) {
      const action = detailActions[position.index];
      // 禁用的条目只吞掉确认，不执行（例如没有本地预告片的「观看 PV」）
      if (action?.disabled) {
        return;
      }
      action?.run();
      return;
    }
    if (position.zoneId === BIG_SCREEN_RAIL_ZONE) {
      // 侧栏确认 = 进入内容区；切分类用左右键
      focus(BIG_SCREEN_SHELF_ZONE, safeShelfIndex);
      return;
    }
    if (position.zoneId === BIG_SCREEN_SHELF_ZONE) {
      handleStartGame(focusedGame);
      return;
    }
    if (position.zoneId === BIG_SCREEN_ACTIONS_ZONE) {
      actions[position.index]?.run();
    }
  }, [
    actions,
    detailActions,
    focus,
    focusedGame,
    handleStartGame,
    position.index,
    position.zoneId,
    safeShelfIndex,
  ]);

  /** 返回：播放器先吃掉，再是详情层，最后才是退出大屏 */
  const goBack = useCallback(() => {
    if (trailerOpen) {
      setTrailerOpen(false);
      return;
    }
    if (detailsOpen) {
      setDetailsOpen(false);
      return;
    }
    exitBigScreen();
  }, [detailsOpen, exitBigScreen, trailerOpen]);

  /**
   * 键盘与手柄共用的意图分发：两者只在「按键怎么翻译」上不同，
   * 翻译成意图之后的行为完全一致。
   */
  const dispatchIntent = useCallback(
    (intent: BigScreenIntent) => {
      switch (intent.type) {
        case "back": {
          setHintKey(key => key + 1);
          playSound("back");
          goBack();
          return;
        }
        case "category": {
          setHintKey(key => key + 1);
          playSound("move");
          stepCategory(intent.delta, false);
          return;
        }
        case "confirm": {
          setHintKey(key => key + 1);
          playSound("confirm");
          activateFocused();
          return;
        }
        case "details": {
          // 详情层已打开时 Y 不再额外做什么
          if (detailsOpen) {
            return;
          }
          setHintKey(key => key + 1);
          playSound("confirm");
          handleOpenDetails(focusedGame);
          return;
        }
        case "favorite": {
          setHintKey(key => key + 1);
          playSound("toggle");
          void handleToggleFavorite();
          return;
        }
        default: {
          setHintKey(key => key + 1);
          playSound("move");
          move(intent.direction);
        }
      }
    },
    [
      activateFocused,
      detailsOpen,
      focusedGame,
      goBack,
      handleOpenDetails,
      handleToggleFavorite,
      move,
      playSound,
      stepCategory,
    ],
  );

  const handleGamepadIntent = useCallback(
    (intent: BigScreenIntent) => {
      markInputDevice("gamepad");
      dispatchIntent(intent);
    },
    [dispatchIntent, markInputDevice],
  );

  const { connected: gamepadConnected } = useGamepad({
    onIntent: handleGamepadIntent,
  });

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const direction = KEY_DIRECTIONS[event.key];
      if (direction) {
        event.preventDefault();
        markInputDevice("keyboard");
        dispatchIntent({ direction, type: "move" });
        return;
      }

      if (event.key === "Escape") {
        event.preventDefault();
        markInputDevice("keyboard");
        dispatchIntent({ type: "back" });
        return;
      }

      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        markInputDevice("keyboard");
        dispatchIntent({ type: "confirm" });
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [dispatchIntent, markInputDevice]);

  const railFocused = position.zoneId === BIG_SCREEN_RAIL_ZONE;
  const isShelfFocused = position.zoneId === BIG_SCREEN_SHELF_ZONE;
  const coverUrl
    = focusedGame?.cover_url || focusedGame?.cover_source_url || "";
  // 用户还没用过任何输入设备时，按手柄是否接入决定提示条形态
  const hintDevice: BigScreenInputDevice
    = inputDevice ?? (gamepadConnected ? "gamepad" : "keyboard");

  return (
    <div className="relative flex h-screen w-screen select-none overflow-hidden bg-brand-900 text-white">
      <BigScreenBackground
        coverUrl={coverUrl}
        isNSFW={Boolean(focusedGame?.is_nsfw)}
        trailerUrl={focusedGame?.trailer_path}
        backgroundTrailerActive={backgroundTrailerActive}
      />
      <BigScreenAtmosphere
        coverUrl={coverUrl}
        level={effectLevel}
        seed={focusedGame?.id ?? ""}
      />

      <div className="relative z-10 flex h-full w-full flex-col">
        <div className="flex min-h-0 flex-1">
          <div
            className="flex h-full"
            onMouseEnter={() => setRailHovered(true)}
            onMouseLeave={() => setRailHovered(false)}
          >
            <BigScreenRail
              activeCategory={activeCategory}
              entryAnimation={entryAnimation}
              expanded={railFocused || railHovered}
              focused={railFocused}
              focusedIndex={position.index}
              onFocusIndexChange={index => focus(BIG_SCREEN_RAIL_ZONE, index)}
              onSelect={id => selectCategory(id)}
            />
          </div>

          <div ref={shelfAreaRef} className="min-h-0 flex-1 px-8 pt-6">
            {games.length > 0 && shelfHeight > 0 && (
              <VirtualGameShelf
                entryAnimation={entryAnimation}
                focused={isShelfFocused}
                focusedIndex={safeShelfIndex}
                games={games}
                onActivate={handleStartGame}
                onFocusIndexChange={index =>
                  focus(BIG_SCREEN_SHELF_ZONE, index)}
                onViewDetails={handleOpenDetails}
                rowHeight={shelfHeight}
              />
            )}

            {games.length === 0 && (
              <div className="flex h-full items-center justify-center text-sm text-brand-400">
                {t("bigScreen.empty")}
              </div>
            )}
          </div>
        </div>

        <div className="shrink-0 pb-4">
          <BigScreenInfoBar
            actions={actions}
            actionsFocused={position.zoneId === BIG_SCREEN_ACTIONS_ZONE}
            focusedActionIndex={position.index}
            game={focusedGame}
            isFavorite={isFavorite}
            onActionActivate={(index) => {
              focus(BIG_SCREEN_ACTIONS_ZONE, index);
              actions[index]?.run();
            }}
            onActionFocus={index => focus(BIG_SCREEN_ACTIONS_ZONE, index)}
            tags={tags}
          />
        </div>

        <BigScreenHintBar
          key={hintKey}
          className="shrink-0 animate-bigscreen-hint-dim px-10 pb-5"
          inputDevice={hintDevice}
          variant="main"
        />
      </div>

      {detailsOpen && focusedGame && (
        <BigScreenDetailsLayer
          actions={detailActions}
          actionsFocused={position.zoneId === BIG_SCREEN_DETAILS_ZONE}
          focusedActionIndex={position.index}
          game={focusedGame}
          inputDevice={hintDevice}
          onActionActivate={(index) => {
            focus(BIG_SCREEN_DETAILS_ZONE, index);
            detailActions[index]?.run();
          }}
          onActionFocus={index => focus(BIG_SCREEN_DETAILS_ZONE, index)}
          onClose={() => setDetailsOpen(false)}
          tags={tags}
        />
      )}

      {trailerOpen && focusedGame?.trailer_path && (
        <BigScreenTrailerPlayer
          title={focusedGame.name}
          url={focusedGame.trailer_path}
          onClose={() => setTrailerOpen(false)}
        />
      )}
    </div>
  );
}
