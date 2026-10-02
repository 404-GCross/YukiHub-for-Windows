import type { UserProfile } from "../../../bindings/yukihub/internal/service/yukihubaccount/models";

import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  GetUserProfile,
  SendFriendRequest,
} from "../../../bindings/yukihub/internal/service/accountservice";
import { useAccountStatus } from "../../hooks/useAccountStatus";
import { ModalPortal } from "../ui/ModalPortal";
import { ChatAvatar } from "./ChatAvatar";
import { formatPlayTime, levelBadgeStyle } from "./levelBadge";

interface UserProfileModalProps {
  isOpen: boolean;
  uid: number;
  onClose: () => void;
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
  const accountStatus = useAccountStatus();
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [isAddingFriend, setIsAddingFriend] = useState(false);
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

  const isSelf = Number(accountStatus?.uid ?? 0) === uid;

  /** 加好友（资料页底部按钮，对齐手机版 renderUserProfile 的 actionBar） */
  const handleAddFriend = async () => {
    setIsAddingFriend(true);
    try {
      await SendFriendRequest(String(uid));
      toast.success(t("friendsChat.toastRequestSent"));
      setProfile(previous =>
        previous
          ? { ...previous, friendStatus: "pending", friendDirection: "sent" }
          : previous,
      );
    }
    catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    }
    finally {
      setIsAddingFriend(false);
    }
  };

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
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="truncate text-base font-bold text-brand-900 dark:text-white">
                        {profile.nickname || `UID ${profile.uid}`}
                      </span>
                      {/* 社区等级徽章（手机版资料页在昵称右侧挂 Lv.N） */}
                      {Number(profile.level) > 0 && (
                        <span
                          className="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-bold leading-none"
                          style={levelBadgeStyle(Number(profile.level))}
                        >
                          Lv.
                          {profile.level}
                        </span>
                      )}
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

                {profile.friendSince && (
                  <p className="text-xs text-brand-500 dark:text-brand-400">
                    {t("friendsChat.friendSince", {
                      time: profile.friendSince,
                    })}
                  </p>
                )}

                {/* 最近游玩（对齐手机版 renderUserProfile 的 recentGames 区块） */}
                {(profile.recentGames?.length ?? 0) > 0 && (
                  <div className="flex flex-col gap-2">
                    <div className="text-[11px] font-semibold text-brand-500 dark:text-brand-400">
                      {t("friendsChat.recentGames")}
                    </div>
                    <div className="flex flex-col divide-y divide-brand-200/70 rounded-lg border border-brand-200/80 dark:divide-brand-700/70 dark:border-brand-700/80">
                      {profile.recentGames?.map(game => (
                        <div
                          key={game.title}
                          className="flex items-center justify-between gap-3 px-3 py-2"
                        >
                          <span className="min-w-0 flex-1 truncate text-xs text-brand-800 dark:text-brand-100">
                            {game.title}
                          </span>
                          <span className="shrink-0 text-[11px] text-brand-500 dark:text-brand-400">
                            {formatPlayTime(game.playTime)}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* 好友操作（看自己时不显示，对齐手机版 actionBar） */}
                {!isSelf && (
                  <div className="pt-1">
                    {profile.friendStatus === "accepted" ? (
                      <div className="flex items-center justify-center gap-1.5 rounded-lg bg-success-500/12 py-2 text-sm font-medium text-success-600 dark:text-success-400">
                        <span className="i-mdi-check text-base" />
                        {t("friendsChat.alreadyFriend")}
                      </div>
                    ) : profile.friendStatus === "pending" ? (
                      <div className="rounded-lg bg-brand-100/80 py-2 text-center text-xs text-brand-500 dark:bg-brand-700/50 dark:text-brand-400">
                        {profile.friendDirection === "received"
                          ? t("friendsChat.friendRequestReceived")
                          : t("friendsChat.friendRequestSent")}
                      </div>
                    ) : (
                      <button
                        type="button"
                        onClick={() => void handleAddFriend()}
                        disabled={isAddingFriend}
                        className="flex w-full items-center justify-center gap-1.5 rounded-lg bg-primary-500 py-2 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:opacity-60"
                      >
                        <span
                          className={`text-base ${isAddingFriend ? "i-mdi-loading animate-spin" : "i-mdi-account-plus-outline"}`}
                        />
                        {t("friendsChat.addFriend")}
                      </button>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </ModalPortal>
  );
}
