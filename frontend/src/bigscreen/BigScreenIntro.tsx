import { useEffect, useState } from "react";

interface BigScreenIntroProps {
  /** 入场结束（或用户跳过）后交回主界面 */
  onFinished: () => void;
}

/** 内置入场动画的时间轴（毫秒），总长约 1.5s，对齐手机端 BigScreenIntro */
const LOGO_IN_AT = 80;
const BAND_AT = 300;
const FADE_OUT_AT = 1120;
const FINISH_AT = 1560;

/**
 * 大屏入场动画，对齐手机端 `BigScreenIntro` 的内置动画分支：
 * 深色底 → 中央 logo 淡入上浮 → 光带自左向右扫过 → 整层淡出交回主界面。
 *
 * 手机端支持用户自选入场视频；桌面端暂无视频选择入口，只保留内置动画，
 * 但**任意输入都能跳过**（由宿主在意图分发最前面拦截）。
 */
export function BigScreenIntro({ onFinished }: BigScreenIntroProps) {
  const [logoIn, setLogoIn] = useState(false);
  const [bandActive, setBandActive] = useState(false);
  const [fadingOut, setFadingOut] = useState(false);

  useEffect(() => {
    const timers = [
      window.setTimeout(() => setLogoIn(true), LOGO_IN_AT),
      window.setTimeout(() => setBandActive(true), BAND_AT),
      window.setTimeout(() => setFadingOut(true), FADE_OUT_AT),
      window.setTimeout(() => onFinished(), FINISH_AT),
    ];
    return () => timers.forEach(timer => window.clearTimeout(timer));
  }, [onFinished]);

  return (
    <div
      className={`absolute inset-0 z-60 flex items-center justify-center bg-brand-950 transition-opacity duration-[440ms] ease-out ${
        fadingOut ? "opacity-0" : "opacity-100"
      }`}
      aria-hidden="true"
    >
      <div className="relative flex flex-col items-center">
        <span
          className={`text-5xl font-black tracking-tight text-white transition-all duration-[520ms] ease-out ${
            logoIn ? "translate-y-0 opacity-100" : "translate-y-7 opacity-0"
          }`}
        >
          YukiHub
        </span>
        {/* 光带：自左向右扫过，扫完自行淡出 */}
        <span
          className={`pointer-events-none absolute left-1/2 top-1/2 h-24 w-[420px] -translate-x-1/2 -translate-y-1/2 bg-gradient-to-r from-transparent via-white/55 to-transparent blur-md transition-all duration-[780ms] ease-out ${
            bandActive ? "translate-x-[220px] opacity-0" : "opacity-0"
          }`}
          style={{ transitionProperty: "transform, opacity" }}
          aria-hidden="true"
        />
      </div>
    </div>
  );
}
