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
  StartQuickLogin,
  SyncAccountNow,
  UpdateAccountNickname,
} from "../../../bindings/yukihub/internal/service/accountservice";
import hikarinagiLoginIconUrl from "../../assets/providers/hikarinagi-login.jpg";
import nextmoeLogoUrl from "../../assets/providers/nextmoe-logo.webp";
import { onWailsEvent } from "../../bindings/runtime";
import { SnowflakeMark } from "../branding/SnowflakeMark";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterInput } from "../ui/better/BetterInput";
import { BetterSwitch } from "../ui/better/BetterSwitch";
import { ModalPortal } from "../ui/ModalPortal";
import { ConfirmModal } from "./ConfirmModal";

interface AccountModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfigRefresh: () => Promise<void> | void;
}

/** 登录表单的三种形态 */
type FormMode = "login" | "register" | "reset";

/** 验证码冷却，与手机版 / 服务端一致（60 秒） */
const CODE_COOLDOWN_SEC = 60;

/**
 * YukiHub 账号弹窗。
 *
 * 这是我们自己的账号体系（不是第三方授权），入口在首页左上角的用户区，
 * 与手机版一致：点击头像 / 用户名进来的就是这里。
 */
export function AccountModal({
  isOpen,
  onClose,
  onConfigRefresh,
}: AccountModalProps) {
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
  const [quickLoginProvider, setQuickLoginProvider] = useState<string | null>(
    null,
  );

  const refreshStatus = async () => {
    try {
      setStatus(await GetAccountStatus());
    }
    catch (error) {
      console.error("Failed to load YukiHub account status:", error);
    }
  };

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    void refreshStatus();
    const off = onWailsEvent<vo.AccountStatus>(
      "yukihub-account:status-changed",
      next => setStatus(next),
    );
    return off;
  }, [isOpen]);

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

  // 弹窗关闭时清空表单态，下次打开是干净的画面
  useEffect(() => {
    if (!isOpen) {
      setMode("login");
      setPassword("");
      setCode("");
      setIsEditingNickname(false);
    }
  }, [isOpen]);

  if (!isOpen) {
    return null;
  }

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

  /** 第三方快捷登录（鲲站 / Hikarinagi），流程与手机版一致 */
  const handleQuickLogin = async (provider: string) => {
    if (quickLoginProvider) {
      return;
    }
    setQuickLoginProvider(provider);
    try {
      await StartQuickLogin(provider);
      toast.success(t("settings.account.toastLoggedIn"));
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
      setQuickLoginProvider(null);
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
    <ModalPortal>
      <div
        className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
        onClick={onClose}
      >
        <div
          className="max-h-[86vh] w-full max-w-lg overflow-y-auto rounded-2xl border border-brand-200 bg-white p-5 shadow-2xl dark:border-brand-700 dark:bg-brand-800"
          onClick={e => e.stopPropagation()}
        >
          {/* 标题栏 */}
          <div className="mb-4 flex items-center justify-between gap-3">
            <h2 className="flex items-center gap-2 text-base font-bold text-brand-900 dark:text-white">
              {/* 用自家应用图标（雪花标），而不是通用的人像图标 */}
              <SnowflakeMark className="h-5 w-5 shrink-0 text-primary-500" />
              {t("settings.account.name")}
            </h2>
            <button
              type="button"
              onClick={onClose}
              aria-label={t("common.close")}
              className="rounded-lg p-1.5 text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-700 dark:text-brand-400 dark:hover:bg-brand-700 dark:hover:text-brand-200"
            >
              <span className="i-mdi-close text-lg" />
            </button>
          </div>

          {loggedIn ? (
            <div className="flex flex-col gap-4">
              <div className="flex items-center gap-3">
                {status?.avatar ? (
                  <img
                    src={status.avatar}
                    alt=""
                    className="h-14 w-14 shrink-0 rounded-2xl border border-brand-200/70 object-cover shadow-sm dark:border-brand-700/70"
                  />
                ) : (
                  <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-primary-500/15 text-lg font-semibold text-primary-600 dark:text-primary-300">
                    {(status?.nickname || "Y").slice(0, 1).toUpperCase()}
                  </div>
                )}

                <div className="min-w-0 flex-1 space-y-1">
                  {isEditingNickname ? (
                    <div className="flex items-center gap-2">
                      <BetterInput
                        value={nicknameDraft}
                        onChange={e => setNicknameDraft(e.target.value)}
                        placeholder={t("settings.account.nicknamePlaceholder")}
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

              <div className="space-y-2 rounded-xl bg-brand-50 p-3 dark:bg-brand-900/40">
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
            <div className="flex flex-col gap-3">
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

              <div className="flex flex-col gap-2">
                <BetterInput
                  type="email"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  placeholder={t("settings.account.emailPlaceholder")}
                  fullWidth
                />
                {mode !== "login" && (
                  <div className="flex gap-2">
                    <BetterInput
                      value={code}
                      onChange={e => setCode(e.target.value)}
                      placeholder={t("settings.account.codePlaceholder")}
                      fullWidth
                    />
                    <BetterButton
                      variant="secondary"
                      className="shrink-0"
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

              {/* 第三方快捷登录，与手机版一致的两条通道 */}
              <div className="flex flex-col gap-2 border-t border-brand-200/70 pt-3 dark:border-brand-700/70">
                <span className="text-[11px] text-brand-500 dark:text-brand-400">
                  {t("settings.account.quickLoginLabel")}
                </span>
                <div className="grid grid-cols-2 gap-2">
                  {/* 对齐手机版：按钮左侧放对应网站的 logo（28dp），文案用平台名 */}
                  <button
                    type="button"
                    className={`flex items-center justify-center gap-2 rounded-lg border border-brand-200/80 px-3 py-2 text-sm font-medium text-brand-700 transition-colors hover:bg-brand-100 disabled:opacity-60 dark:border-brand-700/80 dark:text-brand-200 dark:hover:bg-brand-700/70 ${
                      quickLoginProvider === "kungal" ? "opacity-60" : ""
                    }`}
                    disabled={quickLoginProvider !== null}
                    onClick={() => void handleQuickLogin("kungal")}
                  >
                    {quickLoginProvider === "kungal" ? (
                      <span className="i-mdi-loading h-5 w-5 animate-spin" />
                    ) : (
                      <img
                        src={nextmoeLogoUrl}
                        alt=""
                        className="h-7 w-7 rounded-full object-cover"
                      />
                    )}
                    {t("settings.account.quickLoginKungal")}
                  </button>
                  <button
                    type="button"
                    className={`flex items-center justify-center gap-2 rounded-lg border border-brand-200/80 px-3 py-2 text-sm font-medium text-brand-700 transition-colors hover:bg-brand-100 disabled:opacity-60 dark:border-brand-700/80 dark:text-brand-200 dark:hover:bg-brand-700/70 ${
                      quickLoginProvider === "hikarinagi" ? "opacity-60" : ""
                    }`}
                    disabled={quickLoginProvider !== null}
                    onClick={() => void handleQuickLogin("hikarinagi")}
                  >
                    {quickLoginProvider === "hikarinagi" ? (
                      <span className="i-mdi-loading h-5 w-5 animate-spin" />
                    ) : (
                      <img
                        src={hikarinagiLoginIconUrl}
                        alt=""
                        className="h-7 w-7 rounded-full object-cover"
                      />
                    )}
                    {t("settings.account.quickLoginHikarinagi")}
                  </button>
                </div>
              </div>

              <p className="text-[11px] leading-relaxed text-brand-500 dark:text-brand-400">
                {t("settings.account.formHint")}
              </p>
            </div>
          )}
        </div>
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
    </ModalPortal>
  );
}
