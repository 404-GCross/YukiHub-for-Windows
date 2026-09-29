import { useEffect, useState } from "react";

import { GetGameStats } from "../../bindings/yukihub/internal/service/statsservice";
import { enums } from "../../src/bindings/models";

/** 大屏每换一张卡都要一次时长，按游戏 id 缓存，避免来回重复查询。 */
const playtimeCache = new Map<string, number>();

/** 取单款游戏的总游玩时长（秒）。 */
export function useGamePlaytime(gameId: string | undefined) {
  const [playTime, setPlayTime] = useState(0);

  useEffect(() => {
    let active = true;
    const cached = gameId ? playtimeCache.get(gameId) : undefined;
    // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect
    setPlayTime(cached ?? 0);

    if (!gameId || cached !== undefined) {
      return;
    }

    void GetGameStats({
      game_id: gameId,
      dimension: enums.Period.All,
      start_date: "",
      end_date: "",
    })
      .then((stats) => {
        const next = Math.max(0, Number(stats?.total_play_time ?? 0));
        playtimeCache.set(gameId, next);
        if (active) {
          setPlayTime(next);
        }
      })
      .catch(() => {});

    return () => {
      active = false;
    };
  }, [gameId]);

  return playTime;
}
