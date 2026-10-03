import type { service } from "../../../src/bindings/models";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  DismissFriendPlayNotice,
  GetFriendPlayNotice,
  OpenFriendsPanel,
} from "../../../bindings/yukihub/internal/service/overlayservice";
import { onWailsEvent } from "../../bindings/runtime";
import { ChatAvatar } from "../chat/ChatAvatar";

/** 后端 account_friend_play.go 的通知事件名（Go 侧 FriendPlayNoticeEvent）。 */
const FRIEND_PLAY_EVENT = "friend:playing";

/** 卡片停留时长：昵称和游戏名都要有时间读，比普通 toast 长一点。 */
const NOTICE_VISIBLE_MS = 6000;

/**
 * 拉取兜底窗口：窗口是刚建出来的，前端挂载一定晚于后端推送，
 * 所以「拉一次」是主路径；万一真的什么都没有（比如已经收起了），
 * 也别让一个空窗挂在屏幕角落。
 */
const EMPTY_NOTICE_GRACE_MS = 1500;

/** Steam 的「正在玩」绿（手机版 FriendNotifier 里也是这个色值）。 */
const PLAYING_GREEN = "#90BA3C";

/**
 * 好友「开始玩游戏」通知浮层。
 *
 * 这是一个**独立的全局浮层窗口**（URL 走 /notice，Go 侧 main.go 创建），
 * 不是主界面里的 toast —— 对齐 Steam：它的游玩通知也是全局的，不管你当前
 * 在看哪个窗口、是不是泡在游戏里。之前在应用内弹、还要 YukiHub 处于前台，
 * 等于大部分时候看不到。
 *
 * 窗口带 WS_EX_NOACTIVATE（main.go 里设置），显示时不会抢焦点 —— 通知不该把
 * 正在玩的游戏踢到后台。
 */
export default function FriendPlayNoticeWindow() {
  const { t } = useTranslation();
  const [notice, setNotice] = useState<service.FriendPlayEvent | null>(null);
  const dismissTimer = useRef<number | null>(null);
  const lastSeq = useRef(0);

  const clearDismissTimer = useCallback(() => {
    if (dismissTimer.current !== null) {
      window.clearTimeout(dismissTimer.current);
      dismissTimer.current = null;
    }
  }, []);

  const applyNotice = useCallback(
    (incoming: service.FriendPlayEvent | null | undefined) => {
      if (!incoming?.nicknames?.length || !incoming.game_title) {
        return;
      }
      // 「挂载时拉一次」和「订阅事件」都会拿到同一条，靠 seq 去重，
      // 免得刚渲染出来又被同一条重排一次（会闪）。
      if (incoming.seq && incoming.seq === lastSeq.current) {
        return;
      }
      lastSeq.current = incoming.seq ?? lastSeq.current;
      setNotice(incoming);
      clearDismissTimer();
      dismissTimer.current = window.setTimeout(() => {
        void DismissFriendPlayNotice();
      }, NOTICE_VISIBLE_MS);
    },
    [clearDismissTimer],
  );

  useEffect(() => {
    void (async () => {
      try {
        applyNotice(await GetFriendPlayNotice());
      }
      catch {
        // 拉不到就当没有，下面还有兜底
      }
    })();

    const unsubscribe = onWailsEvent<service.FriendPlayEvent>(
      FRIEND_PLAY_EVENT,
      applyNotice,
    );

    // 什么都没有就别占着屏幕
    const emptyTimer = window.setTimeout(() => {
      if (lastSeq.current === 0) {
        void DismissFriendPlayNotice();
      }
    }, EMPTY_NOTICE_GRACE_MS);

    return () => {
      unsubscribe();
      window.clearTimeout(emptyTimer);
      clearDismissTimer();
    };
  }, [applyNotice, clearDismissTimer]);

  const handleOpen = useCallback(() => {
    // 点通知 = 进好友面板（Steam 同款行为）。恢复主窗口、收起自己，
    // 都交给 Go 侧做 —— 浮层窗口不该去操作别的窗口。
    void OpenFriendsPanel();
  }, []);

  if (!notice) {
    // 窗口是半透明的，没内容时什么都不画（窗口本身可能还在，但看不见）
    return null;
  }

  const names = notice.nicknames ?? [];
  const avatars = notice.avatars ?? [];
  const avatarCount = Math.min(Math.max(names.length, 1), 3);

  return (
    <div className="h-full w-full bg-transparent p-1.5">
      <div
        role="button"
        tabIndex={0}
        onClick={handleOpen}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            handleOpen();
          }
        }}
        className="flex h-full w-full cursor-pointer items-center gap-2.5 rounded-xl border border-white/10 bg-[#16202d]/97 px-3 shadow-2xl backdrop-blur-md transition-colors hover:bg-[#1b2838]/97"
      >
        {/* 头像：多人时向左叠放。Steam 的卡片左侧也是用户头像。 */}
        <div className="flex shrink-0 items-center">
          {Array.from({ length: avatarCount }).map((_, index) => (
            <div
              key={notice.uids?.[index] ?? index}
              className={index > 0 ? "-ml-3" : ""}
              style={{ zIndex: 3 - index }}
            >
              <div className="rounded-full ring-2 ring-[#16202d]">
                <ChatAvatar
                  name={names[index] ?? ""}
                  avatar={avatars[index]}
                  size={38}
                />
              </div>
            </div>
          ))}
        </div>

        <div className="min-w-0 flex-1">
          <div className="truncate text-[13px] font-semibold leading-tight text-white">
            {names.join("、")}
          </div>
          <div className="mt-0.5 flex min-w-0 items-baseline gap-1.5 leading-tight">
            <span className="shrink-0 text-[11px] text-white/50">
              {t("friendsChat.playingLabel")}
            </span>
            <span
              className="truncate text-[12px] font-semibold"
              style={{ color: PLAYING_GREEN }}
            >
              {notice.game_title}
            </span>
          </div>
        </div>

        <button
          type="button"
          aria-label={t("common.close")}
          onClick={(event) => {
            event.stopPropagation();
            void DismissFriendPlayNotice();
          }}
          className="shrink-0 rounded-md p-1 text-white/40 transition-colors hover:bg-white/12 hover:text-white"
        >
          <span className="i-mdi-close text-base" aria-hidden="true" />
        </button>
      </div>
    </div>
  );
}
