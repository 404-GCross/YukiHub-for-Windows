import type { ComponentProps } from "react";
import { useState } from "react";
import { useAppStore } from "../../store";
import { isImageSourceFailed } from "../../utils/imageProxy";
import { ProxyImage } from "./ProxyImage";

type GameCoverImageProps = Omit<
  ComponentProps<typeof ProxyImage>,
  "className" | "isNSFW" | "revealNSFWOnHover"
> & {
  className?: string;
  imageClassName?: string;
  isNSFW?: boolean;
  revealNSFWOnHover?: boolean;
};

export function GameCoverImage({
  className,
  imageClassName,
  isNSFW = false,
  revealNSFWOnHover = false,
  onError,
  ...props
}: GameCoverImageProps) {
  const shouldProtectNSFWCover = useAppStore(
    state => isNSFW && state.config?.blur_nsfw_game_covers !== false,
  );
  const shouldShowWatermark = shouldProtectNSFWCover && revealNSFWOnHover;
  // 记录失败的地址而不是布尔量：换图后自动失效，不必在 effect 里重置
  const [failedSrc, setFailedSrc] = useState("");
  const currentSrc = props.src?.trim() ?? "";
  // 本组件本次会话里失败过、或这一轮 ProxyImage 因失败记忆直接放弃请求时，
  // 都直接出占位（后者让重新挂载的卡片立刻有内容，不必再等一次网络超时）。
  const isFailed
    = currentSrc !== ""
      && (failedSrc === currentSrc || isImageSourceFailed(currentSrc));

  return (
    <span
      className={`relative block overflow-hidden ${className ?? ""}`.trim()}
    >
      <ProxyImage
        {...props}
        isNSFW={isNSFW}
        revealNSFWOnHover={revealNSFWOnHover}
        className={`${imageClassName ?? "h-full w-full object-cover"} ${
          shouldShowWatermark ? "peer" : ""
        } ${isFailed ? "opacity-0" : ""}`.trim()}
        onError={(event) => {
          setFailedSrc(currentSrc);
          onError?.(event);
        }}
      />
      {/* 加载失败时盖掉浏览器自带的「碎图」图标，改用应用内占位 */}
      {isFailed && (
        <span
          className="absolute inset-0 flex items-center justify-center bg-brand-200 text-brand-400 dark:bg-brand-900/60"
          aria-hidden="true"
        >
          <span className="i-mdi-image-off text-3xl" />
        </span>
      )}
      {shouldShowWatermark && (
        <span className="pointer-events-none absolute inset-0 flex items-center justify-center transition-opacity duration-300 peer-hover:opacity-0">
          <span
            aria-hidden="true"
            className="i-mdi-eye-outline size-24 text-white opacity-[0.22] drop-shadow-[0_2px_10px_rgba(0,0,0,0.45)]"
          />
        </span>
      )}
    </span>
  );
}
