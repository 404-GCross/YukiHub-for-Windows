/** 大屏模式的路由路径（同窗口无外壳全屏路由） */
export const BIG_SCREEN_PATH = "/bigscreen";

/** 退出大屏模式后回到首页 */
export const BIG_SCREEN_EXIT_PATH = "/";

/** 焦点区：左侧分类栏 */
export const BIG_SCREEN_RAIL_ZONE = "rail";

/** 焦点区：内容货架 */
export const BIG_SCREEN_SHELF_ZONE = "shelf";

/** 焦点区：底部按钮排 */
export const BIG_SCREEN_ACTIONS_ZONE = "actions";

/** 焦点区：详情层按钮排（详情层打开时它是唯一有内容的区域） */
export const BIG_SCREEN_DETAILS_ZONE = "details";

/** 焦点区：全屏预告片播放器（播放期间它是唯一有内容的区域，不进穿梭链路） */
export const BIG_SCREEN_TRAILER_ZONE = "trailer";

/**
 * 上下方向在焦点区之间的穿梭顺序（**只登记参与上下穿梭的区域**）。
 *
 * 对齐手机端主界面：按钮排在**货架上方**（信息浮层就压在背景上部），
 * 所以从卡片排按「上」是进按钮排、按「下」在按钮排里回卡片排；
 * 卡片排按「下」到边界即停，不会跑到别的地方。
 *
 * 侧栏与详情层刻意不登记：侧栏只由货架的「←」边界显式进入（返回用「→ / Ⓐ」），
 * 详情层自己处理 ↑↓（滚动左列）。早先的实现会把未登记区域追加到末尾，
 * 于是「货架按 ↓」会被引擎丢进侧栏。
 */
export const BIG_SCREEN_FOCUS_ORDER = [
  BIG_SCREEN_ACTIONS_ZONE,
  BIG_SCREEN_SHELF_ZONE,
];

/** 单排货架的卡片上限，对齐手机端 BigScreenRepository 的 500 */
export const BIG_SCREEN_SHELF_LIMIT = 500;

/** 单次请求的分页大小上限，对齐后端 gamehelper.MaxGameListLimit */
export const BIG_SCREEN_PAGE_LIMIT = 240;

/** 侧栏收起 / 展开宽度（px）。桌面端比手机端窄一档：鼠标视距近，不需要那么宽的点击区 */
export const BIG_SCREEN_RAIL_COLLAPSED_WIDTH = 68;
export const BIG_SCREEN_RAIL_EXPANDED_WIDTH = 200;

/** 封面长宽比，与 GameCard 的 `aspect-[3/3.6]` 保持一致 */
const COVER_ASPECT_RATIO = 3.6 / 3;

/** 卡片之间的横向间距（px） */
export const BIG_SCREEN_CARD_GAP = 12;

/** 卡片标题区高度（仅在 `bigscreen_show_titles` 打开时占位） */
const CARD_TITLE_HEIGHT = 24;

/** 行标题高度占内容区的比例（对齐手机端 `headerH = rowTotal × 0.20`） */
const HEADER_HEIGHT_RATIO = 0.18;
const MIN_HEADER_HEIGHT = 22;
const MAX_HEADER_HEIGHT = 36;

/**
 * 封面高度的夹取区间（px）。
 *
 * 手机端夹在 88–200dp，桌面端视距更远、屏幕更大，但**整体密度要更高**：
 * 卡片占屏高的比例刻意压到 22% 上下（手机端约 27%），一屏能多看两三张。
 */
const MIN_CARD_COVER_HEIGHT = 110;

function resolveMaxCardCoverHeight(areaHeight: number) {
  return Math.round(Math.min(Math.max(areaHeight * 0.22, 180), 320));
}

/**
 * 信息浮层要预留的高度占内容区的比例，对应手机端 `infoReserveH = clamp(h×0.34, 100, 150)`。
 *
 * 浮层本身是压在背景上的绝对定位块（不占布局高度），这里只是**预留视觉空间**，
 * 免得卡片排被大标题压住。桌面端把这块也收窄，让卡片排更靠上。
 */
const INFO_RESERVE_RATIO = 0.24;
const MIN_INFO_RESERVE_HEIGHT = 96;
const MAX_INFO_RESERVE_HEIGHT = 150;

/**
 * 一张卡在行里要留出的上下余量（px）。
 *
 * 焦点卡会按用户档位放大（默认 1.045），而货架容器是 `overflow-x-auto`
 * （`overflow-y` 必然退化成 hidden），没有余量就会被裁掉 —— 这就是
 * 「选中后被挤出来」的观感来源之一。
 */
const CARD_SCALE_HEADROOM = 14;

/** 卡片宽度占内容区宽度的上限（对齐手机端 `cardW ≤ wDp × 0.17`，保证一屏好几张） */
const CARD_WIDTH_RATIO_OF_AREA = 0.105;

/** 卡片缩放的夹取区间（与 Go 侧 NormalizeBigScreenCardScale 一致） */
export const MIN_BIG_SCREEN_CARD_SCALE = 80;
export const MAX_BIG_SCREEN_CARD_SCALE = 140;

export type BigScreenShelfMetrics = {
  cardWidth: number;
  /** 封面高度（px），卡片本体按它撑开 */
  coverHeight: number;
  /** 行标题区高度 */
  headerHeight: number;
  /** 信息浮层预留的高度（卡片排上方的空白） */
  infoReserveHeight: number;
  /** 卡片行的实际高度（含缩放余量与标题），卡片在其中垂直居中 */
  rowHeight: number;
};

export type BigScreenShelfMetricsOptions = {
  /** 卡片大小倍率（×100，80–140），来自 `bigscreen_card_scale` */
  cardScale: number;
  /** 是否在卡片下方显示游戏名，来自 `bigscreen_show_titles` */
  showTitles: boolean;
};

/**
 * 大屏货架的尺寸预算，照搬手机端 `BigScreenSizes` 的思路：
 *
 * ```
 * rowTotal = 内容区 / 1.35      // 手机用来露出下一行的一角
 * headerH  = rowTotal × 0.20    // 行标题
 * cardH    = rowTotal − headerH − gap×2，夹取 88–200dp
 * cardW    = min(cardH × 0.75, 屏宽 × 0.17)
 * ```
 *
 * 桌面端是**单排**（不换行），所以把内容区高度整块留给这一行：
 * 行标题 + 卡片 + 缩放余量必须塞得下，因此先按高度算出封面高，再用宽度上限
 * 收一次 —— 两个约束取小，卡片就一定不会溢出。
 *
 * 用户的「卡片大小」档位作用在**封面高度**上（再据此推宽度），
 * 这样三档之间是等比放大，而不是只把某一边拉长。
 */
export function resolveBigScreenShelfMetrics(
  area: { height: number; width: number },
  options: BigScreenShelfMetricsOptions = { cardScale: 112, showTitles: false },
): BigScreenShelfMetrics {
  const empty: BigScreenShelfMetrics = {
    cardWidth: 0,
    coverHeight: 0,
    headerHeight: 0,
    infoReserveHeight: 0,
    rowHeight: 0,
  };
  if (area.height <= 0 || area.width <= 0) {
    return empty;
  }

  const scale
    = Math.min(
      Math.max(options.cardScale, MIN_BIG_SCREEN_CARD_SCALE),
      MAX_BIG_SCREEN_CARD_SCALE,
    ) / 100;
  const metaHeight = options.showTitles ? CARD_TITLE_HEIGHT : 0;

  // 卡片排在最底部，它上方依次是行标题与信息浮层
  const infoReserveHeight = Math.round(
    Math.min(
      Math.max(area.height * INFO_RESERVE_RATIO, MIN_INFO_RESERVE_HEIGHT),
      MAX_INFO_RESERVE_HEIGHT,
    ),
  );
  const rowBudget = area.height - infoReserveHeight;
  const headerHeight = Math.round(
    Math.min(
      Math.max(rowBudget * HEADER_HEIGHT_RATIO, MIN_HEADER_HEIGHT),
      MAX_HEADER_HEIGHT,
    ),
  );
  const available
    = rowBudget
      - headerHeight
      - BIG_SCREEN_CARD_GAP * 2
      - metaHeight
      - CARD_SCALE_HEADROOM * 2;
  const baseCoverHeight = Math.min(
    Math.max(available, MIN_CARD_COVER_HEIGHT),
    resolveMaxCardCoverHeight(area.height),
  );
  const coverHeight = Math.round(baseCoverHeight * scale);
  const cardWidth = Math.max(
    72,
    Math.min(
      Math.round(coverHeight / COVER_ASPECT_RATIO),
      Math.round(area.width * CARD_WIDTH_RATIO_OF_AREA),
    ),
  );

  return {
    cardWidth,
    coverHeight,
    headerHeight,
    infoReserveHeight,
    rowHeight: coverHeight + metaHeight + CARD_SCALE_HEADROOM * 2,
  };
}

/** 氛围特效档位 */
export type BigScreenEffectLevel = "off" | "low" | "high";

/**
 * 归一化配置里的特效档位：识别不了的值一律回落到 `low`，
 * 与 Go 侧 `NormalizeBigScreenEffectLevel` 的白名单保持一致。
 */
export function resolveBigScreenEffectLevel(
  value: string | undefined,
): BigScreenEffectLevel {
  return value === "off" || value === "high" ? value : "low";
}

/** 入场错峰延迟：第 idx 个条目的延迟（对齐手机端 42ms×idx，且封顶避免长列表等待） */
export const BIG_SCREEN_ENTER_STAGGER_MS = 42;

/** 错峰延迟的最大档位，货架滚动到后面的卡片时不会再重新等待 */
export const BIG_SCREEN_ENTER_STAGGER_MAX = 12;

export function resolveBigScreenEnterDelay(index: number) {
  return (
    Math.min(Math.max(index, 0), BIG_SCREEN_ENTER_STAGGER_MAX)
    * BIG_SCREEN_ENTER_STAGGER_MS
  );
}

/** 货架焦点停留多久后才在背景起播预告片（对齐手机端 bsBgVideo 的延迟起播） */
export const BIG_SCREEN_TRAILER_HOVER_DELAY_MS = 1200;

/**
 * 大屏偏好的默认值，与 Go 侧 `defaultAppConfig()` 的大屏段一一对应。
 *
 * 「恢复默认设置」（大屏内面板与设置页两处）都写回这一份，避免两处各写一套默认值。
 * 不含 `blur_nsfw_game_covers` —— 那是主库共用项，恢复大屏默认时不该被动到。
 */
export const BIG_SCREEN_DEFAULT_CONFIG = {
  bigscreen_banner_hold_ms: 2000,
  bigscreen_card_scale: 112,
  bigscreen_default_category: "recent",
  bigscreen_effect_level: "low",
  bigscreen_focus_scale: 100,
  bigscreen_focus_ticks: true,
  bigscreen_hint_mode: "auto",
  bigscreen_intro_enabled: true,
  bigscreen_key_style: "xbox",
  bigscreen_last_category: "",
  bigscreen_pv_fit: false,
  bigscreen_pv_scrim: true,
  bigscreen_pv_scrim_percent: 45,
  bigscreen_rail_expanded: false,
  bigscreen_remember_filter: true,
  bigscreen_show_hidden_game: false,
  bigscreen_show_titles: false,
  bigscreen_snow_enabled: true,
  bigscreen_sound_enabled: true,
  bigscreen_sound_volume: 65,
  bigscreen_trailer_delay_ms: 2000,
  bigscreen_trailer_details_only: false,
  bigscreen_trailer_enabled: true,
  bigscreen_trailer_muted: false,
} as const;
