import type { appconf } from "../../../src/bindings/models";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { createBigScreenSettingSections } from "../../bigscreen/settingsSchema";
import { BetterSelect } from "../ui/better/BetterSelect";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface BigScreenSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

/**
 * 设置页里的「大屏模式」分区。
 *
 * 条目来自 `bigscreen/settingsSchema` —— 与**大屏内的设置面板**（按 ☰ → 设置，
 * 对齐手机端 `BigScreenSettings`）共用同一份 schema，所以两处的可调项永远一致，
 * 新增一项不会只改了一边。这里用设置页惯用的 BetterSelect / BetterSwitch 渲染。
 */
export function BigScreenSettingsPanel({
  formData,
  onChange,
}: BigScreenSettingsProps) {
  const { t } = useTranslation();
  const sections = useMemo(() => createBigScreenSettingSections(t), [t]);

  return (
    <div className="space-y-6">
      {sections.map(section => (
        <div key={section.id} className="space-y-4">
          <h4 className="text-xs font-semibold uppercase tracking-wide text-brand-500 dark:text-brand-400">
            {section.label}
          </h4>

          {section.settings.map((setting) => {
            const current = setting.read(formData);

            if (setting.kind === "switch") {
              return (
                <div
                  key={setting.id}
                  className="flex items-center justify-between gap-4"
                >
                  <label
                    htmlFor={`bigscreen_${setting.id}`}
                    className="flex-1 cursor-pointer text-sm font-medium text-brand-700 dark:text-brand-300"
                  >
                    {setting.label}
                  </label>
                  <BetterSwitch
                    id={`bigscreen_${setting.id}`}
                    checked={current === "true"}
                    onCheckedChange={checked =>
                      onChange(
                        setting.write(formData, checked ? "true" : "false"),
                      )}
                  />
                </div>
              );
            }

            return (
              <div key={setting.id} className="flex items-center gap-4">
                <label className="flex-1 text-sm font-medium text-brand-700 dark:text-brand-300">
                  {setting.label}
                </label>
                <BetterSelect
                  value={current}
                  onChange={value => onChange(setting.write(formData, value))}
                  options={(setting.choices ?? []).map(choice => ({
                    label: choice.label,
                    value: choice.value,
                  }))}
                  className="w-40 shrink-0"
                />
              </div>
            );
          })}
        </div>
      ))}
    </div>
  );
}
