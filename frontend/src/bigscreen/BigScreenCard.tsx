import type { models } from "../../src/bindings/models";
import { memo } from "react";
import { useTranslation } from "react-i18next";

import { ProxyImage } from "../components/ui/ProxyImage";

interface BigScreenCardProps {
  /** 封面高度（px）；宽度由外层容器给 */
  coverHeight: number;
  focused: boolean;
  /** 焦点缩放幅度（%），0 表示只描边不缩放 */
  focusScale: number;
  game: models.Game;
  isFavorite: boolean;
  onActivate: () => void;
  /** 长按 / 右键 = 打开详情层（对齐手机端长按手势） */
  onDetails: () => void;
  /** 鼠标悬停即把焦点带过来，与方向键共用同一套焦点状态 */
  onFocus: () => void;
  showTitle: boolean;
}

/** 状态点颜色，对齐手机端 `BigScreenShelfAdapter` 的 bs_success / bs_primary */
const STATUS_DOT_CLASS: Record<string, string> = {
  completed: "bg-sky-400",
  playing: "bg-emerald-400",
};

/**
 * 大屏专用卡片，对齐手机端 `BigScreenShelfAdapter` 的卡片构成。
 *
 * 刻意**不复用** `GameCard`：游戏库那张卡带状态文字徽标、评分芯片、排序字段覆盖条与
 * 悬浮位移，是给鼠标精读用的；大屏卡片只需要"封面 + 可选标题 + 角标 + 未聚焦压暗"，
 * 信息密度低才能在沙发上隔着几米看清。
 *
 * 按压启动按钮（手机端 M8 的做法）不再出现在封面上，启动统一走 Ⓐ / 点击。
 */
export const BigScreenCard = memo(
  ({
    coverHeight,
    focused,
    focusScale,
    game,
    isFavorite,
    onActivate,
    onDetails,
    onFocus,
    showTitle,
  }: BigScreenCardProps) => {
    const { t } = useTranslation();
    // 手机端把焦点缩放收敛到 1.045，再乘用户档位（0 = 只描边）
    const scale = focused ? 1 + (1.045 - 1) * (focusScale / 100) : 1;
    const hasCover = Boolean(game.cover_url || game.cover_source_url);

    return (
      <div
        className="group relative w-full transition-transform duration-[140ms] ease-out"
        style={{
          // 缩放要同时作用在封面上（标题在缩放区外，免得文字跟着抖）
          transform: `scale(${scale})`,
          zIndex: focused ? 12 : undefined,
        }}
        onMouseEnter={onFocus}
        onContextMenu={(event) => {
          event.preventDefault();
          onDetails();
        }}
      >
        <button
          type="button"
          aria-label={game.name}
          aria-current={focused ? "true" : undefined}
          className="relative block w-full overflow-hidden rounded-xl bg-brand-800 text-left ring-1 ring-white/10"
          style={{ height: coverHeight }}
          onClick={onActivate}
        >
          {hasCover ? (
            <ProxyImage
              src={game.cover_url || game.cover_source_url}
              fallbackSrc={game.cover_source_url}
              alt={game.name}
              isNSFW={game.is_nsfw}
              className="h-full w-full object-cover object-center"
              decoding="async"
            />
          ) : (
            <span className="flex h-full w-full items-center justify-center bg-brand-800 text-lg font-semibold text-brand-500">
              {game.name.trim().slice(0, 1) || "?"}
            </span>
          )}

          {/* 未聚焦压暗：手机端用一层深色遮罩而不是给整卡降 alpha，避免文字一起糊掉 */}
          <span
            className={`pointer-events-none absolute inset-0 bg-brand-950 transition-opacity duration-150 ${
              focused ? "opacity-0" : "opacity-[0.34]"
            }`}
            aria-hidden="true"
          />

          {/* 状态点：游玩中 / 已完成 / 未游玩（未玩不给颜色，减少噪音） */}
          <span
            className={`pointer-events-none absolute bottom-1.5 left-1.5 h-2 w-2 rounded-full ring-2 ring-black/35 ${
              STATUS_DOT_CLASS[game.status] ?? "bg-brand-500"
            }`}
            aria-hidden="true"
          />

          {isFavorite && (
            <span
              className="i-mdi-heart pointer-events-none absolute right-1.5 top-1.5 text-sm text-secondary-500 drop-shadow-[0_1px_3px_rgba(0,0,0,0.8)]"
              title={t("bigScreen.favorite")}
              aria-hidden="true"
            />
          )}

          {game.is_nsfw && (
            <span className="pointer-events-none absolute bottom-1.5 right-1.5 rounded bg-rose-600/90 px-1 py-px text-[10px] font-bold leading-none text-white">
              R18
            </span>
          )}
        </button>

        {focused && (
          <span
            className="pointer-events-none absolute inset-x-0 top-0 rounded-xl ring-3 ring-secondary-500"
            style={{ height: coverHeight }}
            aria-hidden="true"
          />
        )}

        {showTitle && (
          <p className="mt-2 truncate text-center text-xs font-medium text-brand-200">
            {game.name}
          </p>
        )}
      </div>
    );
  },
);
