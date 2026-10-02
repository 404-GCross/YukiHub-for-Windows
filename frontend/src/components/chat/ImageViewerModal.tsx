import { Browser } from "@wailsio/runtime";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";

interface ImageViewerModalProps {
  isOpen: boolean;
  /** 已经补全过的绝对地址 */
  url: string;
  onClose: () => void;
}

/**
 * 图片全屏查看（对齐手机版 showImageViewer）。
 *
 * 手机版支持长按保存到相册；桌面端等价能力是「用系统默认程序打开 / 另存」，
 * 所以这里提供「打开原图」按钮交给系统浏览器处理，点击背景或按 Esc 关闭。
 */
export function ImageViewerModal({
  isOpen,
  url,
  onClose,
}: ImageViewerModalProps) {
  const { t } = useTranslation();

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen || !url) {
    return null;
  }

  return (
    <div
      className="fixed inset-0 z-[90] flex flex-col items-center justify-center gap-4 bg-black/85 p-6"
      onClick={onClose}
    >
      <img
        src={url}
        alt=""
        className="max-h-[80vh] max-w-full rounded-xl object-contain"
        onClick={event => event.stopPropagation()}
      />
      <div
        className="flex items-center gap-2"
        onClick={event => event.stopPropagation()}
      >
        <button
          type="button"
          onClick={() => void Browser.OpenURL(url)}
          className="flex items-center gap-1.5 rounded-lg bg-white/15 px-3 py-2 text-sm text-white transition-colors hover:bg-white/25"
        >
          <span className="i-mdi-open-in-new text-base" />
          {t("friendsChat.openOriginal")}
        </button>
        <button
          type="button"
          onClick={onClose}
          className="flex items-center gap-1.5 rounded-lg bg-white/15 px-3 py-2 text-sm text-white transition-colors hover:bg-white/25"
        >
          <span className="i-mdi-close text-base" />
          {t("common.close")}
        </button>
      </div>
    </div>
  );
}
