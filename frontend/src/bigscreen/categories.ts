import type { models, vo } from "../../src/bindings/models";

import {
  GetCategories,
  GetCategoryGames,
} from "../../bindings/yukihub/internal/service/categoryservice";
import { GetGames } from "../../bindings/yukihub/internal/service/gameservice";
import { enums } from "../../src/bindings/models";
import { primeGamePlaytimes } from "../hooks/useGamePlaytime";
import { BIG_SCREEN_PAGE_LIMIT, BIG_SCREEN_SHELF_LIMIT } from "./constants";

/**
 * 大屏侧栏的六个分类，对齐手机端 ALL/FAV/RECENT/PLAYING/DONE/TODO：
 * `all` / `favorites` / `recent` / `playing` / `completed` / `unplayed`（TODO 即未玩）。
 * 「收藏」不是游戏字段而是系统分类（`system:favorites`），因此走 `GetCategoryGames`。
 */
export type BigScreenCategoryId
  = | "all"
    | "favorites"
    | "recent"
    | "playing"
    | "completed"
    | "unplayed";

export type BigScreenCategory = {
  /** 分类图标（UnoCSS mdi 名） */
  icon: string;
  id: BigScreenCategoryId;
  /** i18n 键；状态类分类直接复用 `common.*` 下的既有译名 */
  labelKey: string;
};

export const BIG_SCREEN_CATEGORIES: BigScreenCategory[] = [
  {
    icon: "i-mdi-view-grid-outline",
    id: "all",
    labelKey: "bigScreen.categoryAll",
  },
  {
    icon: "i-mdi-heart-outline",
    id: "favorites",
    labelKey: "categories.favorites",
  },
  { icon: "i-mdi-history", id: "recent", labelKey: "bigScreen.categoryRecent" },
  {
    icon: "i-mdi-gamepad-variant-outline",
    id: "playing",
    labelKey: "common.playing",
  },
  {
    icon: "i-mdi-trophy-outline",
    id: "completed",
    labelKey: "common.completed",
  },
  {
    icon: "i-mdi-clock-outline",
    id: "unplayed",
    labelKey: "common.unplayed",
  },
];

/** 排序方式，对应手机端主菜单「切换排序方式」的三种（recent / newest / name） */
export type BigScreenSortMode = "name" | "newest" | "recent";

export const BIG_SCREEN_SORT_MODES: BigScreenSortMode[] = [
  "recent",
  "newest",
  "name",
];

const SORT_PLANS: Record<
  BigScreenSortMode,
  { sortBy: enums.GameListSortBy; sortOrder: enums.SortOrder }
> = {
  name: {
    sortBy: enums.GameListSortBy.GameListSortByName,
    sortOrder: enums.SortOrder.SortOrderAsc,
  },
  newest: {
    sortBy: enums.GameListSortBy.GameListSortByCreatedAt,
    sortOrder: enums.SortOrder.SortOrderDesc,
  },
  recent: {
    sortBy: enums.GameListSortBy.GameListSortByLastPlayedAt,
    sortOrder: enums.SortOrder.SortOrderDesc,
  },
};

/** 分类的状态过滤条件（排序交给用户选的排序方式统一决定） */
const CATEGORY_STATUS_FILTER: Partial<
  Record<BigScreenCategoryId, enums.GameStatus>
> = {
  completed: enums.GameStatus.StatusCompleted,
  playing: enums.GameStatus.StatusPlaying,
  unplayed: enums.GameStatus.StatusUnplayed,
};

/** 收藏分类的 id 由后端按 `is_system` 标记返回，前端不硬编码。 */
let favoritesCategoryIdPromise: Promise<string> | null = null;

export function resolveFavoritesCategoryId(): Promise<string> {
  if (!favoritesCategoryIdPromise) {
    favoritesCategoryIdPromise = GetCategories()
      .then(
        categories =>
          (categories ?? []).find(category => category.is_system)?.id ?? "",
      )
      .catch(() => "");
  }
  return favoritesCategoryIdPromise;
}

function buildRequest(
  categoryId: BigScreenCategoryId,
  sortMode: BigScreenSortMode,
  excludeHidden: boolean,
  limit: number,
  offset: number,
): vo.GameListRequest {
  const sort = SORT_PLANS[sortMode];
  return {
    limit,
    offset,
    search_query: "",
    status: CATEGORY_STATUS_FILTER[categoryId] ?? null,
    exclude_hidden: excludeHidden,
    tags: [],
    sort_by: sort.sortBy,
    sort_order: sort.sortOrder,
    secondary_sort_by: enums.GameListSortBy.$zero,
    secondary_sort_order: enums.SortOrder.$zero,
    // 信息浮层与详情层都要显示时长，一次批量带回胜过每次切卡单查。
    with_play_time: true,
  } as vo.GameListRequest;
}

/**
 * 按分类取货架数据。
 *
 * 后端单次查询上限是 `MaxGameListLimit`（240），这里按 `BIG_SCREEN_SHELF_LIMIT`
 * 逐页补齐，保证货架真的能放满 500 张。
 */
export async function fetchBigScreenGames(
  categoryId: BigScreenCategoryId,
  sortMode: BigScreenSortMode,
  excludeHidden: boolean,
): Promise<models.Game[]> {
  const categoryFilter = categoryId === "favorites";
  const favoritesId = categoryFilter ? await resolveFavoritesCategoryId() : "";
  if (categoryFilter && !favoritesId) {
    return [];
  }

  const games: models.Game[] = [];
  while (games.length < BIG_SCREEN_SHELF_LIMIT) {
    const limit = Math.min(
      BIG_SCREEN_PAGE_LIMIT,
      BIG_SCREEN_SHELF_LIMIT - games.length,
    );
    const request = buildRequest(
      categoryId,
      sortMode,
      excludeHidden,
      limit,
      games.length,
    );
    const response = categoryFilter
      ? await GetCategoryGames({
          ...request,
          category_id: favoritesId,
        } as vo.CategoryGameListRequest)
      : await GetGames(request);

    const page = response.games ?? [];
    games.push(...page);
    primeGamePlaytimes(response.play_times ?? undefined);
    if (!response.has_more || page.length === 0) {
      break;
    }
  }
  return games;
}

/**
 * 取各分类的条目数（侧栏展开时显示）。
 *
 * 手机端是「一次性把整库读进内存再各自 filter」，桌面端每次只取当前分类，
 * 所以这里用 `limit=1` 逐分类取总数 —— 只跑 COUNT，不搬数据。
 */
export async function fetchBigScreenCategoryCounts(
  sortMode: BigScreenSortMode,
  excludeHidden: boolean,
): Promise<Partial<Record<BigScreenCategoryId, number>>> {
  const counts: Partial<Record<BigScreenCategoryId, number>> = {};
  await Promise.all(
    BIG_SCREEN_CATEGORIES.map(async (category) => {
      const categoryFilter = category.id === "favorites";
      const favoritesId = categoryFilter
        ? await resolveFavoritesCategoryId()
        : "";
      if (categoryFilter && !favoritesId) {
        counts[category.id] = 0;
        return;
      }
      try {
        const request = buildRequest(
          category.id,
          sortMode,
          excludeHidden,
          1,
          0,
        );
        const response = categoryFilter
          ? await GetCategoryGames({
              ...request,
              category_id: favoritesId,
            } as vo.CategoryGameListRequest)
          : await GetGames(request);
        counts[category.id] = response.total ?? 0;
      }
      catch {
        // 单个分类计数失败不该影响侧栏其它项
      }
    }),
  );
  return counts;
}
