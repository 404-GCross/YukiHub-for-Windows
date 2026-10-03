import { useTranslation } from "react-i18next";
import { ChatAvatar } from "./ChatAvatar";

/** 后端 friend:playing 事件的负载（对应 Go 的 service.FriendPlayEvent）。 */
export interface FriendPlayEvent {
  /** 同时收到通知的好友（Steam 会把玩同一游戏的人合并成一条） */
  uids: number[];
  /** 展示名，多个好友时用顿号连接显示 */
  nicknames: string[];
  avatars?: string[];
  /** 已经剥掉「正在玩：」前缀的游戏名 */
  game_title: string;
  /**
   * true 表示这条后端已经用系统通知送达了（YukiHub 不在前台，典型是游戏全屏）。
   * 前端据此跳过应用内卡片 —— 同一件事不该打扰两次。
   */
  notified_natively?: boolean;
}

interface FriendPlayNoticeProps {
  event: FriendPlayEvent;
  /** 点击卡片：Steam 的通知点了会打开好友列表，这里同样 */
  onOpen: () => void;
  onDismiss: () => void;
}

/** Steam 的「正在玩」绿（手机版 FriendNotifier 用的也是这个色值）。 */
const STEAM_PLAYING_GREEN = "#90BA3C";

/**
 * 「好友开始玩游戏」通知卡片 —— 对齐 Steam 自己的通知样式。
 *
 * 注意这不是 Windows 系统通知，是**应用内**自绘卡片（Steam 的也是它自己在
 * overlay 里画的）：深色底 + 左侧头像 + 三行文字，游戏名用 Steam 绿。
 *
 * 与 Steam 的唯一差别：它左侧放的是游戏封面（它有 AppID），我们拿不到好友
 * 在玩的游戏的封面（服务端只下发游戏名），所以用好友头像代替 —— 保留信息层次，
 * 不假装有封面。
 *
 * 多个好友在玩同一个游戏时合并成一条，昵称用顿号连接（「BPT、Ali」），
 * 与 Steam 一致。
 */
export function FriendPlayNotice({
  event,
  onOpen,
  onDismiss,
}: FriendPlayNoticeProps) {
  const { t } = useTranslation();
  const names = event.nicknames ?? [];
  const avatars = event.avatars ?? [];

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
      className="pointer-events-auto flex w-full cursor-pointer items-center gap-3 rounded-lg border border-white/10 bg-[#16202d]/97 p-2.5 shadow-2xl backdrop-blur-md transition-transform hover:scale-[1.02]"
    >
      {/* 左侧头像：多人时向左叠一张，最多三张，再多不叠了（宽度有限） */}
      <div className="flex shrink-0 items-center">
        {(avatars.length > 0 ? avatars : names).slice(0, 3).map((_, index) => (
          <div
            key={event.uids?.[index] ?? index}
            className={index > 0 ? "-ml-3" : ""}
            style={{ zIndex: 3 - index }}
          >
            <div className="rounded-full ring-2 ring-[#16202d]">
              <ChatAvatar
                name={names[index] ?? ""}
                avatar={avatars[index]}
                size={44}
              />
            </div>
          </div>
        ))}
      </div>

      <div className="min-w-0 flex-1">
        <div className="truncate text-[13px] font-semibold text-white">
          {names.join("、")}
        </div>
        <div className="truncate text-[11px] leading-tight text-white/55">
          {t("friendsChat.playingLabel")}
        </div>
        <div
          className="truncate text-[12px] font-semibold leading-snug"
          style={{ color: STEAM_PLAYING_GREEN }}
        >
          {event.game_title}
        </div>
      </div>

      <button
        type="button"
        aria-label={t("common.close")}
        onClick={(clickEvent) => {
          clickEvent.stopPropagation();
          onDismiss();
        }}
        className="shrink-0 rounded-md p-1 text-white/40 transition-colors hover:bg-white/12 hover:text-white"
      >
        <span className="i-mdi-close text-base" aria-hidden="true" />
      </button>
    </div>
  );
}
