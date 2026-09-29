import { enums } from "../../src/bindings/models";

/**
 * 手机版卡片状态徽标（item_game_card.xml 的 tvStatusBadge）配色。
 * 覆盖在封面上，使用半透明深色底 + 白色文字，保证任意封面下都可读。
 */
export const GAME_STATUS_BADGE_STYLES: Partial<
  Record<
    enums.GameStatus,
    { className: string; icon: string; labelKey: string }
  >
> = {
  [enums.GameStatus.StatusNotStarted]: {
    className: "bg-brand-900/55",
    icon: "i-mdi-clock-outline",
    labelKey: "common.notStarted",
  },
  [enums.GameStatus.StatusWantToPlay]: {
    className: "bg-info-600/70",
    icon: "i-mdi-bookmark-outline",
    labelKey: "common.wantToPlay",
  },
  [enums.GameStatus.StatusPlaying]: {
    className: "bg-success-600/70",
    icon: "i-mdi-gamepad-variant",
    labelKey: "common.playing",
  },
  [enums.GameStatus.StatusCompleted]: {
    className: "bg-warning-600/80",
    icon: "i-mdi-trophy",
    labelKey: "common.completed",
  },
  [enums.GameStatus.StatusOnHold]: {
    className: "bg-orange-600/70",
    icon: "i-mdi-pause-circle-outline",
    labelKey: "common.onHold",
  },
  [enums.GameStatus.StatusDropped]: {
    className: "bg-error-600/70",
    icon: "i-mdi-delete-outline",
    labelKey: "common.dropped",
  },
};
