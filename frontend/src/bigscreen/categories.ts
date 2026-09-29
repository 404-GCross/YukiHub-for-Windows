import type { models, vo } from "../../src/bindings/models";

import {
  GetCategories,
  GetCategoryGames,
} from "../../bindings/yukihub/internal/service/categoryservice";
import { GetGames } from "../../bindings/yukihub/internal/service/gameservice";
import { enums } from "../../src/bindings/models";
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

type CategoryQueryPlan = {
  sortBy: enums.GameListSortBy;
  sortOrder: enums.SortOrder;
  status?: enums.GameStatus;
};

/**
 * 分类 → 后端查询参数的映射。
 * 「全部」按名称正序便于浏览，「最近」按最近游玩时间倒序（未玩过的排在末尾）。
 */
const CATEGORY_QUERY_PLANS: Record<BigScreenCategoryId, CategoryQueryPlan> = {
  all: {
    sortBy: enums.GameListSortBy.GameListSortByName,
    sortOrder: enums.SortOrder.SortOrderAsc,
  },
  favorites: {
    sortBy: enums.GameListSortBy.GameListSortByLastPlayedAt,
    sortOrder: enums.SortOrder.SortOrderDesc,
  },
  recent: {
    sortBy: enums.GameListSortBy.GameListSortByLastPlayedAt,
    sortOrder: enums.SortOrder.SortOrderDesc,
  },
  playing: {
    status: enums.GameStatus.StatusPlaying,
    sortBy: enums.GameListSortBy.GameListSortByLastPlayedAt,
    sortOrder: enums.SortOrder.SortOrderDesc,
  },
  completed: {
    status: enums.GameStatus.StatusCompleted,
    sortBy: enums.GameListSortBy.GameListSortByLastPlayedAt,
    sortOrder: enums.SortOrder.SortOrderDesc,
  },
  unplayed: {
    status: enums.GameStatus.StatusUnplayed,
    sortBy: enums.GameListSortBy.GameListSortByName,
    sortOrder: enums.SortOrder.SortOrderAsc,
  },
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
  plan: CategoryQueryPlan,
  excludeHidden: boolean,
  limit: number,
  offset: number,
): vo.GameListRequest {
  return {
    limit,
    offset,
    search_query: "",
    status: plan.status ?? null,
    exclude_hidden: excludeHidden,
    tags: [],
    sort_by: plan.sortBy,
    sort_order: plan.sortOrder,
    secondary_sort_by: enums.GameListSortBy.$zero,
    secondary_sort_order: enums.SortOrder.$zero,
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
  excludeHidden: boolean,
): Promise<models.Game[]> {
  const plan = CATEGORY_QUERY_PLANS[categoryId];
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
    const request = buildRequest(plan, excludeHidden, limit, games.length);
    const response = categoryFilter
      ? await GetCategoryGames({
          ...request,
          category_id: favoritesId,
        } as vo.CategoryGameListRequest)
      : await GetGames(request);

    const page = response.games ?? [];
    games.push(...page);
    if (!response.has_more || page.length === 0) {
      break;
    }
  }
  return games;
}
