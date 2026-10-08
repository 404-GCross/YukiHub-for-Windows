interface BackgroundTrailerVideoProps {
  /** 受管预告片地址，形如 /local/trailers/<gameID>.mp4 或 http(s) 直链 */
  url: string;
  /** 为 false 时直接卸载：滚动时不能让多张卡片同时持有 video 元素 */
  active: boolean;
  /** true = 原比例留黑边（fit），false = 铺满裁切（cover） */
  fit?: boolean;
  /** 是否静音，来自 `bigscreen_trailer_muted`（默认不静音） */
  muted?: boolean;
  /** 遮罩不透明度（0–1），来自 `bigscreen_pv_scrim_percent`；0 表示不叠遮罩 */
  scrimOpacity?: number;
}

/**
 * 货架焦点停留后在大屏背景里起播的预告片。
 *
 * 插在 KenBurns 封面容器之后、渐变遮罩之前，对齐手机端 `bsBgVideo`
 * 在氛围层之上、UI 之下的层级。播放中再叠一层可调的黑色遮罩
 * （手机端 `applyVideoScrim`），保证背景上的文字仍然读得清。
 */
export function BackgroundTrailerVideo({
  url,
  active,
  fit = false,
  muted = false,
  scrimOpacity = 0,
}: BackgroundTrailerVideoProps) {
  if (!active || !url) {
    return null;
  }

  return (
    <>
      <video
        src={url}
        autoPlay
        muted={muted}
        loop
        playsInline
        preload="auto"
        aria-hidden="true"
        className={`absolute inset-0 h-full w-full opacity-100 transition-opacity duration-700 ease-out ${
          fit ? "object-contain" : "object-cover"
        }`}
        onLoadedData={(event) => {
          // 自动播放策略可能拒绝 play()，静默忽略即可（继续显示封面）
          event.currentTarget.play().catch(() => {});
        }}
      />
      {scrimOpacity > 0 && (
        <div
          className="absolute inset-0 bg-black"
          style={{ opacity: scrimOpacity }}
          aria-hidden="true"
        />
      )}
    </>
  );
}
