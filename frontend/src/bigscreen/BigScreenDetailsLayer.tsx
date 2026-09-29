import type { models } from "../../src/bindings/models";
import type { BigScreenInputDevice } from "./useGamepad";
import { memo } from "react";

import { useTranslation } from "react-i18next";
import { statusOptions } from "../consts/options";
import { useGamePlaytime } from "../hooks/useGamePlaytime";
import { useAppStore } from "../store";
import { getTagDisplayName } from "../utils/tagTranslation";
import { formatDurationCompact, formatLocalDate } from "../utils/time";
import { BigScreenHintBar } from "./BigScreenHintBar";

export interface BigScreenDetailAction {
  icon: string;
  key: string;
  label: string;
  run: () => void;
  /** 不可用（例如游戏没有本地预告片）时禁用而非隐藏，以保持焦点索引稳定 */
  disabled?: boolean;
}

interface BigScreenDetailsLayerProps {
  actions: BigScreenDetailAction[];
  actionsFocused: boolean;
  focusedActionIndex: number;
  game: models.Game;
  /** 最近一次使用的输入设备，决定底部提示显示手柄图标还是键盘按键 */
  inputDevice: BigScreenInputDevice;
  onActionActivate: (index: number) => void;
  onActionFocus: (index: number) => void;
  /** 关闭详情层回到货架 */
  onClose: () => void;
  tags: models.GameTag[];
}

/**
 * 大屏详情层，对齐手机端 `BigScreenDetailsLayer`：
 * 标题 / 副行（原文名·开发商·发行日期）/ 标签 chips（≤3 + R18）/ 统计块 / 简介 / 封面。
 *
 * 与手机端的差异（截图画带需要先补桌面端截图能力，本轮降级为封面大图）记在
 * `docs/ROADMAP.md`；「观看 PV」已在 M3 补齐，无本地预告片时按钮禁用而非隐藏。
 */
export const BigScreenDetailsLayer = memo(
  ({
    actions,
    actionsFocused,
    focusedActionIndex,
    game,
    inputDevice,
    onActionActivate,
    onActionFocus,
    onClose,
    tags,
  }: BigScreenDetailsLayerProps) => {
    const { t } = useTranslation();
    const enableTagTranslation = useAppStore(
      state => state.config?.enable_tag_translation ?? true,
    );
    const playTime = useGamePlaytime(game.id);

    const originalTitle
      = game.aliases?.find(alias => alias.trim().length > 0) ?? "";
    const statusLabel = statusOptions.find(
      option => option.value === game.status,
    )?.label;
    const visibleTags = tags.slice(0, 3);

    const stats = [
      {
        label: t("bigScreen.playTime"),
        value: formatDurationCompact(playTime, t),
      },
      {
        label: t("common.lastPlayedAt"),
        value: game.last_played_at
          ? formatLocalDate(game.last_played_at)
          : t("common.never"),
      },
      {
        label: t("common.status"),
        value: statusLabel ? t(statusLabel) : t("common.unknownDate"),
      },
      {
        label: t("common.rating"),
        value:
          game.rating > 0 ? game.rating.toFixed(1) : t("common.unknownDate"),
      },
    ];

    const coverUrl = game.cover_url || game.cover_source_url;

    return (
      <div
        className="absolute inset-0 z-30 flex items-center justify-center bg-brand-950/85 px-16 py-10 backdrop-blur-md"
        onClick={onClose}
      >
        <div
          className="grid w-full max-w-6xl grid-cols-[minmax(0,320px)_minmax(0,1fr)] items-start gap-10"
          onClick={event => event.stopPropagation()}
        >
          <div className="relative aspect-[3/3.6] overflow-hidden rounded-2xl bg-brand-800 shadow-2xl">
            {coverUrl && (
              <img
                className="h-full w-full object-cover"
                src={coverUrl}
                alt={game.name}
              />
            )}
            {game.is_nsfw && (
              <span className="absolute right-3 top-3 rounded bg-rose-600/90 px-2 py-0.5 text-xs font-bold text-white">
                R18
              </span>
            )}
          </div>

          <div className="flex max-h-[70vh] min-h-0 flex-col">
            <h2 className="text-5xl font-bold leading-tight text-white drop-shadow-[0_2px_12px_rgba(0,0,0,0.55)]">
              {game.name}
            </h2>

            <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-brand-400">
              {originalTitle && (
                <span className="truncate">{originalTitle}</span>
              )}
              {game.company && (
                <>
                  <span aria-hidden="true">·</span>
                  <span className="truncate">{game.company}</span>
                </>
              )}
              {game.release_date && (
                <>
                  <span aria-hidden="true">·</span>
                  <span>{game.release_date}</span>
                </>
              )}
            </div>

            {visibleTags.length > 0 && (
              <div className="mt-4 flex flex-wrap items-center gap-2">
                {visibleTags.map(tag => (
                  <span key={tag.id} className="yh-chip text-[11px]">
                    {getTagDisplayName(tag.name, enableTagTranslation)}
                  </span>
                ))}
              </div>
            )}

            <div className="mt-6 grid grid-cols-4 gap-4">
              {stats.map(item => (
                <div
                  key={item.label}
                  className="rounded-xl border border-white/10 bg-white/5 px-4 py-3"
                >
                  <div className="text-xs text-brand-400">{item.label}</div>
                  <div className="mt-1 truncate text-lg font-semibold text-white">
                    {item.value}
                  </div>
                </div>
              ))}
            </div>

            <div className="mt-6 min-h-0 flex-1 overflow-y-auto pr-2 text-sm leading-relaxed text-brand-300">
              {game.summary || t("common.unknownDate")}
            </div>

            <div className="mt-8 flex shrink-0 flex-wrap items-center gap-3">
              {actions.map((action, index) => {
                const isFocused
                  = actionsFocused && index === focusedActionIndex;

                return (
                  <button
                    key={action.key}
                    type="button"
                    aria-label={action.label}
                    disabled={action.disabled}
                    title={
                      action.disabled
                        ? t("bigScreen.trailerUnavailable")
                        : undefined
                    }
                    className={`inline-flex items-center gap-2 rounded-full border px-6 py-3 text-sm font-medium transition-all duration-150 ${
                      action.disabled
                        ? "cursor-not-allowed border-brand-800 bg-brand-800/40 text-brand-600"
                        : isFocused
                          ? "scale-105 border-secondary-500 bg-brand-750 text-white"
                          : "border-brand-700 bg-brand-800/70 text-brand-400 hover:border-brand-600 hover:text-white"
                    }`}
                    onClick={() => onActionActivate(index)}
                    onMouseEnter={() => onActionFocus(index)}
                  >
                    <span
                      className={`${action.icon} text-lg`}
                      aria-hidden="true"
                    />
                    {action.label}
                  </button>
                );
              })}
            </div>

            <BigScreenHintBar
              className="mt-4 shrink-0"
              inputDevice={inputDevice}
              variant="details"
            />
          </div>
        </div>
      </div>
    );
  },
);
