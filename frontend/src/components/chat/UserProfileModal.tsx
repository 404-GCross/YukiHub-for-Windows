import type { UserProfile } from "../../../bindings/yukihub/internal/service/yukihubaccount/models";

import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { GetUserProfile } from "../../../bindings/yukihub/internal/service/accountservice";
import { ModalPortal } from "../ui/ModalPortal";
import { ChatAvatar } from "./ChatAvatar";

interface UserProfileModalProps {
  isOpen: boolean;
  uid: number;
  onClose: () => void;
}

/** 秒 → 「12 小时 34 分钟」，与手机版资料页的时长展示口径一致。 */
function formatPlayTime(seconds: number): string {
  if (!seconds || seconds <= 0) {
    return "-";
  }
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (hours <= 0) {
    return `${minutes} 分钟`;
  }
  return minutes > 0 ? `${hours} 小时 ${minutes} 分钟` : `${hours} 小时`;
}

/**
 * 用户资料弹窗（对齐手机版 renderUserProfile 的精简版）。
 *
 * 群聊里点别人的头像 / 消息菜单里选「查看资料」都会走到这里。
 */
export function UserProfileModal({
  isOpen,
  uid,
  onClose,
}: UserProfileModalProps) {
  const { t } = useTranslation();
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOpen || uid <= 0) {
      return;
    }
    let cancelled = false;
    setIsLoading(true);
    setError(null);
    setProfile(null);
    GetUserProfile(uid)
      .then((result) => {
        if (!cancelled) {
          setProfile(result);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setIsLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, uid]);

  if (!isOpen) {
    return null;
  }

  const statusLabel = (() => {
    switch (profile?.status) {
      case "online":
        return t("friendsChat.status.online");
      case "away":
        return t("friendsChat.status.away");
      case "busy":
        return t("friendsChat.status.busy");
      default:
        return t("friendsChat.status.offline");
    }
  })();

  return (
    <ModalPortal>
      <div
        className="fixed inset-0 z-[70] flex items-center justify-center bg-black/45 p-4 backdrop-blur-sm"
        onClick={onClose}
      >
        <div
          role="dialog"
          aria-modal="true"
          className="w-full max-w-sm overflow-hidden rounded-2xl border border-brand-200/90 bg-white shadow-2xl dark:border-brand-700/90 dark:bg-brand-800"
          onClick={event => event.stopPropagation()}
        >
          <div className="flex items-center justify-between border-b border-brand-200/80 px-4 py-3 dark:border-brand-700/80">
            <h2 className="flex items-center gap-2 text-sm font-bold text-brand-900 dark:text-white">
              <span className="i-mdi-account-circle-outline text-lg text-primary-500" />
              {t("friendsChat.profileTitle")}
            </h2>
            <button
              type="button"
              onClick={onClose}
              aria-label={t("common.close")}
              className="rounded-lg p-1.5 text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-700 dark:text-brand-400 dark:hover:bg-brand-700"
            >
              <span className="i-mdi-close text-lg" />
            </button>
          </div>

          <div className="p-4">
            {isLoading && (
              <p className="py-8 text-center text-sm text-brand-500 dark:text-brand-400">
                {t("friendsChat.loading")}
              </p>
            )}
            {error && (
              <p className="py-8 text-center text-sm text-error-600 dark:text-error-400">
                {error}
              </p>
            )}
            {profile && !isLoading && (
              <div className="flex flex-col gap-4">
                <div className="flex items-center gap-4">
                  <ChatAvatar
                    name={profile.nickname || `UID ${profile.uid}`}
                    avatar={profile.avatar}
                    size={64}
                    frame={profile.frame}
                  />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate text-base font-bold text-brand-900 dark:text-white">
                        {profile.nickname || `UID ${profile.uid}`}
                      </span>
                      {profile.status === "online" && (
                        <span className="shrink-0 rounded-full bg-success-500/15 px-2 py-0.5 text-[10px] font-semibold text-success-600 dark:text-success-400">
                          {statusLabel}
                        </span>
                      )}
                    </div>
                    <p className="mt-0.5 text-xs text-brand-500 dark:text-brand-400">
                      UID
                      {" "}
                      {profile.uid}
                    </p>
                    {profile.activity && (
                      <p className="mt-0.5 truncate text-xs text-primary-600 dark:text-primary-300">
                        {profile.activity}
                      </p>
                    )}
                  </div>
                </div>

                {profile.signature && (
                  <p className="whitespace-pre-wrap break-words rounded-lg bg-brand-50 p-3 text-xs leading-relaxed text-brand-700 dark:bg-brand-700/50 dark:text-brand-200">
                    {profile.signature}
                  </p>
                )}

                <div className="grid grid-cols-2 gap-3">
                  <div className="rounded-lg border border-brand-200/80 p-3 dark:border-brand-700/80">
                    <div className="text-[11px] text-brand-500 dark:text-brand-400">
                      {t("friendsChat.profileGames")}
                    </div>
                    <div className="mt-0.5 text-sm font-bold text-brand-900 dark:text-white">
                      {profile.totalGames > 0 ? profile.totalGames : "-"}
                    </div>
                  </div>
                  <div className="rounded-lg border border-brand-200/80 p-3 dark:border-brand-700/80">
                    <div className="text-[11px] text-brand-500 dark:text-brand-400">
                      {t("friendsChat.profilePlayTime")}
                    </div>
                    <div className="mt-0.5 text-sm font-bold text-brand-900 dark:text-white">
                      {formatPlayTime(profile.totalPlayTime)}
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
