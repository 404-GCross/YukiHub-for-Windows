import { useTranslation } from "react-i18next";
import { ChatAvatar } from "./ChatAvatar";

/** 后端 friend:playing 事件的负载（对应 Go 的 service.FriendPlayEvent）。 */
export interface FriendPlayEvent {
  uid: number;
  nickname: string;
  avatar?: string;
  /** 已经剥掉「正在玩：」前缀的游戏名 */
  game_title: string;
}

interface FriendPlayNoticeProps {
  event: FriendPlayEvent;
  /** 点击卡片：Steam 的通知点了会打开好友列表，这里同样 */
  onOpen: () => void;
  onDismiss: () => void;
}

/**
 * 「好友开始玩游戏」通知卡片（Steam 风格）。
 *
 * 与普通 toast 的区别：Steam 的这类通知是**带头像的两行卡片**，且专门告诉
 * 你「谁 · 开始玩 · 什么游戏」。文案对齐手机版 FriendNotifier：
 * 标题=昵称，正文=`开始玩 《游戏名》`。
 *
 * 用 toast.custom 渲染，因此定位、堆叠、自动过期都复用 toaster 那套；
 * 这里只负责外观。
 */
export function FriendPlayNotice({
  event,
  onOpen,
  onDismiss,
}: FriendPlayNoticeProps) {
  const { t } = useTranslation();

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(keyboardEvent) => {
        if (keyboardEvent.key === "Enter" || keyboardEvent.key === " ") {
          keyboardEvent.preventDefault();
          onOpen();
        }
      }}
      className="pointer-events-auto flex w-full cursor-pointer items-center gap-3 rounded-xl border border-brand-200/90 bg-white/97 p-3 shadow-2xl backdrop-blur-md transition-transform hover:scale-[1.02] dark:border-brand-700/90 dark:bg-brand-800/97"
    >
      <ChatAvatar name={event.nickname} avatar={event.avatar} size={44} />

      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-semibold text-brand-900 dark:text-white">
          {event.nickname}
        </div>
        <div className="truncate text-xs text-brand-600 dark:text-brand-300">
          {t("friendsChat.friendPlayNotice", { game: event.game_title })}
        </div>
      </div>

      <button
        type="button"
        aria-label={t("common.close")}
        onClick={(clickEvent) => {
          clickEvent.stopPropagation();
          onDismiss();
        }}
        className="shrink-0 rounded-lg p-1 text-brand-400 transition-colors hover:bg-brand-100 hover:text-brand-700 dark:hover:bg-brand-700 dark:hover:text-brand-200"
      >
        <span className="i-mdi-close text-base" aria-hidden="true" />
      </button>
    </div>
  );
}
