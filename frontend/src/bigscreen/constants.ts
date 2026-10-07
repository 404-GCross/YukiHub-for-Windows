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
 * 上下方向在焦点区之间的穿梭顺序。
 *
 * 侧栏刻意不在链路里：否则从货架向上会被引擎按「跨区夹紧」丢到分类栏最后一个条目。
 * 侧栏只由货架的「左 / 上」边界显式进入，返回则用「下 / 右 / Enter」。
 */
export const BIG_SCREEN_FOCUS_ORDER = [
  BIG_SCREEN_SHELF_ZONE,
  BIG_SCREEN_ACTIONS_ZONE,
];

/** 单排货架的卡片上限，对齐手机端 BigScreenRepository 的 500 */
export const BIG_SCREEN_SHELF_LIMIT = 500;

/** 单次请求的分页大小上限，对齐后端 gamehelper.MaxGameListLimit */
export const BIG_SCREEN_PAGE_LIMIT = 240;

/** 侧栏收起 / 展开宽度（px），对齐手机端 BigScreenSizes 的夹取区间上沿 */
export const BIG_SCREEN_RAIL_COLLAPSED_WIDTH = 76;
export const BIG_SCREEN_RAIL_EXPANDED_WIDTH = 216;

/** 封面长宽比，与 GameCard 的 `aspect-[3/3.6]` 保持一致 */
const COVER_ASPECT_RATIO = 3.6 / 3;

/** 卡片之间的横向间距（px） */
export const BIG_SCREEN_CARD_GAP = 14;

/** 卡片标题区高度（仅在 `bigscreen_show_titles` 打开时占位） */
const CARD_TITLE_HEIGHT = 28;

/** 行标题高度占内容区的比例（对齐手机端 `headerH = rowTotal × 0.20`） */
const HEADER_HEIGHT_RATIO = 0.2;
const MIN_HEADER_HEIGHT = 24;
const MAX_HEADER_HEIGHT = 44;

/**
 * 封面高度的夹取区间（px）。
 *
 * 手机端夹在 88–200dp，但**桌面端要更小**：PC 屏幕大、视距远，
 * 卡片占满屏高只会显得笨重。这里让整张卡约占内容区高度的四分之一到三成。
 */
const MIN_CARD_COVER_HEIGHT = 130;

function resolveMaxCardCoverHeight(areaHeight: number) {
  return Math.round(Math.min(Math.max(areaHeight * 0.28, 220), 400));
}

/**
 * 信息浮层要预留的高度占内容区的比例，对应手机端 `infoReserveH = clamp(h×0.34, 100, 150)`。
 *
 * 浮层本身是压在背景上的绝对定位块（不占布局高度），这里只是**预留视觉空间**，
 * 免得卡片排被大标题压住。
 */
const INFO_RESERVE_RATIO = 0.3;
const MIN_INFO_RESERVE_HEIGHT = 120;
const MAX_INFO_RESERVE_HEIGHT = 190;

/**
 * 一张卡在行里要留出的上下余量（px）。
 *
 * 焦点卡会按用户档位放大（默认 1.045），而货架容器是 `overflow-x-auto`
 * （`overflow-y` 必然退化成 hidden），没有余量就会被裁掉 —— 这就是
 * 「选中后被挤出来」的观感来源之一。
 */
const CARD_SCALE_HEADROOM = 16;

/** 卡片宽度占内容区宽度的上限（对齐手机端 `cardW ≤ wDp × 0.17`，保证一屏好几张） */
const CARD_WIDTH_RATIO_OF_AREA = 0.12;

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
