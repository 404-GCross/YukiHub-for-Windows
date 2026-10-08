import { useEffect, useState } from "react";

import { ProxyImage } from "../components/ui/ProxyImage";
import { BackgroundTrailerVideo } from "./BackgroundTrailerVideo";

interface BigScreenBackgroundProps {
  /** 当前焦点游戏的封面地址；为空时保留上一张 */
  coverUrl: string;
  /**
   * 元数据来源的原始封面地址（未压缩）。
   *
   * 本地封面最长边被压到 1600px（`image_covers_optimize.go`），铺满 4K 全屏会糊，
   * 所以这里再叠一层原图：本地封面先出，原图加载完成后淡入替换。
   */
  coverSourceUrl?: string;
  isNSFW: boolean;
  /** 当前焦点游戏的本地预告片地址；为空则只有封面层 */
  trailerUrl?: string;
  /** 焦点停留够久、且没有浮层挡住时，背景改播预告片 */
  backgroundTrailerActive?: boolean;
  /** 预告片显示方式：true = 原比例留黑边，false = 铺满裁切 */
  trailerFit?: boolean;
  /** 预告片是否静音 */
  trailerMuted?: boolean;
  /** 预告片遮罩不透明度（0–1） */
  trailerScrimOpacity?: number;
}

/**
 * 大屏双背景层，对齐手机端 `bsBgA/B`：两层封面互相交叉淡入（600ms），
 * 整体再叠加一次缓慢推进的 KenBurns，焦点停留够久时在其上叠一层静音预告片，
 * 最后盖上左右与底部渐变遮罩。
 */
export function BigScreenBackground({
  coverUrl,
  coverSourceUrl = "",
  isNSFW,
  trailerUrl,
  backgroundTrailerActive = false,
  trailerFit = false,
  trailerMuted = false,
  trailerScrimOpacity = 0,
}: BigScreenBackgroundProps) {
  const [slots, setSlots] = useState<{
    active: "a" | "b";
    a: string;
    b: string;
  }>({ active: "a", a: coverUrl, b: "" });

  useEffect(() => {
    // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect
    setSlots((previous) => {
      const front = previous.active === "a" ? previous.a : previous.b;
      if (front === coverUrl) {
        return previous;
      }
      return previous.active === "a"
        ? { active: "b", a: previous.a, b: coverUrl }
        : { active: "a", a: coverUrl, b: previous.b };
    });
  }, [coverUrl]);

  const activeUrl = slots.active === "a" ? slots.a : slots.b;
  const hiResUrl
    = coverSourceUrl && coverSourceUrl !== activeUrl && activeUrl
      ? coverSourceUrl
      : "";

  const layerClass = (slot: "a" | "b") =>
    `absolute inset-0 transition-opacity duration-[600ms] ease-out ${
      slots.active === slot ? "opacity-100" : "opacity-0"
    }`;

  return (
    <div
      className="pointer-events-none absolute inset-0 overflow-hidden bg-brand-900"
      aria-hidden="true"
    >
      <div className="absolute inset-0 animate-bigscreen-kenburns">
        <div className={layerClass("a")}>
          <BackgroundCover url={slots.a} isNSFW={isNSFW} />
        </div>
        <div className={layerClass("b")}>
          <BackgroundCover url={slots.b} isNSFW={isNSFW} />
        </div>
        {/* 高清层只在原图与底图不同且当前槽位有图时才叠 */}
        {hiResUrl && (
          <div className={layerClass(slots.active)}>
            <HiResBackgroundCover url={hiResUrl} isNSFW={isNSFW} />
          </div>
        )}
      </div>

      <BackgroundTrailerVideo
        active={backgroundTrailerActive}
        fit={trailerFit}
        muted={trailerMuted}
        scrimOpacity={trailerScrimOpacity}
        url={trailerUrl ?? ""}
      />

      {/* 左右渐变遮罩：左侧给分类栏留出可读底，右侧收暗让信息层浮起来 */}
      <div className="absolute inset-0 bg-gradient-to-r from-brand-900 via-brand-900/35 to-brand-900/85" />
      {/* 底部渐变遮罩：承载信息浮层与按钮排 */}
      <div className="absolute inset-0 bg-gradient-to-t from-brand-900 via-brand-900/72 to-transparent" />
    </div>
  );
}

function BackgroundCover({ url, isNSFW }: { url: string; isNSFW: boolean }) {
  if (!url) {
    return null;
  }

  return (
    <ProxyImage
      src={url}
      isNSFW={isNSFW}
      className="h-full w-full object-cover object-center"
      decoding="async"
    />
  );
}

/**
 * 高清封面层：用 `key={url}` 让它随焦点切换重新挂载，
 * 这样「是否已加载」的状态天然重置，不必在 effect 里改 state。
 */
function HiResBackgroundCover({
  url,
  isNSFW,
}: {
  url: string;
  isNSFW: boolean;
}) {
  const [loaded, setLoaded] = useState(false);

  return (
    <ProxyImage
      key={url}
      src={url}
      isNSFW={isNSFW}
      className={`h-full w-full object-cover object-center transition-opacity duration-[600ms] ease-out ${
        loaded ? "opacity-100" : "opacity-0"
      }`}
      decoding="async"
      onLoad={() => setLoaded(true)}
    />
  );
}
