import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useAccountStatus } from "../../hooks/useAccountStatus";
import { AccountModal } from "../modal/AccountModal";

/**
 * 首页左上角的用户区，与手机版 HomeActivity.refreshProfileHeader 对齐：
 *
 * - 头像（云端头像，没有则显示昵称首字母）+ 在线状态点
 *   （登录=绿色在线点；未登录=灰色本地点）
 * - 问候语：按时段问候 + 昵称（手机版是 `period + "，" + name + " › " + tip`，
 *   桌面端窄一点，只放问候 + 名字；名字优先取云端昵称）
 * - 未登录时整块是「登录 YukiHub 账号」的入口；点击弹出账号面板
 */
export function HomeUserProfile() {
  const { t } = useTranslation();
  const status = useAccountStatus();
  const [accountOpen, setAccountOpen] = useState(false);
  const [greeting, setGreeting] = useState("");

  const loggedIn = Boolean(status?.logged_in);
  const displayName
    = loggedIn && status?.nickname ? status.nickname : status?.nickname || "Yuki";

  // 问候语按小时分段，与手机版一致（夜深了/早上好/中午好/下午好/晚上好）
  useEffect(() => {
    const update = () => {
      const hour = new Date().getHours();
      setGreeting(
        hour < 5
          ? t("home.greeting.night")
          : hour < 11
            ? t("home.greeting.morning")
            : hour < 14
              ? t("home.greeting.noon")
              : hour < 18
                ? t("home.greeting.afternoon")
                : t("home.greeting.evening"),
      );
    };
    update();
    const timer = window.setInterval(update, 60_000);
    return () => window.clearInterval(timer);
  }, [t]);

  const initial = (displayName || "Y").trim().slice(0, 1).toUpperCase() || "Y";

  return (
    <>
      <button
        type="button"
        onClick={() => setAccountOpen(true)}
        className="group flex min-w-0 items-center gap-2.5 rounded-xl p-1.5 text-left transition-colors hover:bg-white/45 dark:hover:bg-white/8 data-glass:hover:bg-white/12"
        aria-label={loggedIn ? t("home.openAccount") : t("home.loginPrompt")}
      >
        <div className="relative shrink-0">
          {status?.avatar ? (
            <img
              src={status.avatar}
              alt=""
              className="h-10 w-10 rounded-full border border-white/60 object-cover shadow-sm dark:border-white/15"
            />
          ) : (
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary-500/85 text-sm font-bold text-white shadow-sm">
              {initial}
            </div>
          )}
          {/* 在线状态点：登录绿点，未登录灰点（手机版 bg_profile_dot_online/local） */}
          <span
            className={`absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full border-2 border-white dark:border-brand-900 ${
              loggedIn ? "bg-emerald-500" : "bg-brand-400 dark:bg-brand-500"
            }`}
          />
        </div>

        <div className="min-w-0">
          <h1 className="truncate text-base font-bold leading-tight text-brand-900 dark:text-white">
            {loggedIn ? displayName : "YukiHub"}
          </h1>
          <p className="truncate text-xs text-brand-600 dark:text-white/80">
            {loggedIn
              ? `${greeting}，${t("home.welcomeBack")}`
              : t("home.loginPrompt")}
          </p>
        </div>

        <span className="i-mdi-chevron-right shrink-0 text-brand-400 opacity-0 transition-opacity group-hover:opacity-100 dark:text-white/50" />
      </button>

      <AccountModal
        isOpen={accountOpen}
        onClose={() => setAccountOpen(false)}
        onConfigRefresh={() => Promise.resolve()}
      />
    </>
  );
}
