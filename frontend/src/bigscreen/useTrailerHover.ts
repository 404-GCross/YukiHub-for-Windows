import { useEffect, useState } from "react";

import { BIG_SCREEN_TRAILER_HOVER_DELAY_MS } from "./constants";

/**
 * 货架焦点停留一段时间后才允许背景起播预告片。
 *
 * 停留时长来自用户的「起播延迟」偏好（`bigscreen_trailer_delay_ms`，
 * 手机端默认 2000ms）。`gameId` / `trailerUrl` / `enabled` / 时长任一变化都会取消
 * 计时并复位，因此快速划过卡片不会触发播放。
 */
export function useTrailerHover(
  gameId: string | undefined,
  trailerUrl: string | undefined,
  enabled: boolean,
  delayMs: number = BIG_SCREEN_TRAILER_HOVER_DELAY_MS,
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
      Math.max(0, delayMs),
    );
    return () => window.clearTimeout(timer);
  }, [delayMs, enabled, gameId, trailerUrl]);

  return active;
}
