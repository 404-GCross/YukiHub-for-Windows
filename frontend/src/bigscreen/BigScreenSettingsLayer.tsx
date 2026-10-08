import type { appconf } from "../../src/bindings/models";
import type { BigScreenSettingSection } from "./settingsSchema";
import type { BigScreenIntent } from "./useGamepad";
import {
  forwardRef,
  useCallback,
  useImperativeHandle,
  useMemo,
  useState,
} from "react";

import { useTranslation } from "react-i18next";
import { formatBigScreenSettingValue } from "./settingsSchema";

export interface BigScreenSettingsLayerHandle {
  /** @returns true 表示该意图已被设置面板消费 */
  handleIntent: (intent: BigScreenIntent) => boolean;
}

interface BigScreenSettingsLayerProps {
  config: appconf.AppConfig;
  onChange: (next: appconf.AppConfig) => void;
  onClose: () => void;
  /** select 类条目：交给调用方用通用浮层展示候选值 */
  onOpenChoices: (sectionIndex: number, itemIndex: number) => void;
  sections: BigScreenSettingSection[];
}

type Column = "item" | "section";

/**
 * 大屏内的设置面板，对齐手机端 `BigScreenSettings`：左列分区、右列条目，
 * ←→ 在两列之间切换，↑↓ 在当前列内移动，Ⓐ 修改，Ⓑ 关闭。
 *
 * 焦点状态由组件自己持有（和手机端 `settings.handleIntent` 一样，
 * 设置面板打开时会先于主界面吃掉所有输入），宿主只负责把它接进意图分发。
 */
export const BigScreenSettingsLayer = forwardRef<
  BigScreenSettingsLayerHandle,
  BigScreenSettingsLayerProps
>(({ config, onChange, onClose, onOpenChoices, sections }, ref) => {
  const { t } = useTranslation();
  const [column, setColumn] = useState<Column>("section");
  const [sectionIndex, setSectionIndex] = useState(0);
  const [itemIndex, setItemIndex] = useState(0);

  const section = sections[sectionIndex];
  // useMemo：下面 runCurrent 的依赖里要有稳定引用，否则每次渲染都会重建成新函数
  const settings = useMemo(() => section?.settings ?? [], [section]);

  const step = (delta: number) => {
    if (column === "section") {
      if (sections.length === 0) {
        return;
      }
      const next = (sectionIndex + delta + sections.length) % sections.length;
      setSectionIndex(next);
      setItemIndex(0);
      return;
    }
    if (settings.length === 0) {
      return;
    }
    setItemIndex((settings.length + itemIndex + delta) % settings.length);
  };

  const runCurrent = useCallback(() => {
    const setting = settings[itemIndex];
    if (!setting) {
      return;
    }
    if (setting.kind === "action") {
      // 动作类（清除筛选记忆 / 恢复默认设置）直接执行，回调里自己保证安全性
      setting.run?.();
      return;
    }
    if (setting.kind === "switch") {
      // 布尔项就地翻转，不必再开一层浮层
      const current = setting.read?.(config) ?? "false";
      const next = setting.write?.(
        config,
        current === "true" ? "false" : "true",
      );
      if (next) {
        onChange(next);
      }
      return;
    }
    onOpenChoices(sectionIndex, itemIndex);
  }, [config, itemIndex, onChange, onOpenChoices, sectionIndex, settings]);

  useImperativeHandle(ref, () => ({
    handleIntent: (intent) => {
      switch (intent.type) {
        case "move": {
          if (intent.direction === "up" || intent.direction === "down") {
            step(intent.direction === "down" ? 1 : -1);
            return true;
          }
          if (intent.direction === "left") {
            setColumn("section");
            return true;
          }
          setColumn("item");
          return true;
        }
        case "confirm": {
          if (column === "section") {
            setColumn("item");
            setItemIndex(0);
            return true;
          }
          runCurrent();
          return true;
        }
        case "back": {
          onClose();
          return true;
        }
        default: {
          // 面板打开时吞掉其它输入（收藏 / 详情 / 切分类），避免误操作到下层
          return true;
        }
      }
    },
  }));

  return (
    <div className="absolute inset-0 z-50 flex flex-col bg-brand-950/92 backdrop-blur-xl">
      <div className="pointer-events-none absolute inset-0" onClick={onClose} />

      <div className="relative mx-auto flex min-h-0 w-full max-w-5xl flex-1 flex-col px-8 pt-10">
        <h2 className="mb-5 shrink-0 text-2xl font-bold text-white">
          {t("bigScreen.settingsTitle")}
        </h2>

        <div className="flex min-h-0 flex-1 gap-6">
          {/* 左列：分区 */}
          <div className="w-48 shrink-0 space-y-1">
            {sections.map((entry, index) => {
              const isActive = index === sectionIndex;
              const isFocused = column === "section" && isActive;
              return (
                <button
                  key={entry.id}
                  type="button"
                  onClick={() => {
                    setColumn("section");
                    setSectionIndex(index);
                    setItemIndex(0);
                  }}
                  onMouseEnter={() => {
                    setColumn("section");
                    setSectionIndex(index);
                  }}
                  className={`w-full rounded-xl px-3.5 py-2.5 text-left text-sm font-medium transition-all duration-150 ${
                    isFocused
                      ? "translate-x-1 bg-white/12 text-white ring-2 ring-secondary-500"
                      : isActive
                        ? "bg-white/6 text-secondary-500"
                        : "text-brand-400 hover:bg-white/4 hover:text-white"
                  }`}
                >
                  {entry.label}
                </button>
              );
            })}
          </div>

          {/* 右列：条目 */}
          <div className="min-h-0 flex-1 overflow-y-auto pb-6 pr-2">
            {settings.length === 0 && (
              <p className="text-sm text-brand-500">
                {t("bigScreen.settingsEmpty")}
              </p>
            )}
            {settings.map((setting, index) => {
              const isFocused = column === "item" && index === itemIndex;
              const value = formatBigScreenSettingValue(setting, config);
              return (
                <button
                  key={setting.id}
                  type="button"
                  onClick={() => {
                    setColumn("item");
                    setItemIndex(index);
                    runCurrent();
                  }}
                  onMouseEnter={() => {
                    setColumn("item");
                    setItemIndex(index);
                  }}
                  className={`mb-1 flex w-full items-center justify-between gap-6 rounded-xl px-3.5 py-2.5 text-left transition-all duration-150 ${
                    isFocused
                      ? "bg-white/12 ring-2 ring-secondary-500"
                      : "hover:bg-white/6"
                  }`}
                >
                  <span
                    className={`text-sm font-medium ${
                      isFocused ? "text-white" : "text-brand-200"
                    }`}
                  >
                    {setting.label}
                  </span>
                  <span
                    className={`shrink-0 text-sm ${
                      isFocused ? "text-secondary-500" : "text-brand-400"
                    }`}
                  >
                    {value}
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      </div>

      <div className="relative shrink-0 px-8 pb-5 pt-3 text-xs text-brand-500">
        {t("bigScreen.settingsHint")}
      </div>
    </div>
  );
});

BigScreenSettingsLayer.displayName = "BigScreenSettingsLayer";
