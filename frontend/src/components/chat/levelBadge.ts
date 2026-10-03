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

// 游玩时长的格式化统一用 utils/time 的 formatDuration / formatDurationCompact
// （本文件曾有一份 formatPlayTime，硬编码中文「N 小时 M 分钟」，英文界面也显示
// 中文，已删除；它依赖的 common.duration 单位键由 utils/time 共享）。
//
// 单位说明：服务端 /user/profile 的 totalPlayTime 与 recentGames[].playTime
// 都是**秒**（手机版 FriendsChatDialog.formatPlayTime(int seconds) 同样按秒读），
// 不要按同步格式的毫秒去换算。
