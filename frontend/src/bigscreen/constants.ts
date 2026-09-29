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

const MIN_CARD_WIDTH = 96;

/** 由货架可视高度推导卡片宽度（对齐手机端 BigScreenSizes 的等比缩放思路） */
export function resolveBigScreenCardWidth(rowHeight: number) {
  const available = rowHeight - CARD_META_HEIGHT - BIG_SCREEN_CARD_GAP * 2;
  return Math.max(MIN_CARD_WIDTH, Math.round(available / COVER_ASPECT_RATIO));
}
