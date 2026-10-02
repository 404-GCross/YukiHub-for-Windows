import type { CSSProperties } from "react";

/**
 * 社区等级徽章配色（分档与手机版 levelColor / levelBadgeBg 完全一致）。
 *
 * 手机版返回的是 drawable 背景 + 文字色，桌面端用同色系的半透明底 + 描边还原。
 * 聊天室气泡昵称旁与用户资料页共用这一份，避免两处档位跑偏。
 */
export function levelBadgeStyle(level: number): CSSProperties {
  const color
    = level >= 30
      ? "#FFD27A"
      : level >= 25
        ? "#FF9090"
        : level >= 20
          ? "#FFB37A"
          : level >= 15
            ? "#C9A0FF"
            : level >= 10
              ? "#7DB8FF"
              : level >= 5
                ? "#7EE2A0"
                : "#B9BCC7";
  return {
    color,
    backgroundColor: `${color}22`,
    border: `1px solid ${color}66`,
  };
}

/** 游玩时长（秒）→ 「N 小时 M 分钟」；不足 1 小时只显示分钟。 */
export function formatPlayTime(seconds: number): string {
  if (!seconds || seconds <= 0) {
    return "-";
  }
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (hours <= 0) {
    return `${minutes} 分钟`;
  }
  return minutes > 0 ? `${hours} 小时 ${minutes} 分钟` : `${hours} 小时`;
}
