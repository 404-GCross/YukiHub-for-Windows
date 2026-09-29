import type { models } from "../../src/bindings/models";
import { memo } from "react";
import { useTranslation } from "react-i18next";

import { statusOptions } from "../consts/options";
import { useGamePlaytime } from "../hooks/useGamePlaytime";
import { useAppStore } from "../store";
import { getTagDisplayName } from "../utils/tagTranslation";
import { formatDuration } from "../utils/time";

export interface BigScreenAction {
  icon: string;
  key: string;
  label: string;
  run: () => void;
}

interface BigScreenInfoBarProps {
  actions: BigScreenAction[];
  actionsFocused: boolean;
  focusedActionIndex: number;
  game: models.Game | undefined;
  isFavorite: boolean;
  onActionActivate: (index: number) => void;
  onActionFocus: (index: number) => void;
  tags: models.GameTag[];
}

/** 大屏信息浮层，对齐手机端 `bsInfoBar`：标题 / 副行 / 标签 / 元数据 / 操作按钮排。 */
export const BigScreenInfoBar = memo(
  ({
    actions,
    actionsFocused,
    focusedActionIndex,
    game,
    isFavorite,
    onActionActivate,
    onActionFocus,
    tags,
  }: BigScreenInfoBarProps) => {
    const { t } = useTranslation();
    const enableTagTranslation = useAppStore(
      state => state.config?.enable_tag_translation ?? true,
    );
    // 货架加载时已批量带回时长并预填缓存，这里通常不会再发起单查
    const playTime = useGamePlaytime(game?.id);

    if (!game) {
      return null;
    }

    const company = game.company || t("common.unknownDeveloper");
    const statusLabel = statusOptions.find(
      option => option.value === game.status,
    )?.label;
    const visibleTags = tags.slice(0, 3);

    return (
      /*
        对齐手机端 `bsInfoBar`：左侧一块**竖向堆叠**的浮层，顺序是
        标题 → 标签 chips → 副行（开发商 · 年份…）→ 操作按钮排。
        它压在背景大图上，卡片排在最底部（见 routes/bigscreen.tsx 的布局）。
      */
      <div className="pointer-events-none flex max-w-3xl flex-col items-start gap-3">
        <div className="min-w-0 max-w-3xl">
          <h1 className="truncate text-4xl font-bold leading-tight text-white drop-shadow-[0_2px_12px_rgba(0,0,0,0.55)]">
            {game.name}
          </h1>
        </div>

        {visibleTags.length > 0 && (
          <div className="flex flex-wrap items-center gap-2">
            {visibleTags.map(tag => (
              <span key={tag.id} className="yh-chip text-[11px]">
                {getTagDisplayName(tag.name, enableTagTranslation)}
              </span>
            ))}
          </div>
        )}

        <div className="min-w-0 max-w-3xl">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-brand-400">
            <span className="truncate">{company}</span>
            {game.release_date && (
              <>
                <span aria-hidden="true">·</span>
                <span>{game.release_date}</span>
              </>
            )}
            {statusLabel && (
              <>
                <span aria-hidden="true">·</span>
                <span>{t(statusLabel)}</span>
              </>
            )}
            {playTime > 0 && (
              <>
                <span aria-hidden="true">·</span>
                <span className="inline-flex items-center gap-1 text-white">
                  <span
                    className="i-mdi-timer-outline text-xs text-primary-300"
                    aria-hidden="true"
                  />
                  {formatDuration(playTime, t)}
                </span>
              </>
            )}
            {game.rating > 0 && (
              <>
                <span aria-hidden="true">·</span>
                <span className="inline-flex items-center gap-1 text-white">
                  <span
                    className="i-mdi-star text-xs text-yellow-300"
                    aria-hidden="true"
                  />
                  {game.rating.toFixed(1)}
                </span>
              </>
            )}
          </div>
        </div>

        <div className="pointer-events-auto flex shrink-0 items-center gap-3">
          {actions.map((action, index) => {
            const isFocused = actionsFocused && index === focusedActionIndex;
            const isFavoriteAction = action.key === "favorite";

            return (
              <button
                key={action.key}
                type="button"
                aria-label={action.label}
                aria-pressed={isFavoriteAction ? isFavorite : undefined}
                className={`inline-flex items-center gap-2 rounded-full border px-6 py-3 text-sm font-medium transition-all duration-150 ${
                  isFocused
                    ? "scale-105 border-secondary-500 bg-brand-750 text-white"
                    : "border-brand-700 bg-brand-800/70 text-brand-400 hover:border-brand-600 hover:text-white"
                }`}
                onClick={() => onActionActivate(index)}
                onMouseEnter={() => onActionFocus(index)}
              >
                <span
                  className={`${
                    isFavoriteAction && isFavorite
                      ? "i-mdi-heart text-secondary-500"
                      : action.icon
                  } text-lg`}
                  aria-hidden="true"
                />
                {action.label}
              </button>
            );
          })}
        </div>
      </div>
    );
  },
);
