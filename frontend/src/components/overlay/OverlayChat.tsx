import type {
  ChatMessage,
  Friend,
} from "../../../bindings/yukihub/internal/service/yukihubaccount/models";

import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  GetChatHistory,
  PollChatMessages,
  SendChatMessage,
} from "../../../bindings/yukihub/internal/service/accountservice";
import { resolveChatMediaURL } from "../../utils/chatMedia";
import { proxiedImageSrc } from "../../utils/imageProxy";
import { ChatAvatar } from "../chat/ChatAvatar";

/** 与主界面聊天一致：10 秒轮询新消息（手机版 FriendsChatDialog.POLL_INTERVAL_MS） */
const POLL_INTERVAL_MS = 10_000;
/** 历史消息每页条数，与手机版一致 */
const HISTORY_PAGE_SIZE = 20;

/**
 * 消息排序比较器（与主界面同一套规则）。
 *
 * **绝不能用 id 的字符串序**：服务端下发的 id 是纯数字字符串，字典序会退化成
 * 「按字符比」（"9" > "1000"），列表顺序会随机错乱。
 */
function compareMessages(a: ChatMessage, b: ChatMessage): number {
  const timeA = a.createdAt ?? "";
  const timeB = b.createdAt ?? "";
  if (timeA && timeB && timeA !== timeB) {
    return timeA < timeB ? -1 : 1;
  }
  const idA = Number(a.id);
  const idB = Number(b.id);
  if (Number.isFinite(idA) && Number.isFinite(idB) && idA !== idB) {
    return idA - idB;
  }
  return 0;
}

/**
 * 浮层内的私聊视图（Steam overlay 里那个聊天）。
 *
 * 只做私聊、只做文字与图片：表情包面板、回复、举报、图片上传这些留在主界面的
 * 完整聊天弹窗里 —— 浮层是「游戏里瞄一眼、随手回一句」的场景，塞满功能反而不好用。
 *
 * 不用 `toast`：浮层是独立窗口，主界面的 Toaster 不在这里挂载，弹出来也没人渲染，
 * 出错直接显示在输入框上方那一行。
 */
export function OverlayChat({ friend }: { friend: Friend }) {
  const { t } = useTranslation();
  const friendId = friend.id;

  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const afterIdRef = useRef("");
  const listRef = useRef<HTMLDivElement | null>(null);

  const scrollToBottom = (smooth: boolean) => {
    requestAnimationFrame(() => {
      const element = listRef.current;
      if (!element) {
        return;
      }
      element.scrollTo({
        top: element.scrollHeight,
        behavior: smooth ? "smooth" : "auto",
      });
    });
  };

  const mergeMessages = (incoming: ChatMessage[]) => {
    setMessages((current) => {
      const seen = new Set(current.map(item => item.id));
      const fresh = incoming.filter(item => !seen.has(item.id));
      if (fresh.length === 0) {
        return current;
      }
      return [...current, ...fresh].sort(compareMessages);
    });
  };

  // 拉历史 + 起轮询。
  //
  // 换好友时父组件用 `key={friend.id}` 让本组件整体重挂载（state 自然归零），
  // 所以这里不需要在 effect 里手动 setState 复位 —— 那样反而会多一次渲染。
  useEffect(() => {
    let cancelled = false;

    void (async () => {
      try {
        const history = await GetChatHistory(friendId, 0, HISTORY_PAGE_SIZE);
        if (cancelled) {
          return;
        }
        const ordered = [...history].sort(compareMessages);
        setMessages(ordered);
        afterIdRef.current = ordered[ordered.length - 1]?.id ?? "";
        scrollToBottom(false);
      }
      catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : String(err));
        }
      }
      finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    })();

    const timer = window.setInterval(() => {
      // 浮层收起来（窗口隐藏）时不必轮询：没人在看，白耗服务端和流量
      if (document.hidden) {
        return;
      }
      void (async () => {
        try {
          const fresh = await PollChatMessages(
            afterIdRef.current,
            friendId,
            false,
          );
          if (cancelled || fresh.length === 0) {
            return;
          }
          mergeMessages(fresh);
          afterIdRef.current = fresh[fresh.length - 1].id || afterIdRef.current;
          scrollToBottom(true);
        }
        catch {
          // 轮询失败静默：下个周期自然重试
        }
      })();
    }, POLL_INTERVAL_MS);

    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [friendId]);

  const handleSend = async () => {
    const text = draft.trim();
    if (!text || isSending) {
      return;
    }
    setIsSending(true);
    setError(null);
    try {
      const sent = await SendChatMessage(friendId, text, "text", "");
      setDraft("");
      if (sent.id) {
        mergeMessages([sent]);
        afterIdRef.current = sent.id || afterIdRef.current;
      }
      scrollToBottom(true);
    }
    catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
    finally {
      setIsSending(false);
    }
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div
        ref={listRef}
        className="min-h-0 flex-1 space-y-2 overflow-y-auto px-2.5 py-2"
      >
        {isLoading && (
          <p className="py-4 text-center text-[11px] text-white/50">
            {t("friendsChat.loading")}
          </p>
        )}
        {!isLoading && messages.length === 0 && !error && (
          <p className="py-4 text-center text-[11px] text-white/50">
            {t("friendsOverlay.noMessages")}
          </p>
        )}

        {messages.map((message) => {
          const isMine = Boolean(message.isMine);
          const isImage = message.msgType === "image";
          const isEmoji = message.msgType === "emoji";
          return (
            <div
              key={message.id}
              className={`flex items-end gap-1.5 ${
                isMine ? "justify-end" : "justify-start"
              }`}
            >
              {!isMine && (
                <ChatAvatar
                  name={message.senderName || friend.nickname}
                  avatar={message.senderAvatar || friend.avatar}
                  size={22}
                />
              )}
              <div
                className={`max-w-[76%] overflow-hidden rounded-2xl px-2.5 py-1.5 text-[12.5px] leading-snug break-words ${
                  isMine
                    ? "rounded-br-md bg-primary-500 text-white"
                    : "rounded-bl-md bg-white/10 text-white/92"
                }`}
              >
                {isImage ? (
                  <img
                    src={proxiedImageSrc(resolveChatMediaURL(message.content))}
                    alt=""
                    className="max-h-40 rounded-lg"
                  />
                ) : isEmoji ? (
                  <img
                    src={proxiedImageSrc(resolveChatMediaURL(message.content))}
                    alt=""
                    className="h-12 w-12"
                  />
                ) : (
                  message.content
                )}
              </div>
            </div>
          );
        })}
      </div>

      {error && (
        <p className="shrink-0 px-3 pb-1 text-[11px] text-red-400">{error}</p>
      )}

      <div className="flex shrink-0 items-center gap-1.5 border-t border-white/10 px-2 py-2">
        <input
          // 进聊天就是要打字，直接把光标放进去
          autoFocus
          value={draft}
          onChange={event => setDraft(event.target.value)}
          onKeyDown={(event) => {
            // Enter 发送；输入法组合中不拦（中文选词那一下也是 Enter）
            if (
              event.key === "Enter"
              && !event.shiftKey
              && !event.nativeEvent.isComposing
            ) {
              event.preventDefault();
              void handleSend();
            }
          }}
          placeholder={t("friendsChat.inputPlaceholder")}
          className="min-w-0 flex-1 rounded-lg bg-white/8 px-2.5 py-1.5 text-[12.5px] text-white outline-none placeholder:text-white/35 focus:bg-white/12"
        />
        <button
          type="button"
          aria-label={t("friendsOverlay.send")}
          disabled={!draft.trim() || isSending}
          onClick={() => void handleSend()}
          className="shrink-0 rounded-lg p-1.5 text-primary-300 transition-colors hover:bg-white/12 disabled:opacity-40"
        >
          <span className="i-mdi-send text-base" aria-hidden="true" />
        </button>
      </div>
    </div>
  );
}
