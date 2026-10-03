import type { FriendPlayEvent } from "../components/chat/FriendPlayNotice";
import { useEffect } from "react";

import { toast } from "react-hot-toast";
import { onWailsEvent } from "../../src/bindings/runtime";
import { FriendPlayNotice } from "../components/chat/FriendPlayNotice";

/**
 * 请求打开好友面板的窗口事件。
 *
 * 好友开始玩的通知被点击时派发它（Steam 的通知点了会打开好友列表），
 * 侧栏监听后展开好友弹窗 —— 这样通知不必知道好友面板挂在哪个组件上。
 */
export const OPEN_FRIENDS_EVENT = "yukihub:open-friends";

/** 通知停留时长：比普通 toast 久一点，好友昵称和游戏名都需要时间读。 */
const FRIEND_PLAY_NOTICE_DURATION = 6000;

/**
 * 好友「开始玩游戏」通知。
 *
 * 检测在后端（account_friend_play.go：15 秒轮询 + 快照 diff，对齐手机版
 * PresenceService / FriendNotifier），这里只负责把它渲染成 Steam 风格的卡片。
 *
 * 用 toast.custom 而不是普通 toast：Steam 这类通知是带头像的两行卡片，
 * 走 toaster 的定位/堆叠/自动过期，外观自己画。
 */
export function useFriendPlayNotifications() {
  useEffect(() => {
    const unsubscribe = onWailsEvent<FriendPlayEvent>(
      "friend:playing",
      (event) => {
        if (!event?.nicknames?.length || !event?.game_title) {
          return;
        }
        if (event.notified_natively) {
          // 后端已经用系统通知送达（YukiHub 不在前台，比如游戏全屏）——
          // 系统通知能盖在全屏游戏上，应用内卡片这时反而看不见。
          // 两个都弹就是重复打扰，所以这里直接跳过。
          return;
        }

        // toastId 在 toast.custom 返回后立刻被赋值，而渲染发生在其后，
        // 所以下面的闭包能安全读到它。
        let toastId = "";
        toastId = toast.custom(
          () => (
            <FriendPlayNotice
              event={event}
              onOpen={() => {
                toast.dismiss(toastId);
                window.dispatchEvent(new CustomEvent(OPEN_FRIENDS_EVENT));
              }}
              onDismiss={() => toast.dismiss(toastId)}
            />
          ),
          {
            duration: FRIEND_PLAY_NOTICE_DURATION,
            position: "bottom-right",
          },
        );
      },
    );

    return unsubscribe;
  }, []);
}
