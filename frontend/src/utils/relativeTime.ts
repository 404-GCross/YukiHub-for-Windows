import type { TFunction } from "i18next";

/**
 * 相对时间（毫秒时间戳 → 「刚刚 / N 分钟前 / N 小时前 / N 天前 / yyyy-MM-dd」）。
 *
 * 档位与手机版 FriendsChatDialog.formatRelativeTime 完全一致：超过 30 天
 * 就退回绝对日期，否则越近粒度越细。桌面端补上了 i18n —— 手机版是硬编码中文。
 *
 * @param timestamp 毫秒时间戳；<= 0 视为无数据，返回空串（调用方据此隐藏）
 * @param t i18n 翻译函数
 * @param now 当前时间戳，默认取当前时间（测试时可注入）
 */
export function formatRelativeTime(
  timestamp: number,
  t: TFunction,
  now: number = Date.now(),
): string {
  if (!timestamp || timestamp <= 0) {
    return "";
  }
  // 服务端给的是秒而非毫秒时（时间戳看起来太小），按秒补齐，避免显示「55 年前」
  const normalized = timestamp < 1e11 ? timestamp * 1000 : timestamp;
  const diff = Math.max(0, now - normalized);

  const minutes = Math.floor(diff / (60 * 1000));
  const hours = Math.floor(diff / (60 * 60 * 1000));
  const days = Math.floor(diff / (24 * 60 * 60 * 1000));

  if (days > 30) {
    return new Date(normalized).toLocaleDateString();
  }
  if (days >= 1) {
    return t("common.relativeTime.daysAgo", { days });
  }
  if (hours >= 1) {
    return t("common.relativeTime.hoursAgo", { hours });
  }
  if (minutes >= 1) {
    return t("common.relativeTime.minutesAgo", { minutes });
  }
  return t("common.relativeTime.justNow");
}
