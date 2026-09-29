import { useEffect, useState } from "react";

import { BIG_SCREEN_TRAILER_HOVER_DELAY_MS } from "./constants";

/**
 * 货架焦点停留 `BIG_SCREEN_TRAILER_HOVER_DELAY_MS` 后才允许背景起播预告片。
 *
 * `gameId` / `trailerUrl` / `enabled` 任一变化都会取消计时并复位，
 * 因此快速划过卡片不会触发播放。
 */
export function useTrailerHover(
  gameId: string | undefined,
  trailerUrl: string | undefined,
  enabled: boolean,
): boolean {
  const [active, setActive] = useState(false);

  useEffect(() => {
    // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect
    setActive(false);
    if (!enabled || !gameId || !trailerUrl) {
      return;
    }

    const timer = window.setTimeout(
      () => setActive(true),
      BIG_SCREEN_TRAILER_HOVER_DELAY_MS,
    );
    return () => window.clearTimeout(timer);
  }, [enabled, gameId, trailerUrl]);

  return active;
}
