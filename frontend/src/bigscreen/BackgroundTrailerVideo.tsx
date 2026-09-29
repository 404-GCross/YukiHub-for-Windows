interface BackgroundTrailerVideoProps {
  /** 受管预告片地址，形如 /local/trailers/<gameID>.mp4 */
  url: string;
  /** 为 false 时直接卸载：滚动时不能让多张卡片同时持有 video 元素 */
  active: boolean;
}

/**
 * 货架焦点停留后在大屏背景里静音起播的预告片。
 *
 * 插在 KenBurns 封面容器之后、渐变遮罩之前，对齐手机端 `bsBgVideo`
 * 在氛围层之上、UI 之下的层级。
 */
export function BackgroundTrailerVideo({
  url,
  active,
}: BackgroundTrailerVideoProps) {
  if (!active || !url) {
    return null;
  }

  return (
    <video
      src={url}
      autoPlay
      muted
      loop
      playsInline
      preload="auto"
      aria-hidden="true"
      className="absolute inset-0 h-full w-full object-cover opacity-100 transition-opacity duration-700 ease-out"
      onLoadedData={(event) => {
        // 自动播放策略可能拒绝 play()，静默忽略即可（继续显示封面）
        event.currentTarget.play().catch(() => {});
      }}
    />
  );
}
