import type { vo } from "../../../src/bindings/models";
import { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  GetAccountStatus,
  LoginAccount,
  LogoutAccount,
  RegisterAccount,
  ResetAccountPassword,
  SendAccountCode,
  SetAccountCloudSyncEnabled,
  SetAccountSharePlaying,
  SyncAccountNow,
  UpdateAccountNickname,
} from "../../../bindings/yukihub/internal/service/accountservice";
import { onWailsEvent } from "../../bindings/runtime";
import { ConfirmModal } from "../modal/ConfirmModal";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterInput } from "../ui/better/BetterInput";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface YukiHubAccountSettingsProps {
  isContentVisible: boolean;
  isExpanded: boolean;
  onConfigRefresh: () => Promise<void> | void;
  onExpand: () => void;
}

/** 展开后的三种表单形态 */
type FormMode = "login" | "register" | "reset";

/** 验证码冷却，与手机版 / 服务端一致（60 秒） */
const CODE_COOLDOWN_SEC = 60;

export function YukiHubAccountSettings({
  isContentVisible,
  isExpanded,
  onConfigRefresh,
  onExpand,
}: YukiHubAccountSettingsProps) {
  const { t } = useTranslation();

  const [status, setStatus] = useState<vo.AccountStatus | null>(null);
  const [mode, setMode] = useState<FormMode>("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [nickname, setNickname] = useState("");
  const [code, setCode] = useState("");
  const [codeCountdown, setCodeCountdown] = useState(0);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSyncing, setIsSyncing] = useState(false);
  const [isSendingCode, setIsSendingCode] = useState(false);
  const [showLogoutConfirm, setShowLogoutConfirm] = useState(false);
  const [isEditingNickname, setIsEditingNickname] = useState(false);
  const [nicknameDraft, setNicknameDraft] = useState("");

  const refreshStatus = async () => {
    try {
      setStatus(await GetAccountStatus());
    }
    catch (error) {
      console.error("Failed to load YukiHub account status:", error);
    }
  };

  useEffect(() => {
    void refreshStatus();
    // 后端在登录态变化 / 同步完成后会推事件，界面跟着刷新即可
    const off = onWailsEvent<vo.AccountStatus>(
      "yukihub-account:status-changed",
      next => setStatus(next),
    );
    return off;
  }, []);

  // 验证码倒计时
  useEffect(() => {
    if (codeCountdown <= 0) {
      return;
    }
    const timer = window.setTimeout(
      () => setCodeCountdown(value => value - 1),
      1000,
    );
    return () => window.clearTimeout(timer);
  }, [codeCountdown]);

  const loggedIn = Boolean(status?.logged_in);

  const handleSendCode = async () => {
    if (!email.trim()) {
      toast.error(t("settings.account.errorEmailRequired"));
      return;
    }
    setIsSendingCode(true);
    try {
      await SendAccountCode(
        email.trim(),
        mode === "reset" ? "reset" : "register",
      );
      setCodeCountdown(CODE_COOLDOWN_SEC);
      toast.success(t("settings.account.toastCodeSent"));
    }
    catch (error) {
      toast.error(
        t("settings.account.toastFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
    }
    finally {
      setIsSendingCode(false);
    }
  };

  const handleSubmit = async () => {
    setIsSubmitting(true);
    try {
      if (mode === "login") {
        await LoginAccount(email.trim(), password);
        toast.success(t("settings.account.toastLoggedIn"));
      }
      else if (mode === "register") {
        await RegisterAccount(
          email.trim(),
          password,
          nickname.trim(),
          code.trim(),
        );
        toast.success(t("settings.account.toastRegistered"));
      }
      else {
        await ResetAccountPassword(email.trim(), code.trim(), password);
        toast.success(t("settings.account.toastPasswordReset"));
        setMode("login");
      }
      setPassword("");
      setCode("");
      await refreshStatus();
      await onConfigRefresh();
    }
    catch (error) {
      toast.error(
        t("settings.account.toastFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
    }
    finally {
      setIsSubmitting(false);
    }
  };

  const handleSync = async () => {
    setIsSyncing(true);
    try {
      const result = await SyncAccountNow();
      // 刻意写成显式分支而不是动态键：i18n:clean 识别不出模板拼接的键，
      // 会把它们当成「没人用」直接删掉。
      if (result.action === "noop") {
        toast.success(t("settings.account.toastSyncUpToDate"));
      }
      else if (result.action === "downloaded") {
        toast.success(
          t("settings.account.toastSyncDownloaded", {
            games: result.games,
            sessions: result.sessions,
          }),
        );
      }
      else if (result.action === "merged") {
        toast.success(
          t("settings.account.toastSyncMerged", {
            games: result.games,
            sessions: result.sessions,
          }),
        );
      }
      else {
        toast.success(
          t("settings.account.toastSyncUploaded", {
            games: result.games,
            sessions: result.sessions,
          }),
        );
      }
      await refreshStatus();
      await onConfigRefresh();
    }
    catch (error) {
      toast.error(
        t("settings.account.toastSyncFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
    }
    finally {
      setIsSyncing(false);
    }
  };

  const handleLogout = async () => {
    setShowLogoutConfirm(false);
    try {
      await LogoutAccount();
      await refreshStatus();
      await onConfigRefresh();
      toast.success(t("settings.account.toastLoggedOut"));
    }
    catch (error) {
      toast.error(
        t("settings.account.toastFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
    }
  };

  const handleSaveNickname = async () => {
    const trimmed = nicknameDraft.trim();
    if (!trimmed) {
      return;
    }
    try {
      await UpdateAccountNickname(trimmed);
      setIsEditingNickname(false);
      await refreshStatus();
      await onConfigRefresh();
      toast.success(t("settings.account.toastNicknameUpdated"));
    }
    catch (error) {
      toast.error(
        t("settings.account.toastFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
    }
  };

  const accountName = status?.nickname || t("settings.account.name");
  const accountSummary = loggedIn
    ? `${status?.nickname || "-"}${status?.uid ? ` · UID ${status.uid}` : ""}`
    : t("settings.account.notLoggedIn");

  const submitLabel
    = mode === "login"
      ? t("settings.account.login")
      : mode === "register"
        ? t("settings.account.register")
        : t("settings.account.resetPassword");

  return (
    <>
      <div
        className={`glass-panel relative isolate min-h-[132px] min-w-0 overflow-hidden rounded-2xl border transition-colors duration-200 sm:h-[190px] lg:h-[160px] ${
          isExpanded
            ? "border-brand-300/90 bg-brand-50/70 shadow-sm dark:border-brand-600/90 dark:bg-brand-900/35"
            : "border-brand-200/80 bg-white/55 hover:border-brand-300/80 dark:border-brand-700/80 dark:bg-brand-900/25 dark:hover:border-brand-600/80"
        }`}
      >
        <div
          aria-hidden="true"
          className="pointer-events-none absolute bottom-4 right-4 z-0 h-24 w-24 rounded-full bg-gradient-to-br from-brand-300/40 to-brand-500/20 blur-xl"
        />

        {isExpanded ? (
          <div
            className={`account-choice-content-transition relative z-10 flex h-full flex-col gap-3 overflow-y-auto p-3 motion-reduce:transition-none ${
              isContentVisible ? "opacity-100" : "pointer-events-none opacity-0"
            }`}
          >
            {loggedIn ? (
              <div className="flex flex-col gap-3">
                <div className="flex items-center gap-3">
                  {status?.avatar ? (
                    <img
                      src={status.avatar}
                      alt=""
                      className="h-12 w-12 shrink-0 rounded-2xl border border-brand-200/70 object-cover shadow-sm dark:border-brand-700/70"
                    />
                  ) : (
                    <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-brand-200/80 text-sm font-semibold text-brand-700 dark:bg-brand-700/80 dark:text-brand-200">
                      {(status?.nickname || "Y").slice(0, 1).toUpperCase()}
                    </div>
                  )}

                  <div className="min-w-0 flex-1 space-y-1">
                    {isEditingNickname ? (
                      <div className="flex items-center gap-2">
                        <BetterInput
                          value={nicknameDraft}
                          onChange={e => setNicknameDraft(e.target.value)}
                          placeholder={t(
                            "settings.account.nicknamePlaceholder",
                          )}
                          fullWidth
                        />
                        <BetterButton
                          variant="primary"
                          size="sm"
                          icon="i-mdi-check"
                          onClick={() => void handleSaveNickname()}
                        />
                        <BetterButton
                          variant="secondary"
                          size="sm"
                          icon="i-mdi-close"
                          onClick={() => setIsEditingNickname(false)}
                        />
                      </div>
                    ) : (
                      <div className="flex items-center gap-2">
                        <span className="truncate text-sm font-semibold text-brand-800 dark:text-brand-100">
                          {accountSummary}
                        </span>
                        <BetterButton
                          variant="secondary"
                          size="sm"
                          icon="i-mdi-pencil-outline"
                          aria-label={t("settings.account.editNickname")}
                          onClick={() => {
                            setNicknameDraft(status?.nickname ?? "");
                            setIsEditingNickname(true);
                          }}
                        />
                      </div>
                    )}
                    {status?.email && (
                      <div className="truncate text-xs text-brand-500 dark:text-brand-400">
                        {status.email}
                      </div>
                    )}
                  </div>

                  <BetterButton
                    variant="danger"
                    size="sm"
                    icon="i-mdi-logout"
                    className="!rounded-full"
                    aria-label={t("settings.account.logout")}
                    onClick={() => setShowLogoutConfirm(true)}
                  />
                </div>

                <div className="space-y-2 rounded-xl bg-white/60 p-3 dark:bg-brand-900/40">
                  <div className="flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <div className="text-xs font-medium text-brand-700 dark:text-brand-200">
                        {t("settings.account.cloudSync")}
                      </div>
                      <div className="text-[11px] text-brand-500 dark:text-brand-400">
                        {t("settings.account.cloudSyncHint")}
                      </div>
                    </div>
                    <BetterSwitch
                      id="yukihub-account-cloud-sync"
                      checked={Boolean(status?.cloud_sync_enabled)}
                      onCheckedChange={checked =>
                        void (async () => {
                          await SetAccountCloudSyncEnabled(checked);
                          await refreshStatus();
                          await onConfigRefresh();
                        })()}
                    />
                  </div>

                  <div className="flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <div className="text-xs font-medium text-brand-700 dark:text-brand-200">
                        {t("settings.account.sharePlaying")}
                      </div>
                      <div className="text-[11px] text-brand-500 dark:text-brand-400">
                        {t("settings.account.sharePlayingHint")}
                      </div>
                    </div>
                    <BetterSwitch
                      id="yukihub-account-share-playing"
                      checked={Boolean(status?.share_playing)}
                      onCheckedChange={checked =>
                        void (async () => {
                          await SetAccountSharePlaying(checked);
                          await refreshStatus();
                          await onConfigRefresh();
                        })()}
                    />
                  </div>

                  <div className="flex items-center justify-between gap-3 pt-1">
                    <span className="min-w-0 truncate text-[11px] text-brand-500 dark:text-brand-400">
                      {status?.last_sync_at
                        ? t("settings.account.lastSync", {
                            time: status.last_sync_at,
                          })
                        : t("settings.account.neverSynced")}
                    </span>
                    <BetterButton
                      variant="primary"
                      size="sm"
                      icon="i-mdi-cloud-sync-outline"
                      isLoading={isSyncing}
                      onClick={() => void handleSync()}
                    >
                      {t("settings.account.syncNow")}
                    </BetterButton>
                  </div>
                </div>
              </div>
            ) : (
              <div className="flex flex-col gap-2">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-semibold text-brand-800 dark:text-brand-100">
                    {mode === "login"
                      ? t("settings.account.loginTitle")
                      : mode === "register"
                        ? t("settings.account.registerTitle")
                        : t("settings.account.resetTitle")}
                  </span>
                  <div className="ml-auto flex items-center gap-2 text-[11px]">
                    {mode !== "login" && (
                      <button
                        type="button"
                        className="text-brand-500 underline-offset-2 hover:underline dark:text-brand-400"
                        onClick={() => setMode("login")}
                      >
                        {t("settings.account.backToLogin")}
                      </button>
                    )}
                    {mode === "login" && (
                      <>
                        <button
                          type="button"
                          className="text-brand-500 underline-offset-2 hover:underline dark:text-brand-400"
                          onClick={() => setMode("register")}
                        >
                          {t("settings.account.register")}
                        </button>
                        <span className="text-brand-300">|</span>
                        <button
                          type="button"
                          className="text-brand-500 underline-offset-2 hover:underline dark:text-brand-400"
                          onClick={() => setMode("reset")}
                        >
                          {t("settings.account.forgotPassword")}
                        </button>
                      </>
                    )}
                  </div>
                </div>

                <div className="flex flex-col gap-2 sm:flex-row">
                  <BetterInput
                    type="email"
                    value={email}
                    onChange={e => setEmail(e.target.value)}
                    placeholder={t("settings.account.emailPlaceholder")}
                    fullWidth
                  />
                  {mode !== "login" && (
                    <div className="flex shrink-0 gap-2">
                      <BetterInput
                        value={code}
                        onChange={e => setCode(e.target.value)}
                        placeholder={t("settings.account.codePlaceholder")}
                      />
                      <BetterButton
                        variant="secondary"
                        size="sm"
                        isLoading={isSendingCode}
                        disabled={codeCountdown > 0}
                        onClick={() => void handleSendCode()}
                      >
                        {codeCountdown > 0
                          ? t("settings.account.resendIn", {
                              seconds: codeCountdown,
                            })
                          : t("settings.account.sendCode")}
                      </BetterButton>
                    </div>
                  )}
                </div>

                <div className="flex flex-col gap-2 sm:flex-row">
                  {mode === "register" && (
                    <BetterInput
                      value={nickname}
                      onChange={e => setNickname(e.target.value)}
                      placeholder={t("settings.account.nicknamePlaceholder")}
                      fullWidth
                    />
                  )}
                  <BetterInput
                    type="password"
                    value={password}
                    onChange={e => setPassword(e.target.value)}
                    placeholder={t("settings.account.passwordPlaceholder")}
                    fullWidth
                  />
                  <BetterButton
                    variant="primary"
                    isLoading={isSubmitting}
                    onClick={() => void handleSubmit()}
                  >
                    {submitLabel}
                  </BetterButton>
                </div>

                <p className="text-[11px] leading-relaxed text-brand-500 dark:text-brand-400">
                  {t("settings.account.formHint")}
                </p>
              </div>
            )}
          </div>
        ) : (
          <button
            type="button"
            onClick={onExpand}
            className="relative z-10 flex h-full w-full flex-col items-start justify-between p-3 text-left"
          >
            <div className="flex items-center gap-2">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-brand-500/15 text-lg text-brand-600 dark:text-brand-300">
                <div className="i-mdi-account-circle-outline" />
              </div>
              <span className="text-sm font-semibold text-brand-800 dark:text-brand-100">
                {accountName}
              </span>
            </div>
            <div className="min-w-0">
              <div className="truncate text-xs text-brand-600 dark:text-brand-300">
                {accountSummary}
              </div>
              <div className="mt-1 text-[11px] text-brand-400 dark:text-brand-500">
                {loggedIn
                  ? t("settings.account.collapsedLoggedIn")
                  : t("settings.account.collapsedHint")}
              </div>
            </div>
          </button>
        )}
      </div>

      <ConfirmModal
        isOpen={showLogoutConfirm}
        title={t("settings.account.logoutConfirmTitle")}
        message={t("settings.account.logoutConfirmMsg")}
        confirmText={t("settings.account.logout")}
        type="danger"
        onConfirm={() => void handleLogout()}
        onClose={() => setShowLogoutConfirm(false)}
      />
    </>
  );
}
