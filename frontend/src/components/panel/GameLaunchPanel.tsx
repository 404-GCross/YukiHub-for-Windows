import type { appconf, models, service } from "../../../src/bindings/models";
import { useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import { enums } from "../../../src/bindings/models";
import { BetterActionInput } from "../ui/better/BetterActionInput";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterSelect } from "../ui/better/BetterSelect";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface GameLaunchPanelProps {
  game: models.Game;
  config?: appconf.AppConfig;
  onGameChange: (game: models.Game) => void;
  onLaunchModeChange: (mode: enums.LaunchMode) => void;
  onRefreshSteamSettings?: () => Promise<void>;
  onSaveSteamLaunchOptions?: (
    launchOptions: string,
  ) => Promise<service.SteamLaunchStatus | void>;
  onSelectProcessExecutable: () => void;
  onSelectRunningProcess: () => void;
  onExportShortcut: () => void;
}

type GameWithSteamLaunchOptions = models.Game & {
  steam_launch_options?: string;
};

const steamLaunchOptionPresets = [
  {
    key: "chineseLocale",
    value: "LANG=zh_CN.UTF-8 %command%",
  },
  {
    key: "fullChineseLocale",
    value: "LC_ALL=zh_CN.UTF-8 LANG=zh_CN.UTF-8 %command%",
  },
  {
    key: "protonLog",
    value: "PROTON_LOG=1 %command%",
  },
] as const;

function getSteamLaunchOptions(game: models.Game): string {
  return (game as GameWithSteamLaunchOptions).steam_launch_options || "";
}

function withSteamLaunchOptions(
  game: models.Game,
  steamLaunchOptions: string,
): models.Game {
  return {
    ...game,
    steam_launch_options: steamLaunchOptions,
  } as GameWithSteamLaunchOptions as models.Game;
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

export function GameLaunchPanel({
  game,
  config,
  onGameChange,
  onLaunchModeChange,
  onRefreshSteamSettings,
  onSaveSteamLaunchOptions,
  onSelectProcessExecutable,
  onSelectRunningProcess,
  onExportShortcut,
}: GameLaunchPanelProps) {
  const { t } = useTranslation();
  const hasLocaleEmulatorPath
    = config?.locale_emulator_path && config?.locale_emulator_path.length > 0;
  const hasMagpiePath = config?.magpie_path && config?.magpie_path.length > 0;
  const executableName = game.path
    ? game.path.split(/[\\/]/).pop()
    : t("gameLaunch.noPathSet");
  const launchModeOptions = [
    {
      value: enums.LaunchMode.LaunchModeNormal,
      label: t("gameLaunch.launchModeNormal"),
    },
    {
      value: enums.LaunchMode.LaunchModeAdmin,
      label: t("gameLaunch.launchModeAdmin"),
    },
    {
      value: enums.LaunchMode.LaunchModeSteam,
      label: t("gameLaunch.launchModeSteam"),
    },
  ];
  // 「兼容模式」是 macOS 的 Wine 启动，Windows 上不存在；
  // 历史数据若带入该值，统一回落为普通启动。
  const launchMode
    = game.launch_mode === enums.LaunchMode.LaunchModeCompatibility
      ? enums.LaunchMode.LaunchModeNormal
      : game.launch_mode || enums.LaunchMode.LaunchModeNormal;
  const isSteamLaunch = launchMode === enums.LaunchMode.LaunchModeSteam;
  const [isSteamLaunchOptionsSaving, setIsSteamLaunchOptionsSaving]
    = useState(false);
  const steamLaunchOptions = getSteamLaunchOptions(game);

  const handleRefreshSteamSettings = async () => {
    try {
      await onRefreshSteamSettings?.();
    }
    catch (error) {
      console.error("Failed to refresh Steam settings:", error);
    }
  };

  const handleSteamLaunchOptionsChange = (value: string) => {
    onGameChange(withSteamLaunchOptions(game, value));
  };

  const handleApplySteamLaunchOptionsPreset = (value: string) => {
    handleSteamLaunchOptionsChange(value);
  };

  const handleSaveSteamLaunchOptions = async () => {
    if (!onSaveSteamLaunchOptions || isSteamLaunchOptionsSaving) {
      return;
    }
    setIsSteamLaunchOptionsSaving(true);
    try {
      await onSaveSteamLaunchOptions(steamLaunchOptions);
      toast.success(t("gameLaunch.toast.steamLaunchOptionsSaved"));
    }
    catch (error) {
      toast.error(
        t("gameLaunch.toast.steamLaunchOptionsSaveFailed", {
          error: errorMessage(error),
        }),
      );
    }
    finally {
      setIsSteamLaunchOptionsSaving(false);
    }
  };

  const handleLocaleEmulatorToggle = (checked: boolean) => {
    if (checked && !hasLocaleEmulatorPath) {
      toast.error(t("gameLaunch.toast.lePathRequired"));
      return;
    }
    onGameChange({
      ...game,
      use_locale_emulator: checked,
    } as models.Game);
  };

  const handleMagpieToggle = (checked: boolean) => {
    if (checked && !hasMagpiePath) {
      toast.error(t("gameLaunch.toast.magpiePathRequired"));
      return;
    }
    onGameChange({ ...game, use_magpie: checked } as models.Game);
  };

  return (
    <div className="space-y-6">
      {/* Process Monitor */}
      <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg">
        <div className="space-y-6">
          <div className="border-brand-200 dark:border-brand-700">
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white">
              {t("gameLaunch.processMonitor")}
            </h3>
            <p className="text-sm text-brand-500 dark:text-brand-400 mt-1"></p>
          </div>

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              {t("gameLaunch.launchMode")}
            </label>
            <BetterSelect
              value={launchMode}
              options={launchModeOptions}
              onChange={value =>
                onLaunchModeChange(value as enums.LaunchMode)}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              {t("gameLaunch.executable")}
            </label>
            <div className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md bg-brand-50 dark:bg-brand-700 text-brand-900 dark:text-white font-mono break-all text-sm">
              {executableName}
            </div>
            <p className="mt-1 text-xs text-brand-500">
              {t("gameLaunch.executableHint")}
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300 mb-1">
              {t("gameLaunch.actualProcess")}
            </label>
            <div className="flex flex-col gap-2 sm:flex-row">
              <BetterActionInput
                value={game.process_name || ""}
                onChange={e =>
                  onGameChange({
                    ...game,
                    process_name: e.target.value,
                  } as models.Game)}
                className="font-mono"
                containerClassName="flex-1"
                actions={[
                  {
                    ariaLabel: t("gameLaunch.selectProcessFile"),
                    icon: "i-mdi-file-search-outline",
                    onClick: onSelectProcessExecutable,
                  },
                ]}
              />
              <BetterButton
                variant="secondary"
                icon="i-mdi-application-search-outline"
                onClick={onSelectRunningProcess}
              >
                {t("gameLaunch.selectRunningProcess")}
              </BetterButton>
            </div>
            <p className="mt-1 text-xs text-brand-500">
              {t("gameLaunch.processHint")}
            </p>
          </div>

          <div className="glass-panel rounded-xl border border-brand-200/80 bg-brand-50/70 p-4 dark:border-brand-700 dark:bg-brand-900/30">
            <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <div className="min-w-0">
                <p className="text-sm font-medium text-brand-800 dark:text-brand-200">
                  {t("gameLaunch.exportShortcut")}
                </p>
                <p className="mt-1 text-xs leading-relaxed text-brand-500 dark:text-brand-400">
                  {t("gameLaunch.exportShortcutHint")}
                </p>
              </div>
              <BetterButton
                variant="primary"
                icon="i-mdi-link-variant"
                onClick={onExportShortcut}
              >
                {t("gameLaunch.exportShortcut")}
              </BetterButton>
            </div>
          </div>
        </div>
      </div>

      {isSteamLaunch && (
        <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg">
          <div className="space-y-5">
            <div className="border-brand-200 dark:border-brand-700 pb-2">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h3 className="text-lg font-semibold text-brand-900 dark:text-white">
                    {t("gameLaunch.steamTools")}
                  </h3>
                  <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
                    {t("gameLaunch.steamToolsHint")}
                  </p>
                </div>
                <BetterButton
                  variant="ghost"
                  size="sm"
                  icon="i-mdi-refresh"
                  onClick={handleRefreshSteamSettings}
                  aria-label={t("gameLaunch.steamProtonRefresh")}
                />
              </div>
            </div>

            <div className="space-y-3">
              <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                {t("gameLaunch.steamLaunchOptions")}
              </label>
              <BetterActionInput
                value={steamLaunchOptions}
                onChange={e =>
                  handleSteamLaunchOptionsChange(e.target.value)}
                placeholder={t("gameLaunch.steamLaunchOptionsPlaceholder")}
                className="font-mono"
                actions={[
                  {
                    ariaLabel: t("gameLaunch.steamLaunchOptionsSave"),
                    icon: isSteamLaunchOptionsSaving
                      ? "i-mdi-loading animate-spin"
                      : "i-mdi-content-save-outline",
                    onClick: handleSaveSteamLaunchOptions,
                    disabled:
                      isSteamLaunchOptionsSaving
                      || !onSaveSteamLaunchOptions,
                  },
                ]}
              />
              <p className="text-xs leading-relaxed text-brand-500 dark:text-brand-400">
                {t("gameLaunch.steamLaunchOptionsHint")}
              </p>
              <div className="flex flex-wrap gap-2">
                {steamLaunchOptionPresets.map(preset => (
                  <BetterButton
                    key={preset.key}
                    variant="secondary"
                    size="sm"
                    icon="i-mdi-plus-circle-outline"
                    onClick={() =>
                      handleApplySteamLaunchOptionsPreset(preset.value)}
                  >
                    {t(
                      `gameLaunch.steamLaunchOptionsPresets.${preset.key}`,
                    )}
                  </BetterButton>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}

      <div className="glass-card bg-white dark:bg-brand-800 p-6 rounded-lg">
        <div className="space-y-6">
          <div className="border-brand-200 dark:border-brand-700 pb-2">
            <h3 className="text-lg font-semibold text-brand-900 dark:text-white">
              {t("gameLaunch.enhancementTools")}
            </h3>
          </div>

          {!isSteamLaunch && (
            <div className="flex items-center justify-between">
              <div className="min-w-0 pr-4">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-brand-700 dark:text-brand-300">
                    Locale Emulator
                  </span>
                </div>
                <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
                  {t("gameLaunch.leDesc")}
                </p>
                {!hasLocaleEmulatorPath && (
                  <p className="mt-1 flex items-center gap-1 text-xs text-error-500">
                    <div className="i-mdi-alert-circle text-sm shrink-0" />
                    <span>{t("gameLaunch.leNotConfigured")}</span>
                  </p>
                )}
              </div>
              <div className="shrink-0">
                <BetterSwitch
                  id="use_locale_emulator"
                  checked={game.use_locale_emulator || false}
                  onCheckedChange={handleLocaleEmulatorToggle}
                  disabled={!hasLocaleEmulatorPath}
                />
              </div>
            </div>
          )}

          <div className="flex items-center justify-between">
            <div className="min-w-0 pr-4">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-brand-700 dark:text-brand-300">
                  Magpie
                </span>
              </div>
              <p className="mt-1 text-xs text-brand-500 dark:text-brand-400">
                {t("gameLaunch.magpieDesc")}
              </p>
              {!hasMagpiePath && (
                <p className="mt-1 flex items-center gap-1 text-xs text-error-500">
                  <div className="i-mdi-alert-circle text-sm shrink-0" />
                  <span>{t("gameLaunch.magpieNotConfigured")}</span>
                </p>
              )}
            </div>
            <div className="shrink-0">
              <BetterSwitch
                id="use_magpie"
                checked={game.use_magpie || false}
                onCheckedChange={handleMagpieToggle}
                disabled={!hasMagpiePath}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
