/**
 * 添加游戏弹窗的两张插画（本地导入 / 远程导入）。
 *
 * 上游留下的是两张 webp 位图（不含品牌文字，但仍是上游素材），这里改为
 * 内联 SVG：与顶部文字 logo、未萌图标一样，**不引入新的二进制资源**，
 * 颜色随主题走 currentColor + 品牌色，也不会有 4K 屏下位图发虚的问题。
 */

type AddGameIllustrationVariant = "local" | "remote";

interface AddGameIllustrationProps {
  className?: string;
  variant: AddGameIllustrationVariant;
}

export function AddGameIllustration({
  className,
  variant,
}: AddGameIllustrationProps) {
  const gradientId = `agi-${variant}-surface`;

  return (
    <svg
      viewBox="0 0 200 180"
      className={className}
      role="presentation"
      aria-hidden="true"
      focusable="false"
    >
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#8AB4FF" />
          <stop offset="100%" stopColor="#FF8AB3" />
        </linearGradient>
      </defs>

      {variant === "local" ? (
        <LocalArt gradientId={gradientId} />
      ) : (
        <RemoteArt gradientId={gradientId} />
      )}
    </svg>
  );
}

/** 本地导入：一台立着的游戏窗口 + 手柄方向键，配合雪花点缀。 */
function LocalArt({ gradientId }: { gradientId: string }) {
  return (
    <g>
      <rect
        x="26"
        y="26"
        width="148"
        height="96"
        rx="16"
        fill={`url(#${gradientId})`}
        opacity="0.35"
      />
      <rect
        x="26"
        y="26"
        width="148"
        height="96"
        rx="16"
        fill="none"
        stroke="currentColor"
        strokeWidth="3"
        opacity="0.55"
      />
      {/* 窗口里的播放三角 */}
      <path d="M92 62 L116 74 L92 86 Z" fill="currentColor" opacity="0.75" />
      {/* 底座与支柱 */}
      <rect
        x="88"
        y="122"
        width="24"
        height="18"
        fill="currentColor"
        opacity="0.45"
      />
      <rect
        x="58"
        y="140"
        width="84"
        height="12"
        rx="6"
        fill="currentColor"
        opacity="0.45"
      />
      {/* 手柄方向键 */}
      <g
        stroke="currentColor"
        strokeWidth="4"
        strokeLinecap="round"
        opacity="0.6"
      >
        <path d="M150 128 v14 M143 135 h14" />
      </g>
      <circle cx="46" cy="150" r="6" fill="currentColor" opacity="0.35" />
      <circle cx="168" cy="44" r="4" fill="currentColor" opacity="0.3" />
    </g>
  );
}

/** 远程导入：一朵云 + 下载箭头，配合雪花点缀。 */
function RemoteArt({ gradientId }: { gradientId: string }) {
  return (
    <g>
      <path
        d="M62 116 a30 30 0 0 1 4-58 a38 38 0 0 1 70 6 a26 26 0 0 1 6 52 Z"
        fill={`url(#${gradientId})`}
        opacity="0.35"
      />
      <path
        d="M62 116 a30 30 0 0 1 4-58 a38 38 0 0 1 70 6 a26 26 0 0 1 6 52 Z"
        fill="none"
        stroke="currentColor"
        strokeWidth="3"
        opacity="0.55"
      />
      {/* 下载箭头 */}
      <g
        stroke="currentColor"
        strokeWidth="4"
        strokeLinecap="round"
        strokeLinejoin="round"
        opacity="0.75"
      >
        <path d="M100 96 v44" />
        <path d="M86 126 l14 14 l14-14" />
      </g>
      <circle cx="42" cy="52" r="6" fill="currentColor" opacity="0.35" />
      <circle cx="164" cy="66" r="4" fill="currentColor" opacity="0.3" />
      <circle cx="150" cy="140" r="5" fill="currentColor" opacity="0.28" />
    </g>
  );
}
