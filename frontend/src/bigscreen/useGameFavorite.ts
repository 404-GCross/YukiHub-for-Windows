import { useCallback, useEffect, useState } from "react";

import { GetCategoriesByGame } from "../../bindings/yukihub/internal/service/categoryservice";

/** 焦点来回切换很频繁，收藏状态按游戏 id 缓存，避免重复请求分类关联。 */
const favoriteCache = new Map<string, boolean>();

export function useGameFavorite(gameId: string | undefined) {
  const [isFavorite, setIsFavorite] = useState(false);

  useEffect(() => {
    let active = true;
    const cached = gameId ? favoriteCache.get(gameId) : undefined;
    // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect
    setIsFavorite(cached ?? false);

    if (!gameId || cached !== undefined) {
      return;
    }

    void GetCategoriesByGame(gameId)
      .then((categories) => {
        const next = (categories ?? []).some(category => category.is_system);
        favoriteCache.set(gameId, next);
        if (active) {
          setIsFavorite(next);
        }
      })
      .catch(() => {});

    return () => {
      active = false;
    };
  }, [gameId]);

  /** 收藏切换后同步缓存与本地状态，避免等下一次请求才刷新按钮。 */
  const setFavorite = useCallback(
    (next: boolean) => {
      if (gameId) {
        favoriteCache.set(gameId, next);
      }
      setIsFavorite(next);
    },
    [gameId],
  );

  return { isFavorite, setFavorite };
}
