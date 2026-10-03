import type { appconf, service, vo } from "../../../src/bindings/models";
import { useCallback, useEffect, useRef, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  PreviewGameLibraryPathChange,
  SelectDirectory,
} from "../../../bindings/yukihub/internal/service/configservice";
import {
  GetOverlayShortcut,
  ResetOverlayShortcut,
  SetOverlayShortcut,
} from "../../../bindings/yukihub/internal/service/overlayservice";
import { appZoomOptions, languageOptions } from "../../consts/options";
import { GameLibraryPathChangeModal } from "../modal/GameLibraryPathChangeModal";
import { BetterActionInput } from "../ui/better/BetterActionInput";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterInput } from "../ui/better/BetterInput";
import { BetterSelect } from "../ui/better/BetterSelect";
import { BetterSwitch } from "../ui/better/BetterSwitch";
import { ShortcutRecorder } from "../ui/ShortcutRecorder";
import { BangumiAccountSettings } from "./BangumiAccountSettings";
import { NextMoeAccountSettings } from "./NextMoeAccountSettings";

interface BetterSelectOption {
  value: string;
  label: string;
}

type AccountProvider = "bangumi" | "nextmoe";

const ACCOUNT_CONTENT_FADE_MS = 100;
const ACCOUNT_CARD_RESIZE_MS = 180;
const ACCOUNT_CARD_RESIZE_BUFFER_MS = 32;

interface BasicSettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
  onZoomChange: (zoomFactor: number) => void;
  onConfigRefresh: () => Promise<void>;
  onGameLibraryPathApply: (
    newPath: string,
    syncPaths: boolean,
  ) => Promise<service.GameLibraryPathChangeResult>;
}

export function BasicSettingsPanel({
  formData,
  onChange,
  onZoomChange,
  onConfigRefresh,
  onGameLibraryPathApply,
}: BasicSettingsProps) {
  const { t } = useTranslation();
  const [expandedAccount, setExpandedAccount]
    = useState<AccountProvider | null>(null);
  const [isAccountContentVisible, setIsAccountContentVisible] = useState(true);
  const [pendingGameLibraryInput, setPendingGameLibraryInput] = useState<
    string | null
  >(null);
  const [libraryChangePreview, setLibraryChangePreview]
    = useState<service.GameLibraryPathChangePreview | null>(null);
  const [isLibraryPreviewLoading, setIsLibraryPreviewLoading] = useState(false);
  const [isLibraryChangeApplying, setIsLibraryChangeApplying] = useState(false);
  const accountContentTimerRef = useRef<number | null>(null);
  const gameLibraryInput
    = pendingGameLibraryInput ?? formData.game_library_path ?? "";

  const COMMON_TIMEZONES: BetterSelectOption[] = [
    { value: "Asia/Shanghai", label: "China Standard Time (UTC+8)" },
    { value: "Asia/Tokyo", label: "Japan Standard Time (UTC+9)" },
    { value: "Asia/Seoul", label: "Korea Standard Time (UTC+9)" },
    { value: "Asia/Hong_Kong", label: "Hong Kong Time (UTC+8)" },
    { value: "Asia/Taipei", label: "Taipei Time (UTC+8)" },
    { value: "Asia/Singapore", label: "Singapore Time (UTC+8)" },
    { value: "Asia/Bangkok", label: "Bangkok Time (UTC+7)" },
    { value: "Asia/Dubai", label: "Dubai Time (UTC+4)" },
    { value: "Europe/London", label: "London Time (UTC+0)" },
    { value: "Europe/Paris", label: "Paris Time (UTC+1)" },
    { value: "Europe/Berlin", label: "Berlin Time (UTC+1)" },
    { value: "Europe/Moscow", label: "Moscow Time (UTC+3)" },
    { value: "America/New_York", label: "New York Time (UTC-5)" },
    { value: "America/Chicago", label: "Chicago Time (UTC-6)" },
    { value: "America/Denver", label: "Denver Time (UTC-7)" },
    { value: "America/Los_Angeles", label: "Los Angeles Time (UTC-8)" },
    { value: "America/Sao_Paulo", label: "São Paulo Time (UTC-3)" },
    { value: "Australia/Sydney", label: "Sydney Time (UTC+10)" },
    { value: "Pacific/Auckland", label: "Auckland Time (UTC+12)" },
    { value: "UTC", label: "Coordinated Universal Time (UTC)" },
  ];

  /**
   * 展开某张账户卡片时的列宽。
   *
   * **必须始终是 2 列**：YukiHub 账号是「我们自己的登录」，入口在首页左上角
   * 用户区（对齐手机版），不再出现在这里的授权卡片里；本区只剩 Bangumi /
   * NextMoe 两个第三方授权。按「展开的那张占 60%、另一张占 40%」排在同一行。
   */
  const accountGridColumns
    = expandedAccount === "bangumi"
      ? "sm:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]"
      : "sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]";

  useEffect(() => {
    return () => {
      if (accountContentTimerRef.current !== null) {
        window.clearTimeout(accountContentTimerRef.current);
      }
    };
  }, []);

  // 呼出好友栏的全局快捷键。
  //
  // 它不由 formData 驱动：注册是后端的事，「有没有真的注册上」只有后端知道
  // （可能被别的程序占用），所以以后端返回的信息为准；表单里同步一份
  // accelerator，避免稍后防抖落盘时把新值覆盖回旧的。
  const [overlayShortcut, setOverlayShortcut]
    = useState<vo.OverlayShortcut | null>(null);

  useEffect(() => {
    void (async () => {
      try {
        setOverlayShortcut(await GetOverlayShortcut());
      }
      catch {
        // 拿不到就不显示控件内容，设置页其余部分照常用
      }
    })();
  }, []);

  const applyOverlayShortcutResult = useCallback(
    (info: vo.OverlayShortcut) => {
      setOverlayShortcut(info);
      onChange({
        ...formData,
        overlay_shortcut: info.accelerator,
      } as appconf.AppConfig);
      toast.success(
        t("settings.basic.overlayShortcutSaved", { shortcut: info.display }),
      );
    },
    [formData, onChange, t],
  );

  const handleOverlayShortcutRecord = async (accelerator: string) => {
    try {
      applyOverlayShortcutResult(await SetOverlayShortcut(accelerator));
    }
    catch (error) {
      toast.error(
        t("settings.basic.overlayShortcutFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
    }
  };

  const handleOverlayShortcutReset = async () => {
    try {
      applyOverlayShortcutResult(await ResetOverlayShortcut());
    }
    catch (error) {
      toast.error(
        t("settings.basic.overlayShortcutFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
    }
  };

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>,
  ) => {
    const { name, value, type } = e.target;
    const newValue
      = type === "checkbox" ? (e.target as HTMLInputElement).checked : value;
    onChange({ ...formData, [name]: newValue } as appconf.AppConfig);
  };

  const requestGameLibraryPathChange = async (
    newPath: string,
    saveDirectoryWhenEmpty: boolean,
  ) => {
    setIsLibraryPreviewLoading(true);
    try {
      const preview = await PreviewGameLibraryPathChange(newPath);

      if (preview.affected_game_count === 0) {
        setLibraryChangePreview(null);
        const isConfiguredDirectoryChange
          = newPath.trim() !== (formData.game_library_path ?? "").trim();
        if (!saveDirectoryWhenEmpty || !isConfiguredDirectoryChange) {
          toast.success(t("settings.basic.libraryChange.noAffectedRecords"));
          return;
        }

        setIsLibraryChangeApplying(true);
        try {
          await onGameLibraryPathApply(preview.new_configured_path, false);
          setPendingGameLibraryInput(null);
          toast.success(
            t("settings.basic.libraryChange.changeWithoutAffectedGamesSuccess"),
          );
        }
        catch (error) {
          console.error("Failed to apply game library path change:", error);
          toast.error(t("settings.basic.libraryChange.applyFailed"));
        }
        finally {
          setIsLibraryChangeApplying(false);
        }
        return;
      }

      setLibraryChangePreview(preview);
    }
    catch (error) {
      console.error("Failed to preview game library path change:", error);
      toast.error(t("settings.basic.libraryChange.previewFailed"));
    }
    finally {
      setIsLibraryPreviewLoading(false);
    }
  };

  const handleSelectGameLibraryPath = async () => {
    try {
      const path = await SelectDirectory(
        t("settings.basic.selectGameLibraryTitle"),
      );
      if (path) {
        setPendingGameLibraryInput(path);
        await requestGameLibraryPathChange(path, true);
      }
    }
    catch (error) {
      console.error("Failed to select game library path:", error);
      toast.error(t("settings.basic.selectGameLibraryFailed"));
    }
  };

  const handleCloseLibraryChange = () => {
    setLibraryChangePreview(null);
    setPendingGameLibraryInput(null);
  };

  const handleApplyLibraryChange = async (syncPaths: boolean) => {
    if (!libraryChangePreview) {
      return;
    }

    setIsLibraryChangeApplying(true);
    try {
      const result = await onGameLibraryPathApply(
        libraryChangePreview.new_configured_path,
        syncPaths,
      );
      setPendingGameLibraryInput(null);
      setLibraryChangePreview(null);
      toast.success(
        syncPaths
          ? t("settings.basic.libraryChange.changeSuccess", {
              games: result.updated_game_count,
            })
          : t("settings.basic.libraryChange.changeWithoutSyncSuccess"),
      );
    }
    catch (error) {
      console.error("Failed to apply game library path change:", error);
      toast.error(t("settings.basic.libraryChange.applyFailed"));
    }
    finally {
      setIsLibraryChangeApplying(false);
    }
  };

  const handleAccountExpand = (account: AccountProvider) => {
    if (account === expandedAccount || !isAccountContentVisible)
      return;

    const prefersReducedMotion
      = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
    if (prefersReducedMotion) {
      setExpandedAccount(account);
      return;
    }

    const hasGridAnimation
      = window.matchMedia?.("(min-width: 640px)").matches ?? false;
    setIsAccountContentVisible(false);
    accountContentTimerRef.current = window.setTimeout(() => {
      setExpandedAccount(account);
      accountContentTimerRef.current = window.setTimeout(
        () => {
          setIsAccountContentVisible(true);
          accountContentTimerRef.current = null;
        },
        hasGridAnimation
          ? ACCOUNT_CARD_RESIZE_MS + ACCOUNT_CARD_RESIZE_BUFFER_MS
          : 16,
      );
    }, ACCOUNT_CONTENT_FADE_MS);
  };

  const handleAccountGridTransitionEnd = (
    event: React.TransitionEvent<HTMLDivElement>,
  ) => {
    if (
      event.target !== event.currentTarget
      || event.propertyName !== "grid-template-columns"
      || isAccountContentVisible
    ) {
      return;
    }

    if (accountContentTimerRef.current !== null) {
      window.clearTimeout(accountContentTimerRef.current);
    }
    accountContentTimerRef.current = window.setTimeout(() => {
      setIsAccountContentVisible(true);
      accountContentTimerRef.current = null;
    }, 16);
  };

  return (
    <>
      <section className="space-y-2">
        <h3 className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
          {t("settings.basic.accountAuthorizationSectionLabel")}
        </h3>
        <div
          className={`account-choice-transition grid grid-cols-1 items-stretch gap-3 motion-reduce:transition-none ${accountGridColumns}`}
          role="group"
          aria-label={t("settings.basic.accountAuthorizationSectionLabel")}
          onTransitionEnd={handleAccountGridTransitionEnd}
        >
          <BangumiAccountSettings
            formData={formData}
            isContentVisible={isAccountContentVisible}
            isExpanded={expandedAccount === "bangumi"}
            onChange={onChange}
            onConfigRefresh={onConfigRefresh}
            onExpand={() => handleAccountExpand("bangumi")}
          />

          <NextMoeAccountSettings
            isContentVisible={isAccountContentVisible}
            isExpanded={expandedAccount === "nextmoe"}
            onConfigRefresh={onConfigRefresh}
            onExpand={() => handleAccountExpand("nextmoe")}
          />
        </div>
      </section>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          VNDB Access Token
        </label>
        <BetterInput
          type="text"
          name="vndb_access_token"
          value={formData.vndb_access_token || ""}
          onChange={handleChange}
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.themeLabel")}
        </label>
        <BetterSelect
          name="theme"
          value={formData.theme}
          onChange={value =>
            onChange({ ...formData, theme: value } as appconf.AppConfig)}
          options={[
            { value: "light", label: t("settings.basic.themeLight") },
            { value: "dark", label: t("settings.basic.themeDark") },
            { value: "system", label: t("settings.basic.themeSystem") },
          ]}
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.languageLabel")}
        </label>
        <BetterSelect
          name="language"
          value={formData.language}
          onChange={value =>
            onChange({ ...formData, language: value } as appconf.AppConfig)}
          options={languageOptions}
        />
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.zoomLabel")}
        </label>
        <BetterSelect
          name="window_zoom_factor"
          value={String(formData.window_zoom_factor || 1)}
          onChange={value => onZoomChange(Number(value))}
          options={appZoomOptions}
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.basic.zoomHint")}
        </p>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.timezoneLabel")}
        </label>
        <BetterSelect
          name="timezone"
          value={formData.time_zone || "Asia/Shanghai"}
          onChange={value =>
            onChange({ ...formData, time_zone: value } as appconf.AppConfig)}
          options={COMMON_TIMEZONES}
          placeholder={t("settings.basic.timezonePlaceholder")}
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.basic.timezoneHint")}
        </p>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.basic.gameLibraryPath")}
        </label>
        <BetterActionInput
          value={gameLibraryInput}
          disabled={isLibraryPreviewLoading || isLibraryChangeApplying}
          onChange={e => setPendingGameLibraryInput(e.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              void requestGameLibraryPathChange(gameLibraryInput, true);
            }
            else if (event.key === "Escape") {
              setPendingGameLibraryInput(null);
            }
          }}
          placeholder={t("settings.basic.gameLibraryPathPlaceholder")}
          className="text-sm"
          actions={[
            {
              ariaLabel: t("settings.basic.selectGameLibraryTitle"),
              icon: "i-mdi-folder-open-outline",
              onClick: handleSelectGameLibraryPath,
            },
            {
              ariaLabel: t("settings.basic.libraryChange.scanPaths"),
              icon: isLibraryPreviewLoading
                ? "i-mdi-loading animate-spin"
                : "i-mdi-refresh",
              onClick: () =>
                void requestGameLibraryPathChange(gameLibraryInput, false),
            },
          ]}
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.basic.gameLibraryPathHint")}
        </p>
      </div>

      <GameLibraryPathChangeModal
        preview={libraryChangePreview}
        isApplying={isLibraryChangeApplying}
        onClose={handleCloseLibraryChange}
        onApply={syncPaths => void handleApplyLibraryChange(syncPaths)}
      />

      <div className="space-y-2">
        <div className="flex items-center justify-between gap-4">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.basic.closeToTray")}
          </label>
          <BetterSwitch
            id="close_to_tray"
            checked={formData.close_to_tray || false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                close_to_tray: checked,
              } as appconf.AppConfig)}
          />
        </div>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between gap-4">
          <label
            htmlFor="launch_at_login"
            className="block cursor-pointer text-sm font-medium text-brand-700 dark:text-brand-300"
          >
            {t("settings.basic.launchAtLogin")}
          </label>
          <BetterSwitch
            id="launch_at_login"
            checked={formData.launch_at_login || false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                launch_at_login: checked,
              } as appconf.AppConfig)}
          />
        </div>
      </div>

      <div className="space-y-2">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.basic.overlayShortcutLabel")}
          </label>
          <div className="flex items-center gap-2">
            <ShortcutRecorder
              display={
                overlayShortcut?.active_display
                || overlayShortcut?.display
                || ""
              }
              onRecord={accelerator =>
                void handleOverlayShortcutRecord(accelerator)}
            />
            {overlayShortcut && !overlayShortcut.is_default ? (
              <BetterButton
                variant="ghost"
                size="sm"
                onClick={() => void handleOverlayShortcutReset()}
              >
                {t("settings.basic.overlayShortcutReset")}
              </BetterButton>
            ) : null}
          </div>
        </div>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.basic.overlayShortcutHint")}
        </p>
        {overlayShortcut && !overlayShortcut.active ? (
          <p className="text-xs text-amber-600 dark:text-amber-400">
            {t("settings.basic.overlayShortcutInactive")}
          </p>
        ) : null}
      </div>
    </>
  );
}
