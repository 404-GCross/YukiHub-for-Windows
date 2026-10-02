/**
 * 聊天相关媒体地址（消息图片、头像框素材）的补全。
 *
 * 服务端下发的这两类地址都是**相对路径**（如 `/uploads/chat/xxx.jpg`、
 * `/uploads/frames/xx.png`），直接塞进 `<img src>` 会被 WebView 解析成
 * `wails.localhost/uploads/...` → 404，表现就是「自己发的图看不见」
 * 「头像框加载不出来」。
 *
 * 手机版在渲染层用 `absoluteChatImageUrl()` 补前缀（见 FriendsChatDialog），
 * 桌面端保持同一策略：存储层保留相对路径（双端数据完全一致），只在显示时补全。
 */
const YUKIHUB_SITE_BASE = "https://yukihub.zh.kg";

export function resolveChatMediaURL(url: string | null | undefined): string {
  const value = (url ?? "").trim();
  if (!value) {
    return "";
  }
  if (/^https?:\/\//i.test(value)) {
    return value;
  }
  return `${YUKIHUB_SITE_BASE}${value.startsWith("/") ? value : `/${value}`}`;
}
