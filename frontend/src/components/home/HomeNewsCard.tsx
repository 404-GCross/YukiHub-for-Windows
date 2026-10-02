import type {
  MouseEvent as ReactMouseEvent,
  PointerEvent as ReactPointerEvent,
} from "react";
import type { vo } from "../../bindings/models";
import { Browser } from "@wailsio/runtime";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { GetGalgameNews } from "../../../bindings/yukihub/internal/service/homeservice";
import { proxiedImageSrc } from "../../utils/imageProxy";
import { ModalPortal } from "../ui/ModalPortal";

/**
 * 首页「Galgame 资讯」卡，与手机版 HomeActivity 的资讯轮播同源同行为：
 *
 * - 数据来自 NextMoe `/v2/news`（后端代理 + 2 小时磁盘缓存，见
 *   `internal/service/home_news_service.go`）；冷启动命中新鲜缓存不会发请求。
 * - 6 条自动轮播，间隔 5 秒（手机版 NEWS_CAROUSEL_INTERVAL_MS）。
 * - 圆点可点选、卡片可左右拖动切换（与上方游戏轮播同一套手势）。
 * - 点击打开详情：标题 + 摘要 + 日期/署名 + 阅读原文。
 *   NextMoe 要求引用资讯必须标注来源，所以署名一定要显示。
 *
 * 题图**每条各自渲染成一个元素、层叠后只切透明度**。
 * 上一版是同一个 `<img>` 复用、靠改 src 切图，而「加载失败回退直连」的标记
 * 挂在 DOM 上永不重置 —— 第一张回退成功后，切到第二张时不再回退，切回第一张
 * 时代理又失败而标记还在，表现就是「刚刚加载出来的图又没了」。
 * 每条独立元素后，每张图持有自己的加载/失败状态互不影响，切回时命中浏览器
 * 缓存可瞬时显示（手机版同样是每张图各自持有 bitmap）。
 */
const NEWS_CAROUSEL_INTERVAL_MS = 5000;
const NEWS_MANUAL_RESUME_DELAY_MS = 15000;

/**
 * 横向拖动多少像素算一次「滑动切换」（与 HomeHeroCard 一致）。
 *
 * 用位移阈值而不是「按下/抬起」判定：卡片里有标题、圆点，直接按 pointerup
 * 当翻页会把普通点击也吃掉。
 */
const SWIPE_THRESHOLD_PX = 48;

interface HomeNewsCardProps {
  /**
   * 外部刷新信号：首页顶部的「刷新」按钮自增此值即可强制刷新资讯。
   *
   * 首次挂载传 0（不触发，避免与初始加载重复请求）。
   */
  refreshToken?: number;
}

/**
 * 单条资讯的题图。
 *
 * 代理失败时回退直连，且**只退一次**（靠「当前 src 是否已是原图」判断，
 * 而不是在 DOM 上打一个永不重置的标记）。两种都失败就保持透明、露出卡片
 * 底色 —— 比渲染浏览器的碎图图标体面。
 */
function NewsBannerImage({ url }: { url: string }) {
  const [src, setSrc] = useState(() => (url ? proxiedImageSrc(url) : ""));
  const [isLoaded, setIsLoaded] = useState(false);

  // 题图变化时靠父级的 key（id + url）重新挂载本组件，state 自然重置，
  // 不需要在 effect 里再 set 一次。

  if (!url) {
    return null;
  }

  return (
    <img
      src={src}
      alt=""
      draggable={false}
      className={`h-full w-full object-cover transition-opacity duration-500 ${
        isLoaded ? "opacity-100" : "opacity-0"
      }`}
      onLoad={() => setIsLoaded(true)}
      onError={() => {
        if (src !== url) {
          setSrc(url);
        }
      }}
    />
  );
}

export function HomeNewsCard({ refreshToken = 0 }: HomeNewsCardProps) {
  const { t } = useTranslation();
  const [items, setItems] = useState<vo.HomeNewsItem[]>([]);
  const [activeIndex, setActiveIndex] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isTimedOut, setIsTimedOut] = useState(false);
  const [isHovered, setIsHovered] = useState(false);
  const [isManuallyPaused, setIsManuallyPaused] = useState(false);
  const [detailItem, setDetailItem] = useState<vo.HomeNewsItem | null>(null);
  const resumeTimerRef = useRef<number | null>(null);

  // 拖动切换手势（与 HomeHeroCard 同一套实现）
  const dragStartXRef = useRef<number | null>(null);
  const dragTriggeredRef = useRef(false);
  const suppressClickRef = useRef(false);

  const loadNews = useCallback(async (force: boolean) => {
    if (force) {
      setIsRefreshing(true);
    }
    else {
      setIsLoading(true);
    }
    try {
      const result = await GetGalgameNews(force);
      const nextItems = result?.items ?? [];
      if (nextItems.length > 0) {
        setItems(nextItems);
        setActiveIndex(0);
      }
      setIsTimedOut(Boolean(result?.timed_out));
    }
    catch (error) {
      // 保留已有内容（和手机版一样不打断用户）
      console.error("Failed to load galgame news:", error);
    }
    finally {
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }, []);

  useEffect(() => {
    void loadNews(false);
  }, [loadNews]);

  // 首页顶部的整体刷新：外部信号变化时强制联网刷新资讯（首次挂载不触发）。
  useEffect(() => {
    if (refreshToken <= 0) {
      return;
    }
    void loadNews(true);
  }, [refreshToken, loadNews]);

  useEffect(
    () => () => {
      if (resumeTimerRef.current !== null) {
        window.clearTimeout(resumeTimerRef.current);
      }
    },
    [],
  );

  /** 手动操作后暂停一会儿自动播放，避免「刚滑走又被自动切回去」。 */
  const pauseCarouselBriefly = useCallback(() => {
    setIsManuallyPaused(true);
    if (resumeTimerRef.current !== null) {
      window.clearTimeout(resumeTimerRef.current);
    }
    resumeTimerRef.current = window.setTimeout(() => {
      setIsManuallyPaused(false);
    }, NEWS_MANUAL_RESUME_DELAY_MS);
  }, []);

  /** 切到相对位置的条目（±1）。 */
  const shiftIndex = useCallback(
    (step: number) => {
      if (items.length <= 1) {
        return;
      }
      setActiveIndex(
        current => (current + step + items.length) % items.length,
      );
      pauseCarouselBriefly();
    },
    [items.length, pauseCarouselBriefly],
  );

  const handleSelectIndex = (index: number) => {
    setActiveIndex(index);
    pauseCarouselBriefly();
  };

  // 自动轮播：悬停暂停、详情打开时暂停、手动操作后延时恢复。
  useEffect(() => {
    if (items.length <= 1 || isHovered || isManuallyPaused || detailItem) {
      return;
    }
    const timer = window.setInterval(() => {
      setActiveIndex(current => (current + 1) % items.length);
    }, NEWS_CAROUSEL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [items.length, isHovered, isManuallyPaused, detailItem]);

  const handlePointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    // 只认左键/单指，别把右键菜单、多指手势算成滑动
    if (event.button !== 0) {
      return;
    }
    dragStartXRef.current = event.clientX;
    dragTriggeredRef.current = false;
    suppressClickRef.current = false;
  };

  const handlePointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    const startX = dragStartXRef.current;
    if (startX === null || dragTriggeredRef.current) {
      return;
    }
    const delta = event.clientX - startX;
    if (Math.abs(delta) < SWIPE_THRESHOLD_PX) {
      return;
    }
    // 一次手势只切一次：越过阈值立刻切走，之后继续拖不再重复触发
    dragTriggeredRef.current = true;
    shiftIndex(delta < 0 ? 1 : -1);
  };

  const handlePointerEnd = () => {
    if (dragTriggeredRef.current) {
      // 本次是滑动：吞掉紧随其后的 click，否则松开鼠标会顺手打开详情
      suppressClickRef.current = true;
    }
    dragStartXRef.current = null;
    dragTriggeredRef.current = false;
  };

  const handleClickCapture = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (!suppressClickRef.current) {
      return;
    }
    suppressClickRef.current = false;
    event.preventDefault();
    event.stopPropagation();
  };

  const handleOpenOriginal = (url: string) => {
    void Browser.OpenURL(url).catch(() => {
      console.error("Failed to open news source url");
    });
  };

  const activeItem = items[activeIndex] ?? items[0];
  const publishedDate = (activeItem?.published_at ?? "").slice(0, 10);

  return (
    <section className="yh-glass flex min-w-0 flex-1 flex-col gap-3 p-4">
      <div className="flex items-center gap-2">
        <span className="i-mdi-newspaper-variant-outline text-base text-brand-700 dark:text-white/85" />
        <h3 className="min-w-0 flex-1 truncate text-sm font-bold text-brand-900 dark:text-white">
          {t("home.news.title")}
        </h3>
        <button
          type="button"
          onClick={() => void loadNews(true)}
          disabled={isRefreshing}
          aria-label={t("home.news.refresh")}
          className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-brand-700 transition-colors hover:bg-white/50 disabled:cursor-wait dark:text-white/80 dark:hover:bg-white/10"
        >
          <span
            className={`i-mdi-refresh text-base ${isRefreshing ? "animate-spin" : ""}`}
            aria-hidden="true"
          />
        </button>
      </div>

      <div
        role="button"
        tabIndex={0}
        aria-label={t("home.news.openDetail")}
        className="relative min-h-32 flex-1 cursor-pointer select-none overflow-hidden rounded-xl bg-brand-900/85"
        style={{ touchAction: "pan-y" }}
        onClick={() => {
          if (activeItem) {
            setDetailItem(activeItem);
          }
        }}
        onClickCapture={handleClickCapture}
        onKeyDown={(event) => {
          if (event.key === "Enter" && activeItem) {
            setDetailItem(activeItem);
          }
        }}
        onMouseEnter={() => setIsHovered(true)}
        onMouseLeave={() => {
          setIsHovered(false);
          // 指针没经过 pointerup 就离开（拖到卡片外）时，也要结束本次手势
          handlePointerEnd();
        }}
        onPointerCancel={handlePointerEnd}
        onPointerDown={handlePointerDown}
        onPointerLeave={handlePointerEnd}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerEnd}
      >
        {/* 题图：每条一个元素，层叠后只切透明度（避免复用一个 img 互相污染） */}
        {items.map((item, index) => (
          <div
            key={`${item.id || item.title}-${item.banner_url}`}
            className={`absolute inset-0 transition-opacity duration-500 ease-out ${
              index === activeIndex ? "opacity-100" : "opacity-0"
            }`}
          >
            <NewsBannerImage url={item.banner_url} />
          </div>
        ))}

        {activeItem ? (
          <>
            {/* 遮罩：手机版是同款「从下到上的深色渐变」，保证标题在任何题图上可读 */}
            <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/85 via-black/40 to-transparent" />

            <div className="pointer-events-none absolute inset-x-0 bottom-0 flex flex-col gap-1.5 px-3 pb-2.5">
              <p className="line-clamp-2 text-sm font-bold leading-snug text-white drop-shadow-sm">
                {activeItem.title}
              </p>
              {items.length > 1 ? (
                <div className="pointer-events-auto flex items-center gap-1">
                  {items.map((item, index) => (
                    <span
                      key={item.id || `dot-${index}`}
                      role="button"
                      tabIndex={-1}
                      aria-hidden="true"
                      onClick={(event) => {
                        event.stopPropagation();
                        handleSelectIndex(index);
                      }}
                      className={`h-1 rounded-full transition-all ${
                        index === activeIndex
                          ? "w-3 bg-white"
                          : "w-1.5 bg-white/55 hover:bg-white/80"
                      }`}
                    />
                  ))}
                </div>
              ) : null}
            </div>
          </>
        ) : (
          <div className="flex h-full w-full items-center justify-center px-4 text-center text-xs text-white/85">
            {isLoading ? (
              <span className="flex items-center gap-2">
                <span className="i-mdi-loading animate-spin text-base" />
                {t("home.news.loading")}
              </span>
            ) : isTimedOut ? (
              t("home.news.loadSlow")
            ) : (
              t("home.news.loadFailed")
            )}
          </div>
        )}
      </div>

      {detailItem ? (
        <ModalPortal>
          <div
            className="fixed inset-0 z-[70] flex items-center justify-center bg-black/45 p-4 backdrop-blur-sm"
            onClick={() => setDetailItem(null)}
          >
            <div
              role="dialog"
              aria-modal="true"
              className="flex max-h-[80vh] w-full max-w-lg flex-col overflow-hidden rounded-2xl border border-brand-200/90 bg-white shadow-2xl dark:border-brand-700/90 dark:bg-brand-800"
              onClick={event => event.stopPropagation()}
            >
              <div className="flex items-start justify-between gap-3 border-b border-brand-200/80 px-4 py-3 dark:border-brand-700/80">
                <h2 className="min-w-0 flex-1 text-sm font-bold leading-snug text-brand-900 dark:text-white">
                  {detailItem.title}
                </h2>
                <button
                  type="button"
                  onClick={() => setDetailItem(null)}
                  aria-label={t("common.close")}
                  className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-brand-500 transition-colors hover:bg-brand-100 dark:text-brand-300 dark:hover:bg-brand-700/70"
                >
                  <span className="i-mdi-close text-base" aria-hidden="true" />
                </button>
              </div>

              <div className="min-h-0 flex-1 overflow-y-auto scrollbar-thin px-4 py-3">
                <p className="whitespace-pre-line text-sm leading-relaxed text-brand-800 dark:text-brand-100">
                  {detailItem.summary || t("home.news.noSummary")}
                </p>
                <div className="mt-4 flex flex-col gap-0.5 text-[11px] text-brand-500 dark:text-brand-400">
                  {publishedDate ? (
                    <span>
                      {t("home.news.publishedAt", { date: publishedDate })}
                    </span>
                  ) : null}
                  <span>
                    {detailItem.attribution || t("home.news.viaNextMoe")}
                  </span>
                </div>
              </div>

              <div className="flex items-center justify-end gap-2 border-t border-brand-200/80 px-4 py-3 dark:border-brand-700/80">
                {detailItem.source_url ? (
                  <button
                    type="button"
                    onClick={() => handleOpenOriginal(detailItem.source_url)}
                    className="rounded-lg bg-primary-500 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-primary-600"
                  >
                    {t("home.news.readOriginal")}
                  </button>
                ) : null}
                <button
                  type="button"
                  onClick={() => setDetailItem(null)}
                  className="rounded-lg bg-brand-100 px-3 py-1.5 text-xs font-medium text-brand-700 transition-colors hover:bg-brand-200 dark:bg-brand-700 dark:text-brand-100 dark:hover:bg-brand-600"
                >
                  {t("common.close")}
                </button>
              </div>
            </div>
          </div>
        </ModalPortal>
      ) : null}
    </section>
  );
}
