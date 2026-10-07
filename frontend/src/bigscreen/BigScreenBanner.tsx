import { memo, useEffect, useState } from "react";

/** 一条提示：`id` 变化即视为新消息，动画会重播（对齐手机端 show() 重置停留） */
export interface BigScreenBannerMessage {
  icon?: string;
  id: number;
  text: string;
}

interface BigScreenBannerProps {
  /** 停留时长（毫秒），来自 `bigscreen_banner_hold_ms`，默认 2000 */
  holdMs?: number;
  message: BigScreenBannerMessage | null;
  /** 入场动画播放中：抑制一切提示（"手柄已连接"不该盖在启动动画上） */
  suppressed?: boolean;
}

/**
 * 大屏顶部提示条，对齐手机端 `BigScreenBanner`（spec §S7）。
 *
 * 用于「已收藏 / 已切换排序 / 已绑定 PV / 手柄已连接」这类**操作反馈**。
 * 手机端在 M14-3 两轮用户反馈后回退到最简：只要一次干净的淡入 + 轻微下移，
 * 不要回弹、不要脉冲、不要渐变花活 —— 这里保持一致。
 */
export const BigScreenBanner = memo(
  ({ holdMs = 2000, message, suppressed = false }: BigScreenBannerProps) => {
    const [visible, setVisible] = useState(false);

    useEffect(() => {
      if (!message || suppressed) {
        // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect
        setVisible(false);
        return;
      }
      // eslint-disable-next-line react-hooks-extra/no-direct-set-state-in-use-effect
      setVisible(true);
      const timer = window.setTimeout(() => setVisible(false), holdMs);
      return () => window.clearTimeout(timer);
    }, [holdMs, message, suppressed]);

    if (!message || suppressed) {
      return null;
    }

    return (
      <div className="pointer-events-none absolute inset-x-0 top-0 z-50 flex justify-center pt-3">
        {/*
          key 挂 message.id：换一条消息时整块重挂载，滑入动画才会重新播
          （否则连续提示只会"原地换字"，看不出反馈）。
        */}
        <div
          key={message.id}
          className={`inline-flex max-w-[70vw] items-center gap-2 rounded-full border border-white/12 bg-brand-900/88 px-4 py-2 text-sm text-white shadow-[0_8px_28px_rgba(0,0,0,0.45)] backdrop-blur-md transition-all duration-200 ease-out ${
            visible ? "translate-y-0 opacity-100" : "-translate-y-3 opacity-0"
          }`}
        >
          {message.icon && (
            <span
              className={`${message.icon} shrink-0 text-base text-primary-300`}
              aria-hidden="true"
            />
          )}
          <span className="truncate">{message.text}</span>
        </div>
      </div>
    );
  },
);
