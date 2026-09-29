import type { appconf } from "../../../src/bindings/models";
import type { BigScreenCategoryId } from "../../bigscreen/categories";
import { useTranslation } from "react-i18next";
import { BIG_SCREEN_CATEGORIES } from "../../bigscreen/categories";
import { BetterSelect } from "../ui/better/BetterSelect";
import { SettingSwitchRow } from "../ui/SettingSwitchRow";

interface BigScreenSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function BigScreenSettingsPanel({
  formData,
  onChange,
}: BigScreenSettingsProps) {
  const { t } = useTranslation();

  const defaultCategory: BigScreenCategoryId = BIG_SCREEN_CATEGORIES.some(
    category => category.id === formData.bigscreen_default_category,
  )
    ? (formData.bigscreen_default_category as BigScreenCategoryId)
    : "recent";

  return (
    <div className="space-y-4">
      <SettingSwitchRow
        id="bigscreen_show_hidden_game"
        label={t("settings.bigScreen.showHiddenGame")}
        hint={t("settings.bigScreen.showHiddenGameHint")}
        checked={formData.bigscreen_show_hidden_game || false}
        onCheckedChange={checked =>
          onChange({
            ...formData,
            bigscreen_show_hidden_game: checked,
          } as appconf.AppConfig)}
      />

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.bigScreen.defaultCategory")}
        </label>
        <BetterSelect
          value={defaultCategory}
          onChange={value =>
            onChange({
              ...formData,
              bigscreen_default_category: value,
            } as appconf.AppConfig)}
          options={BIG_SCREEN_CATEGORIES.map(category => ({
            value: category.id,
            label: t(category.labelKey),
          }))}
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.bigScreen.defaultCategoryHint")}
        </p>
      </div>
    </div>
  );
}
