import type { AvatarFrame } from "../../../bindings/yukihub/internal/service/yukihubaccount/models";

import { resolveChatMediaURL } from "../../utils/chatMedia";
import { proxiedImageSrc } from "../../utils/imageProxy";

/**
 * 头像框尺寸换算（对齐手机版 AvatarFrame / 网页端 community.js）：
 *
 *   框边长   = 头像边长 × scale
 *   横向位移 = 头像边长 × offsetX / 100
 *   纵向位移 = 头像边长 × offsetY / 100
 *
 * 三处（手机端、网页端、桌面端）必须完全一致，否则管理员在后台调好的框
 * 在不同端看起来不是一回事。
 */
export function frameBoxSize(avatarSize: number, frame?: FrameLike | null) {
  const scale = frame?.scale && frame.scale > 0 ? frame.scale : 1.4;
  return Math.round(avatarSize * scale);
}

type FrameLike = AvatarFrame;

interface ChatAvatarProps {
  name: string;
  avatar?: string;
  /** 正方形边长（px） */
  size: number;
  frame?: FrameLike | null;
  className?: string;
  /** 点击头像（打开用户资料页） */
  onClick?: () => void;
  /** 右键/长按头像（群聊里插入 @提及，对齐手机版 mentionInGroup） */
  onContextMenu?: (event: {
    preventDefault: () => void;
    clientX: number;
    clientY: number;
  }) => void;
}

/**
 * 昵称 → 头像底色（手机版 avatarBgColor 的等价实现）。
 *
 * 同一用户在任何地方颜色一致；算法是「hash 的低 15 位拆成 RGB 的增量」，
 * 保证偏暗、不刺眼，白字始终可读。
 */
export function avatarBgColor(name: string): string {
  if (!name) {
    return "rgb(69, 90, 100)";
  }
  let hash = 0;
  for (let index = 0; index < name.length; index += 1) {
    hash = (hash * 31 + name.charCodeAt(index)) | 0;
  }
  const positive = Math.abs(hash);
  const r = 40 + (positive & 0x7F);
  const g = 60 + ((positive >> 7) & 0x7F);
  const b = 80 + ((positive >> 14) & 0x7F);
  return `rgb(${r}, ${g}, ${b})`;
}

/**
 * 聊天头像（可选头像框）。
 *
 * 有框时外层槽位按框的尺寸撑开（框边长 > 头像边长），头像在槽位里居中，
 * 框以绝对定位盖在最上层 —— 与手机版 wrapAvatarWithFrame / applyFrameToBox 同构。
 */
export function ChatAvatar({
  name,
  avatar,
  size,
  frame,
  className = "",
  onClick,
  onContextMenu,
}: ChatAvatarProps) {
  const hasFrame = Boolean(frame?.imageUrl);
  const boxSize = hasFrame ? frameBoxSize(size, frame) : size;
  const offsetX = hasFrame
    ? Math.round((size * (frame?.offsetX ?? 0)) / 100)
    : 0;
  const offsetY = hasFrame
    ? Math.round((size * (frame?.offsetY ?? 0)) / 100)
    : 0;
  const inset = Math.round((boxSize - size) / 2);

  return (
    <div
      className={`relative shrink-0 ${className}`}
      style={{ width: boxSize, height: boxSize }}
      onClick={onClick}
      onContextMenu={(event) => {
        if (!onContextMenu) {
          return;
        }
        event.preventDefault();
        onContextMenu(event);
      }}
    >
      {avatar ? (
        <img
          src={avatar}
          alt=""
          loading="lazy"
          className="absolute rounded-full object-cover"
          style={{
            width: size,
            height: size,
            left: inset + offsetX,
            top: inset + offsetY,
          }}
          onError={(e) => {
            // 外链头像被墙时退回图片代理再试一次，仍失败则由浏览器显示占位
            const img = e.currentTarget;
            const proxied = proxiedImageSrc(avatar);
            if (proxied !== avatar && img.src !== proxied) {
              img.src = proxied;
            }
          }}
        />
      ) : (
        <div
          className="absolute flex items-center justify-center rounded-full font-semibold text-white/95"
          style={{
            width: size,
            height: size,
            left: inset + offsetX,
            top: inset + offsetY,
            backgroundColor: avatarBgColor(name),
            fontSize: Math.max(10, Math.round(size * 0.42)),
          }}
        >
          {(name || "?").slice(0, 1).toUpperCase()}
        </div>
      )}
      {hasFrame && (
        <img
          // 框素材与聊天图片同源，服务端下发的是相对路径，必须先补全站点地址
          src={proxiedImageSrc(resolveChatMediaURL(frame?.imageUrl))}
          alt=""
          loading="lazy"
          className="pointer-events-none absolute"
          style={{
            width: boxSize,
            height: boxSize,
            left: offsetX,
            top: offsetY,
          }}
          onError={(e) => {
            // 代理失败时退回直连原地址；再失败就自然显示为空（不影响头像本身）
            const img = e.currentTarget;
            const direct = resolveChatMediaURL(frame?.imageUrl);
            if (direct && img.src !== direct) {
              img.src = direct;
            }
          }}
        />
      )}
    </div>
  );
}
