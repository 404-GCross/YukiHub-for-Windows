import type { appconf } from "../../src/bindings/models";

import { BIG_SCREEN_CATEGORIES } from "./categories";

/**
 * 大屏偏好的**单一事实来源**：设置页的「大屏模式」分区与大屏内的设置面板
 * （对齐手机端 `BigScreenSettings`）都从这份 schema 渲染，避免两处各写一遍之后
 * 新增一项只改了一边。
 *
 * 文案在这里就翻译好（`label` / `choices[].label`），调用方只负责排版：
 * 项目里的 i18n 提取器**只认字面量的翻译调用**，如果这里存 labelKey
 * 再由渲染层动态查表，这些键在提取器眼里就是"未被引用"，
 * `pnpm i18n:clean` 会把它们删掉并直接报错。
 */

export type BigScreenSettingKind = "action" | "select" | "switch";

export type BigScreenSettingChoice = {
  /** 已翻译的显示文案 */
  label: string;
  /** 存储值（布尔项用 "true" / "false"） */
  value: string;
};

export type BigScreenSetting = {
  choices?: BigScreenSettingChoice[];
  id: string;
  kind: BigScreenSettingKind;
  label: string;
  /** select / switch：读当前值，统一以字符串返回 */
  read?: (config: appconf.AppConfig) => string;
  /** action：执行体（清除筛选记忆 / 恢复默认设置） */
  run?: () => void;
  /** action：右列显示的说明文案（如"最近游玩"） */
  valueText?: string;
  /** select / switch：写回，返回新的 config 对象（调用方负责落盘） */
  write?: (config: appconf.AppConfig, value: string) => appconf.AppConfig;
};

export type BigScreenSettingSection = {
  id: string;
  label: string;
  settings: BigScreenSetting[];
};

/** 翻译函数的最小签名（i18next 的 `t` 兼容） */
export type BigScreenTranslator = (key: string) => string;

/** action 类条目需要的回调（由宿主注入，schema 本身保持纯数据） */
export type BigScreenSettingHandlers = {
  /** 清除「记住筛选」与每个分类各自的焦点记忆 */
  clearFilterMemory?: () => void;
  /** 当前记住的分类文案，显示在「清除筛选记忆」右侧 */
  filterMemoryLabel?: string;
  /** 恢复大屏的全部默认设置 */
  resetDefaults?: () => void;
};

type BoolField
  = | "bigscreen_focus_ticks"
    | "bigscreen_intro_enabled"
    | "bigscreen_pv_fit"
    | "bigscreen_pv_scrim"
    | "bigscreen_rail_expanded"
    | "bigscreen_remember_filter"
    | "bigscreen_show_hidden_game"
    | "bigscreen_show_titles"
    | "bigscreen_snow_enabled"
    | "bigscreen_sound_enabled"
    | "bigscreen_trailer_details_only"
    | "bigscreen_trailer_enabled"
    | "bigscreen_trailer_muted"
    | "blur_nsfw_game_covers";

type NumField
  = | "bigscreen_banner_hold_ms"
    | "bigscreen_card_scale"
    | "bigscreen_focus_scale"
    | "bigscreen_pv_scrim_percent"
    | "bigscreen_sound_volume"
    | "bigscreen_trailer_delay_ms";

type EnumField
  = | "bigscreen_default_category"
    | "bigscreen_effect_level"
    | "bigscreen_hint_mode"
    | "bigscreen_key_style";

function boolSetting(
  id: string,
  label: string,
  field: BoolField,
  t: BigScreenTranslator,
): BigScreenSetting {
  return {
    choices: [
      { label: t("bigScreen.on"), value: "true" },
      { label: t("bigScreen.off"), value: "false" },
    ],
    id,
    kind: "switch",
    label,
    read: config => (config[field] ? "true" : "false"),
    write: (config, value) =>
      ({ ...config, [field]: value === "true" }) as appconf.AppConfig,
  };
}

function enumSetting(
  id: string,
  label: string,
  field: EnumField,
  choices: BigScreenSettingChoice[],
): BigScreenSetting {
  return {
    choices,
    id,
    kind: "select",
    label,
    read: (config) => {
      const current = config[field];
      return current === undefined || current === null ? "" : String(current);
    },
    write: (config, value) =>
      ({ ...config, [field]: value }) as appconf.AppConfig,
  };
}

function numSetting(
  id: string,
  label: string,
  field: NumField,
  choices: BigScreenSettingChoice[],
): BigScreenSetting {
  return {
    choices,
    id,
    kind: "select",
    label,
    read: (config) => {
      const current = config[field];
      return current === undefined || current === null ? "" : String(current);
    },
    write: (config, value) =>
      ({ ...config, [field]: Number(value) }) as appconf.AppConfig,
  };
}

/** action 类条目：只执行一件事，右列可以显示一句状态文案。 */
function actionSetting(
  id: string,
  label: string,
  valueText: string,
  run: (() => void) | undefined,
): BigScreenSetting {
  return { id, kind: "action", label, run, valueText };
}

/**
 * 大屏偏好的分区结构（对齐手机端六个分区，去掉桌面端无对应概念的项）。
 * 顺序即界面顺序。
 *
 * 相比手机端：去掉「兼容」（KR 存档兜底是安卓专有）、「PV 占用与清理」
 * （桌面端的 PV 是本地文件路径，没有受管目录要做占用统计）。
 */
export function createBigScreenSettingSections(
  t: BigScreenTranslator,
  handlers: BigScreenSettingHandlers = {},
): BigScreenSettingSection[] {
  return [
    {
      id: "general",
      label: t("bigScreen.sectionGeneral"),
      settings: [
        enumSetting(
          "effect_level",
          t("settings.bigScreen.effectLevel"),
          "bigscreen_effect_level",
          [
            { label: t("settings.bigScreen.effectOff"), value: "off" },
            { label: t("settings.bigScreen.effectLow"), value: "low" },
            { label: t("settings.bigScreen.effectHigh"), value: "high" },
          ],
        ),
        boolSetting(
          "intro_enabled",
          t("bigScreen.introEnabled"),
          "bigscreen_intro_enabled",
          t,
        ),
        enumSetting(
          "default_category",
          t("settings.bigScreen.defaultCategory"),
          "bigscreen_default_category",
          BIG_SCREEN_CATEGORIES.map(category => ({
            label: t(category.labelKey),
            value: category.id,
          })),
        ),
        boolSetting(
          "show_hidden_game",
          t("settings.bigScreen.showHiddenGame"),
          "bigscreen_show_hidden_game",
          t,
        ),
        boolSetting(
          "remember_filter",
          t("bigScreen.rememberFilter"),
          "bigscreen_remember_filter",
          t,
        ),
      ],
    },
    {
      id: "visual",
      label: t("bigScreen.sectionVisual"),
      settings: [
        numSetting(
          "card_scale",
          t("bigScreen.cardScale"),
          "bigscreen_card_scale",
          [
            { label: t("bigScreen.scaleSmall"), value: "100" },
            { label: t("bigScreen.scaleDefault"), value: "112" },
            { label: t("bigScreen.scaleLarge"), value: "128" },
          ],
        ),
        boolSetting(
          "show_titles",
          t("bigScreen.showTitles"),
          "bigscreen_show_titles",
          t,
        ),
        numSetting(
          "focus_scale",
          t("bigScreen.focusScale"),
          "bigscreen_focus_scale",
          [
            { label: t("bigScreen.focusScaleNone"), value: "0" },
            { label: t("bigScreen.focusScaleLight"), value: "50" },
            { label: t("bigScreen.focusScaleStandard"), value: "100" },
            { label: t("bigScreen.focusScaleStrong"), value: "150" },
          ],
        ),
        boolSetting(
          "snow_enabled",
          t("bigScreen.snowEnabled"),
          "bigscreen_snow_enabled",
          t,
        ),
        boolSetting(
          "nsfw_blur",
          t("bigScreen.nsfwBlur"),
          "blur_nsfw_game_covers",
          t,
        ),
        boolSetting(
          "rail_expanded",
          t("bigScreen.railExpanded"),
          "bigscreen_rail_expanded",
          t,
        ),
        enumSetting(
          "hint_mode",
          t("bigScreen.hintMode"),
          "bigscreen_hint_mode",
          [
            { label: t("bigScreen.hintModeAuto"), value: "auto" },
            { label: t("bigScreen.hintModeAlways"), value: "always" },
            { label: t("bigScreen.hintModeOff"), value: "off" },
          ],
        ),
        enumSetting(
          "key_style",
          t("bigScreen.keyStyle"),
          "bigscreen_key_style",
          [
            { label: t("bigScreen.keyStyleXbox"), value: "xbox" },
            { label: t("bigScreen.keyStylePs"), value: "ps" },
          ],
        ),
      ],
    },
    {
      id: "audio",
      label: t("bigScreen.sectionAudio"),
      settings: [
        boolSetting(
          "sound_enabled",
          t("settings.bigScreen.soundEnabled"),
          "bigscreen_sound_enabled",
          t,
        ),
        numSetting(
          "sound_volume",
          t("bigScreen.soundVolume"),
          "bigscreen_sound_volume",
          [
            { label: t("bigScreen.volume0"), value: "0" },
            { label: t("bigScreen.volume25"), value: "25" },
            { label: t("bigScreen.volume50"), value: "50" },
            { label: t("bigScreen.volume75"), value: "75" },
            { label: t("bigScreen.volume100"), value: "100" },
          ],
        ),
        boolSetting(
          "focus_ticks",
          t("bigScreen.focusTicks"),
          "bigscreen_focus_ticks",
          t,
        ),
        boolSetting(
          "trailer_muted",
          t("bigScreen.trailerMuted"),
          "bigscreen_trailer_muted",
          t,
        ),
      ],
    },
    {
      id: "layout",
      label: t("bigScreen.sectionLayout"),
      settings: [
        numSetting(
          "banner_hold",
          t("bigScreen.bannerHold"),
          "bigscreen_banner_hold_ms",
          [
            { label: t("bigScreen.bannerHoldShort"), value: "1200" },
            { label: t("bigScreen.bannerHoldStandard"), value: "2000" },
            { label: t("bigScreen.bannerHoldLong"), value: "3000" },
            { label: t("bigScreen.bannerHoldLongest"), value: "4500" },
          ],
        ),
        boolSetting(
          "trailer_enabled",
          t("bigScreen.trailerEnabled"),
          "bigscreen_trailer_enabled",
          t,
        ),
        numSetting(
          "trailer_delay",
          t("bigScreen.trailerDelay"),
          "bigscreen_trailer_delay_ms",
          [
            { label: t("bigScreen.delayShort"), value: "500" },
            { label: t("bigScreen.delayStandard"), value: "1200" },
            { label: t("bigScreen.delayLong"), value: "2000" },
            { label: t("bigScreen.delayLongest"), value: "3000" },
          ],
        ),
        boolSetting(
          "trailer_details_only",
          t("bigScreen.trailerDetailsOnly"),
          "bigscreen_trailer_details_only",
          t,
        ),
        boolSetting("pv_fit", t("bigScreen.pvFit"), "bigscreen_pv_fit", t),
        boolSetting(
          "pv_scrim",
          t("bigScreen.pvScrim"),
          "bigscreen_pv_scrim",
          t,
        ),
        numSetting(
          "pv_scrim_percent",
          t("bigScreen.pvScrimPercent"),
          "bigscreen_pv_scrim_percent",
          [
            { label: t("bigScreen.percent0"), value: "0" },
            { label: t("bigScreen.percent20"), value: "20" },
            { label: t("bigScreen.percent35"), value: "35" },
            { label: t("bigScreen.percent45"), value: "45" },
            { label: t("bigScreen.percent60"), value: "60" },
            { label: t("bigScreen.percent80"), value: "80" },
          ],
        ),
      ],
    },
    {
      id: "menu",
      label: t("bigScreen.sectionMenu"),
      settings: [
        actionSetting(
          "clear_filter_memory",
          t("bigScreen.clearFilterMemory"),
          handlers.filterMemoryLabel ?? "",
          handlers.clearFilterMemory,
        ),
        actionSetting(
          "reset_defaults",
          t("bigScreen.resetDefaults"),
          "",
          handlers.resetDefaults,
        ),
      ],
    },
  ];
}

/**
 * 设置项当前值的显示文案（大屏内设置面板右列的 value 列）。
 */
export function formatBigScreenSettingValue(
  setting: BigScreenSetting,
  config: appconf.AppConfig,
): string {
  if (setting.kind === "action") {
    return setting.valueText ?? "";
  }
  const current = setting.read?.(config) ?? "";
  const choice = setting.choices?.find(item => item.value === current);
  return choice ? choice.label : current;
}
