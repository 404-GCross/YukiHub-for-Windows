import { useEffect, useState } from "react";

import type { models } from "../../src/bindings/models";

import { GetTagsByGame } from "../../bindings/yukihub/internal/service/tagservice";

/** 大屏里左右切游戏很频繁，标签按游戏 id 缓存，避免来回重复请求。 */
const tagsCache = new Map<string, models.GameTag[]>();

export function useGameTags(gameId: string | undefined) {
  const [tags, setTags] = useState<models.GameTag[]>([]);

  useEffect(() => {
    let active = true;
    const cached = gameId ? tagsCache.get(gameId) : undefined;
    // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect
    setTags(cached ?? []);

    if (!gameId || cached) {
      return;
    }

    void GetTagsByGame(gameId)
      .then((result) => {
        const next = result ?? [];
        tagsCache.set(gameId, next);
        if (active) {
          setTags(next);
        }
      })
      .catch(() => {});

    return () => {
      active = false;
    };
  }, [gameId]);

  return tags;
}
