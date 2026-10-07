/**
 * 大屏按键图标字形，对齐手机端 `BigScreenKeys`。
 *
 * 用字符字形而不是图片：ⒶⓍ 与 ✕○△□ 都是通用符号，字体里就有，
 * 既不出图、也能随字号缩放、还跟文字基线对齐 —— 比切图更稳。
 * 底栏提示、信息浮层按钮、详情层按钮与各面板都从这里取，切换风格后立刻跟着变。
 */

export type BigScreenKeyStyle = "ps" | "xbox";

export type BigScreenKeyGlyphs = {
  /** 返回 */
  back: string;
  /** 确认 / 启动 */
  confirm: string;
  /** 第四键（详情 / 菜单） */
  fourth: string;
  /** 左肩键 */
  lb: string;
  /** 菜单键 */
  menu: string;
  /** 右肩键 */
  rb: string;
  /** 风格显示名 */
  styleName: string;
  /** 第三键（收藏） */
  third: string;
};

const GLYPHS: Record<BigScreenKeyStyle, BigScreenKeyGlyphs> = {
  xbox: {
    back: "Ⓑ",
    confirm: "Ⓐ",
    fourth: "Ⓨ",
    lb: "LB",
    menu: "☰",
    rb: "RB",
    styleName: "Xbox",
    third: "Ⓧ",
  },
  ps: {
    back: "○",
    confirm: "✕",
    fourth: "△",
    lb: "L1",
    menu: "Options",
    rb: "R1",
    styleName: "PlayStation",
    third: "□",
  },
};

/** 识别不了的风格一律回 Xbox，与 Go 侧 `NormalizeBigScreenKeyStyle` 保持一致。 */
export function resolveBigScreenKeyStyle(
  style: string | undefined,
): BigScreenKeyStyle {
  return style === "ps" ? "ps" : "xbox";
}

export function resolveBigScreenKeyGlyphs(
  style: string | undefined,
): BigScreenKeyGlyphs {
  return GLYPHS[resolveBigScreenKeyStyle(style)];
}

/** 键盘提示用的按键名（没有手柄字形时才用） */
export const KEYBOARD_GLYPHS = {
  back: "Esc",
  confirm: "Enter",
  categoryLeft: "Q",
  categoryRight: "E",
  details: "Y",
  favorite: "X",
  menu: "Tab",
  move: "WASD / ← → ↑ ↓",
} as const;
