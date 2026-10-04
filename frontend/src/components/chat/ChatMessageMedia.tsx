import { resolveChatMediaURL } from "../../utils/chatMedia";
import { proxiedImageSrc } from "../../utils/imageProxy";

/** 表情列表项（只用到这两个字段，避免依赖完整模型）。 */
export interface ChatEmojiLike {
  name?: string;
  url?: string;
}

/**
 * 表情名 → 可显示 URL。
 *
 * `content` 可能是三种东西：完整 http(s) 地址、本站表情的**名字**、或已补全的路径。
 * 只有第一种能直接当 src；名字必须查 `/chat/emojis` 下发的映射 —— 直接拼
 * `uploads/emojis/<名>.webp` 对改名或子目录里的表情会 404（就是丢图）。
 *
 * 与手机版 `emojiUrlMap` 同一套规则：先查映射，查不到再按老规则兜底。
 */
export function emojiDisplayURL(
  content: string,
  emojis?: ChatEmojiLike[] | null,
): string {
  if (/^https?:\/\//i.test(content)) {
    return content;
  }
  const mapped = emojis?.find(emoji => emoji.name === content);
  if (mapped?.url) {
    return mapped.url;
  }
  return `https://yukihub.zh.kg/uploads/emojis/${encodeURIComponent(content)}.webp`;
}

interface ChatMessageMediaProps {
  /** "image" 或 "emoji"，其它值不渲染图片 */
  msgType: string;
  content: string;
  /** 表情映射表（`ListChatEmojis` 的结果），没有就退回按名字拼路径 */
  emojis?: ChatEmojiLike[] | null;
  className?: string;
}

/**
 * 聊天消息里的图片 / 表情。
 *
 * 两条加载尝试都要做，**顺序不能省**：
 * 1. 先走图片代理（国内直连不通的域名、防盗链的站点都得靠它）；
 * 2. 代理失败再退回**直连原地址** —— 少这一步的话，代理偶发失败就是永久丢图
 *    （用户看到的是一个个碎图标）。
 *
 * 浮层与主界面的聊天共用这个组件：以前各写一份，浮层那份少了第 2 步和表情映射，
 * 于是同一个会话在主界面看得见图、在浮层里全是碎图标。
 */
export function ChatMessageMedia({
  msgType,
  content,
  emojis,
  className,
}: ChatMessageMediaProps) {
  if (msgType !== "image" && msgType !== "emoji") {
    return null;
  }

  const isEmoji = msgType === "emoji";
  const raw = isEmoji
    ? emojiDisplayURL(content, emojis)
    : resolveChatMediaURL(content);
  if (!raw) {
    return null;
  }

  return (
    <img
      src={proxiedImageSrc(raw)}
      alt=""
      loading="lazy"
      className={
        className
        ?? (isEmoji
          ? "h-24 w-24 object-contain"
          : "max-h-48 rounded-xl object-contain")
      }
      onError={(event) => {
        // 代理没拿到就直连。用「已经为哪个 raw 回退过」而不是一个布尔标记：
        // 表情映射表是异步到达的，raw 变了之后要允许再走一轮回退，
        // 布尔标记会把后到的那次改进永远挡在门外。
        const img = event.currentTarget;
        if (img.dataset.fallbackFor !== raw && img.src !== raw) {
          img.dataset.fallbackFor = raw;
          img.src = raw;
        }
      }}
    />
  );
}
