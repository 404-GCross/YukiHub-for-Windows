import type {
  Friend,
  FriendList,
} from "../../../bindings/yukihub/internal/service/yukihubaccount/models";

import { Window } from "@wailsio/runtime";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { ListFriends } from "../../../bindings/yukihub/internal/service/accountservice";
import { onWailsEvent } from "../../bindings/runtime";
import { ChatAvatar } from "../chat/ChatAvatar";

/** 后端 account_friend_play.go 的好友列表推送事件 */
const FRIEND_LIST_UPDATED_EVENT = "friend:list-updated";

interface Sections {
  playing: Friend[];
  online: Friend[];
  offline: Friend[];
}

/**
 * 游戏内好友栏（overlay）—— 按 Alt+Shift+Tab 呼出，对齐 Steam 的 Shift+Tab。
 *
 * 这是一个**独立的置顶窗口**（URL 走 /overlay），不是主窗口里的浮层：
 * 只有独立窗口才能盖在游戏画面上。窗口由 Go 侧惰性创建（main.go 的
 * toggleOverlayWindow），这里只负责画内容。
 *
 * 数据来源与主界面完全一致：后端每 10 秒轮询一次 /friends/list，有变化就推
 * 整份列表过来，所以浮层里的状态和主界面永远同步。
 *
 * 诚实的限制：能盖在窗口化 / 无边框全屏游戏上，盖不住独占全屏游戏
 * （Steam 靠往游戏进程注入 hook 才做到，YukiHub 不做注入）。
 */
export default function FriendsOverlay() {
  const { t } = useTranslation();
  const [friendList, setFriendList] = useState<FriendList | null>(null);
  const [error, setError] = useState<string | null>(null);

  // 拉一次 + 订阅后端推送：浮层通常开得比第一次轮询早，所以要主动拉一次
  useEffect(() => {
    let cancelled = false;

    ListFriends()
      .then((list) => {
        if (!cancelled) {
          setFriendList(list);
          setError(null);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err));
        }
      });

    const unsubscribe = onWailsEvent<FriendList>(
      FRIEND_LIST_UPDATED_EVENT,
      (list) => {
        if (list?.friends) {
          setFriendList(list);
          setError(null);
        }
      },
    );

    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, []);

  // Esc 收起浮层：游戏里手不会离开键盘，鼠标点关闭太慢
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        void Window.Hide();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  // 分组规则与主界面一致（正在游戏 → 在线 → 离线）
  const sections = useMemo<Sections>(() => {
    const result: Sections = { playing: [], online: [], offline: [] };
    for (const friend of friendList?.friends ?? []) {
      if (friend.status === "online" && friend.activity?.trim()) {
        result.playing.push(friend);
      }
      else if (
        friend.status === "online"
        || friend.status === "away"
        || friend.status === "busy"
      ) {
        result.online.push(friend);
      }
      else {
        result.offline.push(friend);
      }
    }
    return result;
  }, [friendList]);

  const hasAnyFriend
    = sections.playing.length + sections.online.length + sections.offline.length
      > 0;

  return (
    // 浮层整体透明，真实外观靠内层卡片；外层留一点边距当阴影空间
    <div className="h-screen w-screen bg-transparent p-1.5">
      <div className="flex h-full flex-col overflow-hidden rounded-2xl border border-white/12 bg-brand-900/90 text-white shadow-2xl backdrop-blur-xl">
        {/* 顶栏兼拖动柄：overlay 没有标题栏，得给用户一个能拖的地方 */}
        <div
          className="flex shrink-0 items-center justify-between border-b border-white/10 px-3.5 py-2.5"
          style={{ "--wails-draggable": "drag" } as React.CSSProperties}
        >
          <span className="flex items-center gap-1.5 text-[13px] font-bold">
            <span
              className="i-mdi-account-group-outline text-base text-primary-300"
              aria-hidden="true"
            />
            {t("friendsOverlay.title")}
          </span>
          <button
            type="button"
            aria-label={t("common.close")}
            onClick={() => void Window.Hide()}
            className="rounded-md p-1 text-white/60 transition-colors hover:bg-white/12 hover:text-white"
          >
            <span className="i-mdi-close text-base" aria-hidden="true" />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-1.5 py-2">
          {error && (
            <p className="px-2 py-6 text-center text-xs text-white/60">
              {error}
            </p>
          )}
          {!error && !friendList && (
            <p className="px-2 py-6 text-center text-xs text-white/60">
              {t("friendsChat.loading")}
            </p>
          )}
          {!error && friendList && !hasAnyFriend && (
            <p className="px-3 py-6 text-center text-xs whitespace-pre-line text-white/60">
              {t("friendsChat.noFriends")}
            </p>
          )}

          <OverlaySection
            title={t("friendsChat.section.playing")}
            friends={sections.playing}
            highlight
          />
          <OverlaySection
            title={t("friendsChat.section.online")}
            friends={sections.online}
          />
          <OverlaySection
            title={t("friendsChat.section.offline")}
            friends={sections.offline}
            dim
          />
        </div>

        <div className="shrink-0 border-t border-white/10 px-3.5 py-1.5 text-[10px] text-white/40">
          {t("friendsOverlay.hint")}
        </div>
      </div>
    </div>
  );
}

function OverlaySection({
  title,
  friends,
  highlight = false,
  dim = false,
}: {
  title: string;
  friends: Friend[];
  highlight?: boolean;
  dim?: boolean;
}) {
  if (friends.length === 0) {
    return null;
  }

  return (
    <div className="mb-1.5">
      <div className="px-2 py-1 text-[10px] font-semibold tracking-wide text-white/45">
        {title}
        {" — "}
        {friends.length}
      </div>
      {friends.map(friend => (
        <div
          key={friend.id || friend.uid}
          className="flex items-center gap-2.5 rounded-lg px-2 py-1.5 transition-colors hover:bg-white/8"
        >
          <div className={`relative shrink-0 ${dim ? "opacity-55" : ""}`}>
            <ChatAvatar
              name={friend.note || friend.nickname}
              avatar={friend.avatar}
              size={32}
            />
            <span
              className={`absolute -right-0.5 -bottom-0.5 h-2.5 w-2.5 rounded-full border-2 border-brand-900 ${
                friend.status === "online"
                  ? "bg-emerald-500"
                  : friend.status === "away" || friend.status === "busy"
                    ? "bg-amber-500"
                    : "bg-brand-400"
              }`}
            />
          </div>
          <div className="min-w-0 flex-1">
            <div
              className={`truncate text-[12px] font-medium ${dim ? "text-white/55" : "text-white/92"}`}
            >
              {friend.note || friend.nickname}
            </div>
            <div
              className={`truncate text-[10px] ${
                highlight ? "text-emerald-400" : "text-white/50"
              }`}
            >
              {friend.activity?.trim() || ""}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
