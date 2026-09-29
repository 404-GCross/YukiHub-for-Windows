import { useEffect, useState } from "react";

import { ProxyImage } from "../components/ui/ProxyImage";
import { BackgroundTrailerVideo } from "./BackgroundTrailerVideo";

interface BigScreenBackgroundProps {
  /** 当前焦点游戏的封面地址；为空时保留上一张 */
  coverUrl: string;
  isNSFW: boolean;
  /** 当前焦点游戏的本地预告片地址；为空则只有封面层 */
  trailerUrl?: string;
  /** 焦点停留够久、且没有浮层挡住时，背景改播预告片 */
  backgroundTrailerActive?: boolean;
}

/**
 * 大屏双背景层，对齐手机端 `bsBgA/B`：两层封面互相交叉淡入（600ms），
 * 整体再叠加一次缓慢推进的 KenBurns，焦点停留够久时在其上叠一层静音预告片，
 * 最后盖上左右与底部渐变遮罩。
 */
export function BigScreenBackground({
  coverUrl,
  isNSFW,
  trailerUrl,
  backgroundTrailerActive = false,
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
      </div>

      <BackgroundTrailerVideo
        url={trailerUrl ?? ""}
        active={backgroundTrailerActive}
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
