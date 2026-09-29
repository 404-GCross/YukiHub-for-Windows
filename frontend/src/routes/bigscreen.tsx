import type { models } from "../../src/bindings/models";
import type { BigScreenAction } from "../bigscreen/BigScreenInfoBar";
import type { BigScreenCategoryId } from "../bigscreen/categories";
import type { FocusDirection, FocusZone } from "../bigscreen/focusEngine";
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
import { BigScreenBackground } from "../bigscreen/BigScreenBackground";
import { BigScreenInfoBar } from "../bigscreen/BigScreenInfoBar";
import { BigScreenRail } from "../bigscreen/BigScreenRail";
import {
  BIG_SCREEN_CATEGORIES,
  fetchBigScreenGames,
  resolveFavoritesCategoryId,
} from "../bigscreen/categories";
import {
  BIG_SCREEN_ACTIONS_ZONE,
  BIG_SCREEN_EXIT_PATH,
  BIG_SCREEN_FOCUS_ORDER,
  BIG_SCREEN_RAIL_ZONE,
  BIG_SCREEN_SHELF_ZONE,
} from "../bigscreen/constants";
import { useBigScreenFullscreen } from "../bigscreen/useBigScreenFullscreen";
import { useFocusEngine } from "../bigscreen/useFocusEngine";
import { useGameFavorite } from "../bigscreen/useGameFavorite";
import { useGameTags } from "../bigscreen/useGameTags";
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

  const [activeCategory, setActiveCategory] = useState<BigScreenCategoryId>(
    () => normalizeCategoryId(defaultCategory),
  );
  const [games, setGames] = useState<models.Game[]>([]);
  const [reloadToken, setReloadToken] = useState(0);
  const [shelfIndex, setShelfIndex] = useState(0);
  const [shelfHeight, setShelfHeight] = useState(0);
  const [railHovered, setRailHovered] = useState(false);
  const [hintKey, setHintKey] = useState(0);
  const shelfAreaRef = useRef<HTMLDivElement | null>(null);
  const restoredCategoryRef = useRef<BigScreenCategoryId | null>(null);
  // 引擎在构造时就固定了 onBoundary，这里用 ref 把「之后才定义的处理函数」接进去
  const boundaryHandlerRef = useRef<
    (zoneId: string, direction: FocusDirection) => boolean
  >(() => false);

  useBigScreenFullscreen();

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

  const zones = useMemo<FocusZone[]>(
    () => [
      {
        id: BIG_SCREEN_RAIL_ZONE,
        rowLengths: BIG_SCREEN_CATEGORIES.map(() => 1),
      },
      { id: BIG_SCREEN_SHELF_ZONE, rowLengths: [games.length] },
      { id: BIG_SCREEN_ACTIONS_ZONE, rowLengths: [ACTION_COUNT] },
    ],
    [games.length],
  );

  const activeCategoryIndex = BIG_SCREEN_CATEGORIES.findIndex(
    category => category.id === activeCategory,
  );
  const safeShelfIndex = Math.min(shelfIndex, Math.max(0, games.length - 1));
  const focusedGame = games[safeShelfIndex];
  const { isFavorite, setFavorite } = useGameFavorite(focusedGame?.id);
  const tags = useGameTags(focusedGame?.id);

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
        run: () => handleViewDetails(focusedGame),
      },
    ],
    [
      focusedGame,
      handleStartGame,
      handleToggleFavorite,
      handleViewDetails,
      isFavorite,
      t,
    ],
  );

  const exitBigScreen = useCallback(() => {
    void navigate({ to: BIG_SCREEN_EXIT_PATH });
  }, [navigate]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        exitBigScreen();
        return;
      }

      const direction = KEY_DIRECTIONS[event.key];
      if (direction) {
        event.preventDefault();
        setHintKey(key => key + 1);
        move(direction);
        return;
      }

      if (event.key !== "Enter" && event.key !== " ") {
        return;
      }

      event.preventDefault();
      setHintKey(key => key + 1);

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
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [
    actions,
    exitBigScreen,
    focus,
    focusedGame,
    handleStartGame,
    move,
    position.index,
    position.zoneId,
    safeShelfIndex,
  ]);

  const railFocused = position.zoneId === BIG_SCREEN_RAIL_ZONE;
  const isShelfFocused = position.zoneId === BIG_SCREEN_SHELF_ZONE;
  const coverUrl
    = focusedGame?.cover_url || focusedGame?.cover_source_url || "";

  return (
    <div className="relative flex h-screen w-screen select-none overflow-hidden bg-brand-900 text-white">
      <BigScreenBackground
        coverUrl={coverUrl}
        isNSFW={Boolean(focusedGame?.is_nsfw)}
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
                focused={isShelfFocused}
                focusedIndex={safeShelfIndex}
                games={games}
                onActivate={handleStartGame}
                onFocusIndexChange={index =>
                  focus(BIG_SCREEN_SHELF_ZONE, index)}
                onViewDetails={handleViewDetails}
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

        <div
          key={hintKey}
          className="flex shrink-0 animate-bigscreen-hint-dim items-center gap-6 px-10 pb-5 text-xs text-brand-400"
        >
          <span className="inline-flex items-center gap-2">
            <kbd className="rounded border border-brand-700 px-1.5 py-0.5 font-sans">
              ← →
            </kbd>
            {t("bigScreen.hintMove")}
          </span>
          <span className="inline-flex items-center gap-2">
            <kbd className="rounded border border-brand-700 px-1.5 py-0.5 font-sans">
              Enter
            </kbd>
            {t("bigScreen.hintConfirm")}
          </span>
          <span className="inline-flex items-center gap-2">
            <kbd className="rounded border border-brand-700 px-1.5 py-0.5 font-sans">
              Esc
            </kbd>
            {t("bigScreen.hintExit")}
          </span>
        </div>
      </div>
    </div>
  );
}
