import type { CSSProperties } from "react";
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from "react";
import { useTranslation } from "react-i18next";

/**
 * 大屏入场动画，逐帧对齐手机端 `BigScreenIntro`（spec §S1）。
 *
 * 手机端时间轴（毫秒，`BigScreenIntro.java`）：
 * ```
 *   80   logo 淡入 + 上浮 28dp        （520ms，Decelerate）
 *  300   光带自左向右扫过             （780ms，AccelerateDecelerate，峰值 alpha .55）
 * 980   logo 上浮淡出 -14dp          （240ms）
 * 1120   入场层淡出 260ms  +  主界面圆形揭示（0 → 全屏半径，560ms）+ 1.06 → 1.0
 * 1680   交回主界面
 * ```
 * 之前桌面版只做了「logo 淡入 + 光带」，logo 淡出、圆形揭示、回缩全都没有，
 * 所以看起来"一闪就过去了"。
 *
 * 三档行为：
 * - `rich`（默认）：完整时间轴，任意输入可跳过（跳过时只做一次 240ms 淡出）；
 * - 低性能档（effect_level = off）：不做动画，120ms 后直接交回主界面；
 * - 自选入场视频：铺满播放，播完/坏掉/跳过都进主界面（视频坏掉回退内置动画）。
 */

/** 手机端时间轴常量（毫秒） */
const T_LOGO_IN = 80;
const D_LOGO_IN = 520;
const T_BAND = 300;
const D_BAND = 780;
const T_LOGO_OUT = 980;
const D_LOGO_OUT = 240;
const T_REVEAL = 1120;
const D_REVEAL = 560;
/** 入场层自身的淡出时长（手机端 260ms） */
const D_FADE_OUT = 260;
/** 低性能档：直通延迟 */
const D_LOW_END = 120;

export interface BigScreenIntroHandle {
  /** 任意输入跳过：立刻进入结束流程（不再是"直接卸载组件"） */
  skip: () => void;
}

interface BigScreenIntroProps {
  /** 入场结束（主界面已完全可见） */
  onFinished: () => void;
  /** 进入"主界面揭示"时刻，宿主给内容层挂上揭示动画 */
  onRevealStart?: () => void;
  /** false = 低性能档，不做动画 */
  richEffects?: boolean;
  /** 自选入场视频（/local/... 地址）；空串 = 内置动画 */
  videoUrl?: string;
}

export const BigScreenIntro = forwardRef<
  BigScreenIntroHandle,
  BigScreenIntroProps
>(({ onFinished, onRevealStart, richEffects = true, videoUrl = "" }, ref) => {
  const { t } = useTranslation();
  const [logoIn, setLogoIn] = useState(false);
  const [logoOut, setLogoOut] = useState(false);
  const [bandActive, setBandActive] = useState(false);
  const [bandFaded, setBandFaded] = useState(false);
  const [fadingOut, setFadingOut] = useState(false);
  /** 视频起不来（文件被删/编码不支持）→ 回退内置动画，且只回退一次 */
  const [videoFailed, setVideoFailed] = useState(false);
  const [videoDone, setVideoDone] = useState(false);
  /** 已经交回主界面（避免 skip 与自然结束重复触发） */
  const finishedRef = useRef(false);
  const onFinishedRef = useRef(onFinished);
  const onRevealStartRef = useRef(onRevealStart);

  useEffect(() => {
    onFinishedRef.current = onFinished;
    onRevealStartRef.current = onRevealStart;
  }, [onFinished, onRevealStart]);

  const finish = () => {
    if (finishedRef.current) {
      return;
    }
    finishedRef.current = true;
    onFinishedRef.current();
  };

  useImperativeHandle(ref, () => ({
    skip: () => {
      if (finishedRef.current) {
        return;
      }
      setFadingOut(true);
      setVideoDone(true);
      window.setTimeout(finish, richEffects ? D_FADE_OUT : 0);
    },
  }));

  const useVideo = richEffects && videoUrl !== "" && !videoFailed;

  // ===== 内置动画时间轴（每个定时器单独清理，便于静态检查确认不漏清） =====
  useEffect(() => {
    if (!richEffects || useVideo) {
      return;
    }
    const logoInTimer = window.setTimeout(() => setLogoIn(true), T_LOGO_IN);
    const bandTimer = window.setTimeout(() => setBandActive(true), T_BAND);
    const bandFadeTimer = window.setTimeout(
      () => setBandFaded(true),
      T_BAND + 340,
    );
    const logoOutTimer = window.setTimeout(() => setLogoOut(true), T_LOGO_OUT);
    const revealTimer = window.setTimeout(() => {
      onRevealStartRef.current?.();
      setFadingOut(true);
    }, T_REVEAL);
    const finishTimer = window.setTimeout(finish, T_REVEAL + D_REVEAL);
    return () => {
      window.clearTimeout(logoInTimer);
      window.clearTimeout(bandTimer);
      window.clearTimeout(bandFadeTimer);
      window.clearTimeout(logoOutTimer);
      window.clearTimeout(revealTimer);
      window.clearTimeout(finishTimer);
    };
  }, [richEffects, useVideo]);

  // 低性能档：手机端是「不做动画，120ms 后直接到位」
  useEffect(() => {
    if (richEffects) {
      return;
    }
    const timer = window.setTimeout(finish, D_LOW_END);
    return () => window.clearTimeout(timer);
  }, [richEffects]);

  // 视频播完 → 走一次淡出再交回主界面（setFadingOut 在 onEnded 里设，不在这里 setState）
  useEffect(() => {
    if (!videoDone || !useVideo || finishedRef.current) {
      return;
    }
    const timer = window.setTimeout(finish, D_FADE_OUT);
    return () => window.clearTimeout(timer);
  }, [videoDone, useVideo]);

  /** 与手机端一致：logo 是 46sp，副标题 13sp、字距 0.30em、焦点色 */
  const logoStyle: CSSProperties = {
    opacity: logoOut ? 0 : logoIn ? 1 : 0,
    transform: `translateY(${logoOut ? "-14px" : logoIn ? "0" : "28px"})`,
    transition: logoOut
      ? `opacity ${D_LOGO_OUT}ms ease-out, transform ${D_LOGO_OUT}ms ease-out`
      : `opacity ${D_LOGO_IN}ms ease-out, transform ${D_LOGO_IN}ms ease-out`,
  };

  const bandStyle: CSSProperties = {
    opacity: bandActive ? (bandFaded ? 0 : 0.55) : 0,
    transform: `translate3d(${bandActive ? "280px" : "-220px"}, 0, 0)`,
    transition: `transform ${D_BAND}ms cubic-bezier(.4,0,.2,1), opacity ${
      bandFaded ? 380 : 160
    }ms ease-out`,
    // 手机端那条约 420×180 的光带只有**横向**渐变，上下两条边是硬边：
    // 静止截图里会看到"文字后面有个灰盒子"。再叠一层纵向蒙版把上下也淡掉。
    maskImage:
      "linear-gradient(to bottom, transparent 0%, black 38%, black 62%, transparent 100%)",
    WebkitMaskImage:
      "linear-gradient(to bottom, transparent 0%, black 38%, black 62%, transparent 100%)",
  };

  return (
    <div
      // 底色用 brand-900（= 手机端 bs_bg #0B1020，也是应用根背景）：
      // 与主界面同色，圆形揭示时才像"内容从同一片底色里长出来"。
      // 这里必须是**不透明**的 —— 之前写的是色板里不存在的 brand-950，
      // UnoCSS 直接不生成背景色，导致入场层透明、背后的游戏列表一览无余。
      className="absolute inset-0 z-60 flex items-center justify-center bg-brand-900"
      style={{
        opacity: fadingOut ? 0 : 1,
        transition: `opacity ${D_FADE_OUT}ms ease-out`,
      }}
      aria-hidden="true"
    >
      {useVideo && (
        <video
          className="absolute inset-0 h-full w-full object-cover"
          src={videoUrl}
          autoPlay
          muted
          playsInline
          onEnded={() => {
            setVideoDone(true);
            setFadingOut(true);
          }}
          // 视频坏了不能把用户卡在黑屏：回退内置动画（只回退一次，防死循环）
          onError={() => setVideoFailed(true)}
        />
      )}

      {!useVideo && (
        <div className="relative flex flex-col items-center">
          {/* 光带（手机端 420×180dp，自 x=-220 扫到 +280，峰值 alpha 0.55） */}
          <span
            className="pointer-events-none absolute left-1/2 top-1/2 -ml-[210px] -mt-[90px] h-[180px] w-[420px] bg-gradient-to-r from-transparent via-[#FFE3F1]/85 to-transparent blur-[2px]"
            style={bandStyle}
          />
          <div className="flex flex-col items-center" style={logoStyle}>
            <span
              className="text-[clamp(2.1rem,4.5vh,3.1rem)] font-black leading-none text-[#F5F7FF]"
              style={{ letterSpacing: "0.22em" }}
            >
              YukiHub
            </span>
            <span
              className="mt-2.5 text-[clamp(0.75rem,1.45vh,1rem)] font-semibold text-secondary-500"
              style={{ letterSpacing: "0.30em" }}
            >
              {t("bigScreen.introSubtitle")}
            </span>
          </div>
        </div>
      )}
    </div>
  );
});

BigScreenIntro.displayName = "BigScreenIntro";
