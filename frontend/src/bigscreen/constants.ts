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

/** GameCard 底部标题 + 厂商区的高度（px） */
const CARD_META_HEIGHT = 52;

/**
 * 封面高度占货架可视高度的比例。
 *
 * 对齐手机端 `BigScreenSizes` 的等比思路（手机端 `rowTotal = 内容区 / 1.35`，
 * 卡片再占掉其中一部分）。桌面端取 0.42：一排卡片约占货架高度的一半，
 * 上下留出背景与行标题的空间，是「沉浸式货架」而不是「贴满整屏」。
 *
 * **尺寸必须随屏幕缩放并夹取**：早先的写法直接把货架可视高度当成卡片高度，
 * 1080p 全屏下卡片会被放大到近 700px 宽、占满整屏，右侧全是空白。
 */
const CARD_HEIGHT_RATIO = 0.42;

/** 封面高度的夹取区间（px），防止极端窗口下过小或过大 */
const MIN_CARD_COVER_HEIGHT = 140;
const MAX_CARD_COVER_HEIGHT = 520;

/** 由货架可视高度推导卡片宽度（先算封面高度、夹取后再按比例换算宽度） */
export function resolveBigScreenCardWidth(rowHeight: number) {
  // 比例说的是「整张卡（封面 + 标题区）」占货架高度的多少
  const available = rowHeight * CARD_HEIGHT_RATIO - CARD_META_HEIGHT;
  const coverHeight = Math.min(
    Math.max(available, MIN_CARD_COVER_HEIGHT),
    MAX_CARD_COVER_HEIGHT,
  );
  return Math.round(coverHeight / COVER_ASPECT_RATIO);
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
