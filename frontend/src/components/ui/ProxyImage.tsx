import type { ImgHTMLAttributes } from "react";
import { useMemo, useState } from "react";
import { useAppStore } from "../../store";
import {
  imageSourceCandidates,
  loadableImageSources,
  markImageSourceFailed,
} from "../../utils/imageProxy";

type ProxyImageProps = Omit<ImgHTMLAttributes<HTMLImageElement>, "src"> & {
  src: string | null | undefined;
  fallbackSrc?: string | null | undefined;
  isNSFW?: boolean;
  revealNSFWOnHover?: boolean;
};

export function ProxyImage({
  src,
  fallbackSrc,
  referrerPolicy = "no-referrer",
  draggable = false,
  onDragStart,
  onError,
  className,
  isNSFW = false,
  revealNSFWOnHover = false,
  ...props
}: ProxyImageProps) {
  const shouldBlurNSFW = useAppStore(
    state => state.config?.blur_nsfw_game_covers !== false,
  );
  const rawSrc = src?.trim() ?? "";
  const rawFallbackSrc = fallbackSrc?.trim() ?? "";
  const candidates = useMemo(
    () => imageSourceCandidates(rawSrc, rawFallbackSrc),
    [rawFallbackSrc, rawSrc],
  );
  /*
    剔除最近失败过的候选：切片页面会重新挂载 <img>，没有这层记忆就会把同一串
    超时再跑一遍（封面每次切页都「重新加载」的来源）。全部候选都失败过时不再
    发请求，直接交给上层渲染占位。
  */
  const activeCandidates = useMemo(
    () => loadableImageSources(candidates),
    [candidates],
  );
  const candidateSignature = activeCandidates.join("\0");
  const [failureState, setFailureState] = useState({
    signature: candidateSignature,
    index: 0,
  });
  const failureIndex
    = failureState.signature === candidateSignature ? failureState.index : 0;
  const resolvedSrc = activeCandidates[failureIndex] ?? "";

  if (activeCandidates.length === 0) {
    return null;
  }

  return (
    <img
      {...props}
      src={resolvedSrc}
      className={`${className ?? ""} ${
        isNSFW && shouldBlurNSFW
          ? `nsfw-cover-blur ${
            revealNSFWOnHover
              ? "hover:nsfw-cover-reveal transition-[filter] duration-300"
              : ""
          }`
          : ""
      }`.trim()}
      referrerPolicy={referrerPolicy}
      draggable={draggable}
      onError={(event) => {
        markImageSourceFailed(resolvedSrc);
        if (failureIndex + 1 < activeCandidates.length) {
          setFailureState({
            signature: candidateSignature,
            index: failureIndex + 1,
          });
          return;
        }
        onError?.(event);
      }}
      onDragStart={onDragStart ?? (event => event.preventDefault())}
    />
  );
}
