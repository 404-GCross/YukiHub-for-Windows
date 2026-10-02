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
 * - 圆点可点选，点选后暂停一会儿自动播放（与首页游戏轮播同一手感：
 *   「点一次就再也不动」是 bug，不是特性）。
 * - 点击整块打开详情：标题 + 摘要 + 日期/署名 + 阅读原文。
 *   NextMoe 要求引用资讯必须标注来源，所以署名一定要显示。
 */
const NEWS_CAROUSEL_INTERVAL_MS = 5000;
const NEWS_MANUAL_RESUME_DELAY_MS = 15000;

interface HomeNewsCardProps {
  /**
   * 外部刷新信号：首页顶部的「刷新」按钮自增此值即可强制刷新资讯。
   *
   * 首次挂载传 0（不触发，避免与初始加载重复请求）。
   */
  refreshToken?: number;
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
      // 后端在有旧缓存时会回退旧数据，这里只在前端拿不到任何内容时才清空。
      if (nextItems.length > 0) {
        setItems(nextItems);
        setActiveIndex(0);
      }
      else if (items.length === 0) {
        setItems([]);
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
    // items.length 只用于「是否清空」的判定，不作为触发条件
    // eslint-disable-next-line react-hooks/exhaustive-deps
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

  // 自动轮播：悬停暂停、详情打开时暂停、手动点选后延时恢复。
  useEffect(() => {
    if (items.length <= 1 || isHovered || isManuallyPaused || detailItem) {
      return;
    }
    const timer = window.setInterval(() => {
      setActiveIndex(current => (current + 1) % items.length);
    }, NEWS_CAROUSEL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [items.length, isHovered, isManuallyPaused, detailItem]);

  useEffect(
    () => () => {
      if (resumeTimerRef.current !== null) {
        window.clearTimeout(resumeTimerRef.current);
      }
    },
    [],
  );

  const handleSelectIndex = (index: number) => {
    setActiveIndex(index);
    setIsManuallyPaused(true);
    if (resumeTimerRef.current !== null) {
      window.clearTimeout(resumeTimerRef.current);
    }
    resumeTimerRef.current = window.setTimeout(() => {
      setIsManuallyPaused(false);
    }, NEWS_MANUAL_RESUME_DELAY_MS);
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
        className="relative min-h-32 flex-1 overflow-hidden rounded-xl bg-brand-900/85"
        onMouseEnter={() => setIsHovered(true)}
        onMouseLeave={() => setIsHovered(false)}
      >
        {activeItem ? (
          <button
            type="button"
            onClick={() => setDetailItem(activeItem)}
            aria-label={t("home.news.openDetail")}
            className="absolute inset-0 block h-full w-full cursor-pointer text-left"
          >
            {activeItem.banner_url ? (
              <img
                src={proxiedImageSrc(activeItem.banner_url)}
                alt=""
                className="h-full w-full object-cover"
                onError={(event) => {
                  // 代理拿不到就直连原图（与聊天图片同一兜底策略）
                  const img = event.currentTarget;
                  if (
                    !img.dataset.fallbackRaw
                    && img.src !== activeItem.banner_url
                  ) {
                    img.dataset.fallbackRaw = "1";
                    img.src = activeItem.banner_url;
                  }
                }}
              />
            ) : null}

            {/* 遮罩：手机版是同款「从下到上的深色渐变」，保证标题在任何题图上可读 */}
            <div className="absolute inset-0 bg-gradient-to-t from-black/85 via-black/40 to-transparent" />

            <div className="absolute inset-x-0 bottom-0 flex flex-col gap-1.5 px-3 pb-2.5">
              <p className="line-clamp-2 text-sm font-bold leading-snug text-white drop-shadow-sm">
                {activeItem.title}
              </p>
              {items.length > 1 ? (
                <div className="flex items-center gap-1">
                  {items.map((item, index) => (
                    <span
                      key={item.id || item.title}
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
          </button>
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
