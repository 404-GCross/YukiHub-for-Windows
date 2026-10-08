import { useEffect, useRef } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";

interface BigScreenTrailerPlayerProps {
  /** 受管预告片地址，形如 /local/trailers/<gameID>.mp4 或 http(s) 直链 */
  url: string;
  title: string;
  /** true = 原比例留黑边（fit），false = 铺满裁切 */
  fit?: boolean;
  /** 是否静音，来自 `bigscreen_trailer_muted` */
  muted?: boolean;
  onClose: () => void;
}

/**
 * 大屏全屏预告片播放器。
 *
 * 键盘与手柄统一走 `bigscreen.tsx` 的 window 层分发（任意键返回），组件内不监听按键；
 * 播完或解码失败都会自动关闭，把控制权交回原来的层级。
 */
export function BigScreenTrailerPlayer({
  url,
  title,
  fit = false,
  muted = false,
  onClose,
}: BigScreenTrailerPlayerProps) {
  const { t } = useTranslation();
  const videoRef = useRef<HTMLVideoElement>(null);

  // 自动播放策略可能拒绝 play()，此时保留首帧画面由用户自行拖进度条
  useEffect(() => {
    videoRef.current?.play().catch(() => {});
  }, []);

  return (
    <div className="absolute inset-0 z-40 flex items-center justify-center bg-black">
      <video
        ref={videoRef}
        src={url}
        aria-label={title}
        autoPlay
        muted={muted}
        controls={false}
        className={`h-full w-full ${fit ? "object-contain" : "object-cover"}`}
        onEnded={onClose}
        onError={() => {
          toast.error(t("bigScreen.trailerPlayFailed"));
          onClose();
        }}
      />
    </div>
  );
}
