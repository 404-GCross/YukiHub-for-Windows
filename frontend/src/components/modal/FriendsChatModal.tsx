import type {
  ChatEmoji,
  ChatGroup,
  ChatMessage,
  Friend,
  FriendList,
} from "../../../bindings/yukihub/internal/service/yukihubaccount/models";
import type { vo } from "../../../src/bindings/models";
import { useEffect, useMemo, useRef, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  AcceptFriendRequest,
  GetChatHistory,
  GetGroupHistory,
  ListChatEmojis,
  ListChatGroups,
  ListChatStickerPacks,
  ListChatStickerURLs,
  ListFriends,
  PollChatMessages,
  PollGroupMessages,
  RejectFriendRequest,
  SearchUsers,
  SelectChatImage,
  SendChatMessage,
  SendFriendRequest,
  SendGroupMessage,
  UploadChatImage,
} from "../../../bindings/yukihub/internal/service/accountservice";
import { proxiedImageSrc } from "../../utils/imageProxy";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterInput } from "../ui/better/BetterInput";
import { ModalPortal } from "../ui/ModalPortal";

interface FriendsChatModalProps {
  isOpen: boolean;
  isLoggedIn: boolean;
  onClose: () => void;
}

/** 轮询间隔，与手机版 FriendsChatDialog.POLL_INTERVAL_MS 一致（10 秒） */
const POLL_INTERVAL_MS = 10_000;
/** 历史消息每页条数，与手机版一致 */
const HISTORY_PAGE_SIZE = 20;

type ChatTarget
  = | { kind: "friend"; friend: Friend }
    | { kind: "group"; group: ChatGroup };

type MainView = "list" | "requests" | "add" | "chat";

interface ChatDraft {
  text: string;
}

/** 表情/贴纸选择面板：0=本站表情 1=未萌贴纸包列表 2=包内表情（与手机版一致） */
type EmojiTab = 0 | 1 | 2;

interface StickerPackView {
  id: string;
  title: string;
  cover: string;
  stickerCount: number;
}

/**
 * 好友 / 聊天弹窗，功能对齐手机版 FriendsChatDialog：
 *
 * - 好友列表按「正在游戏 → 在线 → 离线」分组（Steam 风格）
 * - 群组列表置顶
 * - 私聊 / 群聊：历史消息分页、10 秒轮询新消息、回复
 * - 添加好友：按 UID 或昵称搜索
 * - 好友请求：接受 / 拒绝
 */
export function FriendsChatModal({
  isOpen,
  isLoggedIn,
  onClose,
}: FriendsChatModalProps) {
  const { t } = useTranslation();

  const [view, setView] = useState<MainView>("list");
  const [friendList, setFriendList] = useState<FriendList | null>(null);
  const [groups, setGroups] = useState<ChatGroup[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [target, setTarget] = useState<ChatTarget | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [hasMoreHistory, setHasMoreHistory] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [draft, setDraft] = useState<ChatDraft>({ text: "" });
  const [isSending, setIsSending] = useState(false);
  const [replyTo, setReplyTo] = useState<ChatMessage | null>(null);
  const [searchKeyword, setSearchKeyword] = useState("");
  const [searchResults, setSearchResults] = useState<Friend[] | null>(null);
  const [isSearching, setIsSearching] = useState(false);

  // 表情 / 贴纸面板状态
  const [emojiPanelOpen, setEmojiPanelOpen] = useState(false);
  const [emojiTab, setEmojiTab] = useState<EmojiTab>(0);
  const [emojis, setEmojis] = useState<ChatEmoji[] | null>(null);
  const [stickerEnabled, setStickerEnabled] = useState(false);
  const [stickerPacks, setStickerPacks] = useState<StickerPackView[]>([]);
  const [activeStickerPack, setActiveStickerPack]
    = useState<StickerPackView | null>(null);
  const [stickerURLs, setStickerURLs] = useState<string[]>([]);
  const [stickerLoading, setStickerLoading] = useState(false);
  const [isUploadingImage, setIsUploadingImage] = useState(false);

  const listRef = useRef<HTMLDivElement | null>(null);
  const oldestIdRef = useRef<string>("");
  const targetRef = useRef<ChatTarget | null>(null);
  const afterIdRef = useRef<string>("");
  const pollTimerRef = useRef<number | null>(null);

  targetRef.current = target;

  /** 表情名 → 可显示 URL：http(s) 原样；本站表情按手机版规则兜底 */
  const emojiDisplayURL = (content: string): string => {
    if (/^https?:\/\//i.test(content)) {
      return content;
    }
    return `https://yukihub.zh.kg/uploads/emojis/${content}.webp`;
  };

  /** 懒加载表情与贴纸数据（首次打开面板时） */
  const loadEmojiPanelData = async () => {
    if (emojis === null) {
      ListChatEmojis()
        .then(list => setEmojis(list))
        .catch(() => setEmojis([]));
    }
    if (!stickerEnabled && stickerPacks.length === 0) {
      ListChatStickerPacks()
        .then((list) => {
          setStickerEnabled(list.enabled);
          setStickerPacks(
            list.packs.map(pack => ({
              id: pack.id ?? "",
              title: pack.title ?? "",
              cover: pack.cover ?? "",
              stickerCount: pack.sticker_count ?? 0,
            })),
          );
        })
        .catch(() => setStickerEnabled(false));
    }
  };

  /** 进入某个贴纸包 */
  const openStickerPack = async (pack: StickerPackView) => {
    setStickerLoading(true);
    try {
      const urls = await ListChatStickerURLs(pack.id);
      if (urls.length === 0) {
        toast.error(t("friendsChat.stickerPackEmpty"));
        return;
      }
      setActiveStickerPack(pack);
      setStickerURLs(urls);
      setEmojiTab(2);
    }
    catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    }
    finally {
      setStickerLoading(false);
    }
  };

  /** 滚到消息底部 */
  const scrollToBottom = (smooth = true) => {
    requestAnimationFrame(() => {
      const el = listRef.current;
      if (el) {
        el.scrollTo({
          top: el.scrollHeight,
          behavior: smooth ? "smooth" : "auto",
        });
      }
    });
  };

  const mergeMessages = (incoming: ChatMessage[]) => {
    if (incoming.length === 0) {
      return;
    }
    setMessages((current) => {
      const seen = new Set(current.map(item => item.id));
      const merged = [...current];
      for (const item of incoming) {
        if (!seen.has(item.id)) {
          merged.push(item);
          seen.add(item.id);
        }
      }
      merged.sort((a, b) => String(a.id).localeCompare(String(b.id)));
      return merged;
    });
  };

  /** 发送表情消息（本站表情传名字，贴纸传 URL，与手机版一致） */
  const sendEmoji = async (content: string) => {
    const current = target;
    if (!current || isSending) {
      return;
    }
    setIsSending(true);
    try {
      const isFriend = current.kind === "friend";
      const sent = isFriend
        ? await SendChatMessage(current.friend.id, content, "emoji", "")
        : await SendGroupMessage(current.group.id, content, "emoji", "");
      if (sent.id) {
        mergeMessages([sent]);
        afterIdRef.current = sent.id || afterIdRef.current;
      }
      setEmojiPanelOpen(false);
      scrollToBottom();
    }
    catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    }
    finally {
      setIsSending(false);
    }
  };

  /** 选图 → 上传 → 发图片消息（选图/上传在后端，前端只串流程） */
  const pickAndSendImage = async () => {
    if (isUploadingImage) {
      return;
    }
    setIsUploadingImage(true);
    try {
      const picked: vo.ChatImagePick = await SelectChatImage();
      if (!picked.path) {
        return; // 用户取消
      }
      const url = await UploadChatImage(picked.data, picked.mime_type);
      const current = target;
      if (!current) {
        return;
      }
      const isFriend = current.kind === "friend";
      const sent = isFriend
        ? await SendChatMessage(current.friend.id, url, "image", "")
        : await SendGroupMessage(current.group.id, url, "image", "");
      if (sent.id) {
        mergeMessages([sent]);
        afterIdRef.current = sent.id || afterIdRef.current;
      }
      scrollToBottom();
    }
    catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    }
    finally {
      setIsUploadingImage(false);
    }
  };

  /** 拉好友 + 群列表 */
  const refreshLists = async (withLoading: boolean) => {
    if (withLoading) {
      setIsLoading(true);
    }
    setLoadError(null);
    try {
      const [friends, groupItems] = await Promise.all([
        ListFriends(),
        ListChatGroups(),
      ]);
      setFriendList(friends);
      setGroups(groupItems);
    }
    catch (error) {
      setLoadError(error instanceof Error ? error.message : String(error));
    }
    finally {
      setIsLoading(false);
    }
  };

  // 打开时拉一次列表；关闭时清状态
  useEffect(() => {
    if (!isOpen) {
      setView("list");
      setTarget(null);
      setMessages([]);
      setReplyTo(null);
      setDraft({ text: "" });
      setSearchResults(null);
      setSearchKeyword("");
      return;
    }
    if (!isLoggedIn) {
      return;
    }
    void refreshLists(true);
  }, [isOpen, isLoggedIn]);

  // 轮询清理
  useEffect(
    () => () => {
      if (pollTimerRef.current !== null) {
        window.clearInterval(pollTimerRef.current);
        pollTimerRef.current = null;
      }
    },
    [],
  );

  /** 打开会话：拉历史并启动轮询 */
  const openChat = async (nextTarget: ChatTarget) => {
    if (pollTimerRef.current !== null) {
      window.clearInterval(pollTimerRef.current);
      pollTimerRef.current = null;
    }
    setTarget(nextTarget);
    setView("chat");
    setMessages([]);
    setReplyTo(null);
    setHasMoreHistory(false);
    setHistoryLoading(true);

    try {
      const isFriend = nextTarget.kind === "friend";
      const id = isFriend ? nextTarget.friend.id : nextTarget.group.id;
      const history = isFriend
        ? await GetChatHistory(id, 0, HISTORY_PAGE_SIZE)
        : await GetGroupHistory(id, 0, HISTORY_PAGE_SIZE);
      setMessages(history);
      oldestIdRef.current = history[0]?.id ?? "";
      afterIdRef.current = history[history.length - 1]?.id ?? "";
      setHasMoreHistory(history.length >= HISTORY_PAGE_SIZE);
      scrollToBottom(false);

      // 10 秒轮询新消息（与手机版一致）
      pollTimerRef.current = window.setInterval(() => {
        void (async () => {
          const current = targetRef.current;
          if (!current) {
            return;
          }
          try {
            if (current.kind === "friend") {
              const fresh = await PollChatMessages(
                afterIdRef.current,
                current.friend.id,
                false,
              );
              if (fresh.length > 0) {
                mergeMessages(fresh);
                afterIdRef.current
                  = fresh[fresh.length - 1].id || afterIdRef.current;
                scrollToBottom();
              }
            }
            else {
              const fresh = await PollGroupMessages(
                current.group.id,
                afterIdRef.current,
              );
              if (fresh.length > 0) {
                mergeMessages(fresh);
                afterIdRef.current
                  = fresh[fresh.length - 1].id || afterIdRef.current;
                scrollToBottom();
              }
            }
          }
          catch {
            // 轮询失败静默：下个周期再试
          }
        })();
      }, POLL_INTERVAL_MS);
    }
    catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
      setView("list");
    }
    finally {
      setHistoryLoading(false);
    }
  };

  /** 加载更早的历史 */
  const loadOlderHistory = async () => {
    const current = target;
    if (!current || historyLoading || !hasMoreHistory || !oldestIdRef.current) {
      return;
    }
    setHistoryLoading(true);
    try {
      const isFriend = current.kind === "friend";
      const id = isFriend ? current.friend.id : current.group.id;
      const listEl = listRef.current;
      const previousHeight = listEl?.scrollHeight ?? 0;

      const history = isFriend
        ? await GetChatHistory(id, messages.length, HISTORY_PAGE_SIZE)
        : await GetGroupHistory(id, messages.length, HISTORY_PAGE_SIZE);

      if (history.length > 0) {
        setMessages((currentMessages) => {
          const seen = new Set(currentMessages.map(item => item.id));
          const older = history.filter(item => !seen.has(item.id));
          return [...older, ...currentMessages].sort((a, b) =>
            String(a.id).localeCompare(String(b.id)),
          );
        });
        oldestIdRef.current = history[0]?.id ?? oldestIdRef.current;
        requestAnimationFrame(() => {
          const el = listRef.current;
          if (el) {
            el.scrollTop = el.scrollHeight - previousHeight;
          }
        });
      }
      setHasMoreHistory(history.length >= HISTORY_PAGE_SIZE);
    }
    catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    }
    finally {
      setHistoryLoading(false);
    }
  };

  /** 发消息（私聊与群聊共用） */
  const handleSend = async () => {
    const current = target;
    const text = draft.text.trim();
    if (!current || !text || isSending) {
      return;
    }
    setIsSending(true);
    try {
      const isFriend = current.kind === "friend";
      const replyToID = replyTo?.id ?? "";
      const sent = isFriend
        ? await SendChatMessage(current.friend.id, text, "text", replyToID)
        : await SendGroupMessage(current.group.id, text, "text", replyToID);
      if (sent.id) {
        mergeMessages([sent]);
        afterIdRef.current = sent.id || afterIdRef.current;
      }
      setDraft({ text: "" });
      setReplyTo(null);
      scrollToBottom();
    }
    catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    }
    finally {
      setIsSending(false);
    }
  };

  /** 搜索用户（UID 或昵称） */
  const handleSearch = async () => {
    const keyword = searchKeyword.trim();
    if (!keyword || isSearching) {
      return;
    }
    setIsSearching(true);
    try {
      setSearchResults(await SearchUsers(keyword));
    }
    catch (error) {
      toast.error(error instanceof Error ? error.message : String(error));
    }
    finally {
      setIsSearching(false);
    }
  };

  /** 好友列表分组：正在游戏 → 在线 → 离线（Steam 风格） */
  const friendSections = useMemo(() => {
    const friends = friendList?.friends ?? [];
    const playing: Friend[] = [];
    const online: Friend[] = [];
    const offline: Friend[] = [];
    for (const friend of friends) {
      if (friend.status === "online" && friend.activity?.trim()) {
        playing.push(friend);
      }
      else if (
        friend.status === "online"
        || friend.status === "away"
        || friend.status === "busy"
      ) {
        online.push(friend);
      }
      else {
        offline.push(friend);
      }
    }
    return { playing, online, offline };
  }, [friendList]);

  if (!isOpen) {
    return null;
  }

  const renderAvatar = (name: string, avatar?: string, size = "h-10 w-10") => {
    if (avatar) {
      return (
        <img
          src={avatar}
          alt=""
          className={`${size} shrink-0 rounded-full border border-brand-200/70 object-cover dark:border-brand-700/70`}
        />
      );
    }
    return (
      <div
        className={`${size} flex shrink-0 items-center justify-center rounded-full bg-primary-500/15 text-sm font-semibold text-primary-600 dark:text-primary-300`}
      >
        {(name || "?").slice(0, 1).toUpperCase()}
      </div>
    );
  };

  const statusDot = (status?: string) => {
    const colorClass
      = status === "online"
        ? "bg-emerald-500"
        : status === "away" || status === "busy"
          ? "bg-amber-500"
          : "bg-brand-400 dark:bg-brand-500";
    return (
      <span className={`inline-block h-2.5 w-2.5 rounded-full ${colorClass}`} />
    );
  };

  const renderMessageBubble = (message: ChatMessage, isGroup: boolean) => {
    const isMine = Boolean(message.isMine);
    const senderName = message.senderName || `UID ${message.senderUid}`;
    // 表情 / 图片 / 文本三种气泡（与手机版 buildMessageBubble 的分支一致）
    const bubbleContent = (() => {
      if (message.msgType === "emoji") {
        return (
          <img
            src={proxiedImageSrc(emojiDisplayURL(message.content))}
            alt=""
            className="h-24 w-24 object-contain"
          />
        );
      }
      if (message.msgType === "image") {
        return (
          <img
            src={proxiedImageSrc(message.content)}
            alt=""
            className="max-h-48 rounded-xl object-contain"
          />
        );
      }
      return message.content;
    })();
    const isMedia = message.msgType === "emoji" || message.msgType === "image";
    return (
      <div
        key={message.id}
        className={`flex gap-2 ${isMine ? "flex-row-reverse" : "flex-row"}`}
      >
        <div className="w-8 shrink-0 pt-1">
          {!isMine && renderAvatar(senderName, message.senderAvatar, "h-8 w-8")}
        </div>
        <div
          className={`flex max-w-[72%] flex-col gap-0.5 ${isMine ? "items-end" : "items-start"}`}
        >
          {isGroup && !isMine && (
            <span className="px-1 text-[11px] text-brand-500 dark:text-brand-400">
              {senderName}
            </span>
          )}
          {message.replyPreview && (
            <div className="max-w-full truncate rounded-md border-l-2 border-primary-400/60 bg-brand-100/70 px-2 py-0.5 text-[11px] text-brand-600 dark:bg-brand-900/50 dark:text-brand-300">
              {message.replyPreview}
            </div>
          )}
          {isMedia ? (
            <div
              className="cursor-pointer"
              title={t("friendsChat.reply")}
              onClick={() => setReplyTo(message)}
            >
              {bubbleContent}
            </div>
          ) : (
            <div
              className={`cursor-pointer rounded-2xl px-3 py-2 text-sm leading-relaxed ${
                isMine
                  ? "rounded-br-md bg-primary-500 text-white"
                  : "rounded-bl-md bg-brand-100 text-brand-900 dark:bg-brand-700/70 dark:text-brand-50"
              }`}
              title={t("friendsChat.reply")}
              onClick={() => setReplyTo(message)}
            >
              {bubbleContent}
            </div>
          )}
          {message.createdAt && (
            <span className="px-1 text-[10px] text-brand-400 dark:text-brand-500">
              {message.createdAt}
            </span>
          )}
        </div>
      </div>
    );
  };

  const pendingCount = friendList?.pendingCount ?? 0;

  return (
    <ModalPortal>
      <div
        className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
        onClick={onClose}
      >
        <div
          className="flex h-[min(640px,88vh)] w-full max-w-3xl flex-col overflow-hidden rounded-2xl border border-brand-200 bg-white shadow-2xl dark:border-brand-700 dark:bg-brand-800"
          onClick={e => e.stopPropagation()}
        >
          {/* 标题栏 */}
          <div className="flex items-center gap-2 border-b border-brand-200/80 px-4 py-3 dark:border-brand-700/80">
            {view !== "list" && (
              <button
                type="button"
                onClick={() => {
                  if (view === "chat" && pollTimerRef.current !== null) {
                    window.clearInterval(pollTimerRef.current);
                    pollTimerRef.current = null;
                  }
                  setView("list");
                  setTarget(null);
                  setReplyTo(null);
                  void refreshLists(false);
                }}
                aria-label={t("common.back")}
                className="rounded-lg p-1.5 text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-700 dark:text-brand-400 dark:hover:bg-brand-700"
              >
                <span className="i-mdi-arrow-left text-lg" />
              </button>
            )}
            <h2 className="flex min-w-0 flex-1 items-center gap-2 truncate text-sm font-bold text-brand-900 dark:text-white">
              {view === "chat" && target ? (
                <>
                  <span className="i-mdi-chat-outline text-primary-500" />
                  <span className="truncate">
                    {target.kind === "friend"
                      ? target.friend.note || target.friend.nickname
                      : target.group.name}
                  </span>
                </>
              ) : (
                t("friendsChat.title")
              )}
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

          {/* 未登录提示 */}
          {!isLoggedIn ? (
            <div className="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center">
              <span className="i-mdi-account-lock-outline text-4xl text-brand-400" />
              <p className="text-sm text-brand-600 dark:text-brand-300">
                {t("friendsChat.loginRequired")}
              </p>
            </div>
          ) : (
            view === "list" && (
              <div className="flex min-h-0 flex-1 flex-col">
                {/* 操作栏 */}
                <div className="flex gap-2 px-4 pt-3">
                  <BetterButton
                    variant="secondary"
                    size="sm"
                    icon="i-mdi-account-plus-outline"
                    className="flex-1"
                    onClick={() => setView("add")}
                  >
                    {t("friendsChat.addFriend")}
                  </BetterButton>
                  <BetterButton
                    variant="secondary"
                    size="sm"
                    icon="i-mdi-email-outline"
                    className="flex-1"
                    onClick={() => setView("requests")}
                  >
                    {pendingCount > 0
                      ? t("friendsChat.requests", { count: pendingCount })
                      : t("friendsChat.requestsZero")}
                  </BetterButton>
                </div>

                <div className="mt-3 min-h-0 flex-1 overflow-y-auto px-4 pb-4">
                  {isLoading && !friendList && (
                    <p className="py-8 text-center text-sm text-brand-500">
                      {t("friendsChat.loading")}
                    </p>
                  )}
                  {loadError && (
                    <div className="flex flex-col items-center gap-2 py-8">
                      <p className="text-sm text-error-500">{loadError}</p>
                      <BetterButton
                        variant="secondary"
                        size="sm"
                        onClick={() => void refreshLists(true)}
                      >
                        {t("common.retry")}
                      </BetterButton>
                    </div>
                  )}

                  {friendList && (
                    <>
                      {/* 群组分区 */}
                      {groups.length > 0 && (
                        <>
                          <p className="mb-1.5 mt-1 text-[11px] font-semibold uppercase tracking-wide text-brand-400 dark:text-brand-500">
                            {t("friendsChat.groups")}
                            {" "}
                            —
                            {groups.length}
                          </p>
                          <div className="mb-3 flex flex-col gap-1">
                            {groups.map(group => (
                              <button
                                key={group.id}
                                type="button"
                                onClick={() =>
                                  void openChat({ kind: "group", group })}
                                className="flex items-center gap-3 rounded-xl p-2 text-left transition-colors hover:bg-brand-100 dark:hover:bg-brand-700/60"
                              >
                                {renderAvatar(group.name, group.avatar)}
                                <div className="min-w-0 flex-1">
                                  <div className="truncate text-sm font-medium text-brand-800 dark:text-brand-100">
                                    {group.name}
                                  </div>
                                  <div className="truncate text-[11px] text-brand-500 dark:text-brand-400">
                                    {t("friendsChat.groupOnline", {
                                      online: group.onlineCount,
                                      total: group.memberCount,
                                    })}
                                  </div>
                                </div>
                                {group.unreadCount > 0 && (
                                  <span className="min-w-[18px] rounded-full bg-error-500 px-1.5 py-0.5 text-center text-[10px] font-bold text-white">
                                    {group.unreadCount > 99
                                      ? "99+"
                                      : group.unreadCount}
                                  </span>
                                )}
                              </button>
                            ))}
                          </div>
                        </>
                      )}

                      {/* 好友分区 */}
                      {friendSections.playing.length === 0
                        && friendSections.online.length === 0
                        && friendSections.offline.length === 0 && (
                        <div className="flex flex-col items-center gap-2 py-10 text-center">
                          <span className="i-mdi-account-multiple-outline text-4xl text-brand-300 dark:text-brand-600" />
                          <p className="whitespace-pre-line text-sm text-brand-500 dark:text-brand-400">
                            {t("friendsChat.noFriends")}
                          </p>
                        </div>
                      )}

                      {(
                        [
                          ["playing", friendSections.playing],
                          ["online", friendSections.online],
                          ["offline", friendSections.offline],
                        ] as const
                      ).map(([key, list]) =>
                        list.length > 0 ? (
                          <div key={key}>
                            <p className="mb-1.5 mt-2 text-[11px] font-semibold uppercase tracking-wide text-brand-400 dark:text-brand-500">
                              {t(`friendsChat.section.${key}`)}
                              {" "}
                              —
                              {list.length}
                            </p>
                            <div className="flex flex-col gap-1">
                              {list.map(friend => (
                                <button
                                  key={friend.id}
                                  type="button"
                                  onClick={() =>
                                    void openChat({ kind: "friend", friend })}
                                  className="flex items-center gap-3 rounded-xl p-2 text-left transition-colors hover:bg-brand-100 dark:hover:bg-brand-700/60"
                                >
                                  <div className="relative">
                                    {renderAvatar(
                                      friend.note || friend.nickname,
                                      friend.avatar,
                                    )}
                                    <span className="absolute -bottom-0.5 -right-0.5">
                                      {statusDot(friend.status)}
                                    </span>
                                  </div>
                                  <div className="min-w-0 flex-1">
                                    <div className="truncate text-sm font-medium text-brand-800 dark:text-brand-100">
                                      {friend.note || friend.nickname}
                                      {friend.note && (
                                        <span className="ml-1 text-[11px] font-normal text-brand-400">
                                          {friend.nickname}
                                        </span>
                                      )}
                                    </div>
                                    <div className="truncate text-[11px] text-brand-500 dark:text-brand-400">
                                      {friend.activity
                                        || friend.lastMessage
                                        || t(
                                          `friendsChat.status.${friend.status || "offline"}`,
                                        )}
                                    </div>
                                  </div>
                                  {friend.unreadCount > 0 && (
                                    <span className="min-w-[18px] rounded-full bg-error-500 px-1.5 py-0.5 text-center text-[10px] font-bold text-white">
                                      {friend.unreadCount > 99
                                        ? "99+"
                                        : friend.unreadCount}
                                    </span>
                                  )}
                                </button>
                              ))}
                            </div>
                          </div>
                        ) : null,
                      )}
                    </>
                  )}
                </div>
              </div>
            )
          )}

          {isLoggedIn && view === "requests" && (
            <div className="min-h-0 flex-1 overflow-y-auto p-4">
              <p className="mb-3 text-sm font-semibold text-brand-800 dark:text-brand-100">
                {t("friendsChat.requestsTitle")}
              </p>
              {(friendList?.pendingRequests?.length ?? 0) === 0 ? (
                <p className="py-8 text-center text-sm text-brand-500 dark:text-brand-400">
                  {t("friendsChat.noRequests")}
                </p>
              ) : (
                <div className="flex flex-col gap-2">
                  {friendList?.pendingRequests?.map(request => (
                    <div
                      key={request.friendshipId}
                      className="flex items-center gap-3 rounded-xl border border-brand-200/70 p-2.5 dark:border-brand-700/70"
                    >
                      {renderAvatar(request.nickname, request.avatar)}
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-sm font-medium text-brand-800 dark:text-brand-100">
                          {request.nickname}
                        </div>
                        <div className="text-[11px] text-brand-500 dark:text-brand-400">
                          UID
                          {" "}
                          {request.uid}
                        </div>
                      </div>
                      <BetterButton
                        variant="primary"
                        size="sm"
                        onClick={() =>
                          void (async () => {
                            try {
                              await AcceptFriendRequest(
                                request.friendshipId,
                                request.uid,
                              );
                              toast.success(t("friendsChat.toastAccepted"));
                              await refreshLists(false);
                            }
                            catch (error) {
                              toast.error(
                                error instanceof Error
                                  ? error.message
                                  : String(error),
                              );
                            }
                          })()}
                      >
                        {t("friendsChat.accept")}
                      </BetterButton>
                      <BetterButton
                        variant="secondary"
                        size="sm"
                        onClick={() =>
                          void (async () => {
                            try {
                              await RejectFriendRequest(request.friendshipId);
                              await refreshLists(false);
                            }
                            catch (error) {
                              toast.error(
                                error instanceof Error
                                  ? error.message
                                  : String(error),
                              );
                            }
                          })()}
                      >
                        {t("friendsChat.reject")}
                      </BetterButton>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {isLoggedIn && view === "add" && (
            <div className="min-h-0 flex-1 overflow-y-auto p-4">
              <p className="mb-3 text-sm font-semibold text-brand-800 dark:text-brand-100">
                {t("friendsChat.addFriendTitle")}
              </p>
              <div className="flex gap-2">
                <BetterInput
                  value={searchKeyword}
                  onChange={e => setSearchKeyword(e.target.value)}
                  placeholder={t("friendsChat.searchPlaceholder")}
                  fullWidth
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      void handleSearch();
                    }
                  }}
                />
                <BetterButton
                  variant="primary"
                  className="shrink-0"
                  isLoading={isSearching}
                  onClick={() => void handleSearch()}
                >
                  {t("friendsChat.search")}
                </BetterButton>
              </div>

              <div className="mt-4 flex flex-col gap-2">
                {searchResults?.length === 0 && (
                  <p className="py-6 text-center text-sm text-brand-500 dark:text-brand-400">
                    {t("friendsChat.noSearchResults")}
                  </p>
                )}
                {searchResults?.map(user => (
                  <div
                    key={user.id}
                    className="flex items-center gap-3 rounded-xl border border-brand-200/70 p-2.5 dark:border-brand-700/70"
                  >
                    {renderAvatar(user.nickname, user.avatar)}
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium text-brand-800 dark:text-brand-100">
                        {user.nickname}
                      </div>
                      <div className="text-[11px] text-brand-500 dark:text-brand-400">
                        UID
                        {" "}
                        {user.uid}
                        {user.signature ? ` · ${user.signature}` : ""}
                      </div>
                    </div>
                    <BetterButton
                      variant="primary"
                      size="sm"
                      icon="i-mdi-account-plus"
                      onClick={() =>
                        void (async () => {
                          try {
                            await SendFriendRequest(String(user.uid));
                            toast.success(t("friendsChat.toastRequestSent"));
                          }
                          catch (error) {
                            toast.error(
                              error instanceof Error
                                ? error.message
                                : String(error),
                            );
                          }
                        })()}
                    >
                      {t("friendsChat.sendRequest")}
                    </BetterButton>
                  </div>
                ))}
              </div>
            </div>
          )}

          {isLoggedIn && view === "chat" && target && (
            <div className="flex min-h-0 flex-1 flex-col">
              <div
                ref={listRef}
                className="min-h-0 flex-1 space-y-3 overflow-y-auto p-4"
              >
                {hasMoreHistory && (
                  <div className="flex justify-center">
                    <BetterButton
                      variant="secondary"
                      size="sm"
                      isLoading={historyLoading}
                      onClick={() => void loadOlderHistory()}
                    >
                      {t("friendsChat.loadOlder")}
                    </BetterButton>
                  </div>
                )}
                {historyLoading && messages.length === 0 && (
                  <p className="py-8 text-center text-sm text-brand-500">
                    {t("friendsChat.loading")}
                  </p>
                )}
                {!historyLoading && messages.length === 0 && (
                  <p className="py-8 text-center text-sm text-brand-500 dark:text-brand-400">
                    {t("friendsChat.startChat")}
                  </p>
                )}
                {messages.map(message =>
                  renderMessageBubble(message, target.kind === "group"),
                )}
              </div>

              {/* 回复预览 */}
              {replyTo && (
                <div className="flex items-center gap-2 border-t border-brand-200/70 px-4 py-2 dark:border-brand-700/70">
                  <span className="i-mdi-reply text-brand-400" />
                  <span className="min-w-0 flex-1 truncate text-[11px] text-brand-500 dark:text-brand-400">
                    {replyTo.senderName || `UID ${replyTo.senderUid}`}
                    {": "}
                    {replyTo.content}
                  </span>
                  <button
                    type="button"
                    onClick={() => setReplyTo(null)}
                    aria-label={t("common.cancel")}
                    className="text-brand-400 hover:text-brand-600 dark:hover:text-brand-300"
                  >
                    <span className="i-mdi-close text-sm" />
                  </button>
                </div>
              )}

              {/* 表情 / 贴纸选择面板（与手机版双 tab 一致） */}
              {emojiPanelOpen && (
                <div className="border-t border-brand-200/80 bg-brand-50/80 p-3 dark:border-brand-700/80 dark:bg-brand-900/40">
                  <div className="mb-2 flex items-center gap-2">
                    {emojiTab === 2 ? (
                      <button
                        type="button"
                        onClick={() => setEmojiTab(1)}
                        className="text-xs text-primary-600 hover:underline dark:text-primary-300"
                      >
                        {t("friendsChat.backToPacks")}
                      </button>
                    ) : (
                      <>
                        <button
                          type="button"
                          onClick={() => {
                            setEmojiTab(0);
                            void loadEmojiPanelData();
                          }}
                          className={`rounded-lg px-3 py-1 text-xs font-medium transition-colors ${
                            emojiTab === 0
                              ? "bg-primary-500 text-white"
                              : "text-brand-600 hover:bg-brand-100 dark:text-brand-300 dark:hover:bg-brand-700/60"
                          }`}
                        >
                          {t("friendsChat.emojiTabLocal")}
                        </button>
                        {stickerEnabled && (
                          <button
                            type="button"
                            onClick={() => {
                              setEmojiTab(1);
                              void loadEmojiPanelData();
                            }}
                            className={`rounded-lg px-3 py-1 text-xs font-medium transition-colors ${
                              emojiTab === 1
                                ? "bg-primary-500 text-white"
                                : "text-brand-600 hover:bg-brand-100 dark:text-brand-300 dark:hover:bg-brand-700/60"
                            }`}
                          >
                            {t("friendsChat.emojiTabStickers")}
                          </button>
                        )}
                      </>
                    )}
                    <button
                      type="button"
                      onClick={() => setEmojiPanelOpen(false)}
                      aria-label={t("common.close")}
                      className="ml-auto rounded-lg p-1.5 text-brand-400 transition-colors hover:bg-brand-100 hover:text-brand-600 dark:hover:bg-brand-700"
                    >
                      <span className="i-mdi-close text-sm" />
                    </button>
                  </div>

                  {stickerLoading && (
                    <p className="py-6 text-center text-xs text-brand-500">
                      {t("friendsChat.loading")}
                    </p>
                  )}

                  {/* tab 0：本站表情网格 */}
                  {emojiTab === 0 && (
                    <div className="grid max-h-48 grid-cols-6 gap-1 overflow-y-auto sm:grid-cols-8">
                      {emojis === null && (
                        <p className="col-span-full py-4 text-center text-xs text-brand-500">
                          {t("friendsChat.loading")}
                        </p>
                      )}
                      {emojis?.length === 0 && (
                        <p className="col-span-full py-4 text-center text-xs text-brand-500 dark:text-brand-400">
                          {t("friendsChat.noEmojis")}
                        </p>
                      )}
                      {emojis?.map(emoji => (
                        <button
                          key={emoji.name}
                          type="button"
                          title={emoji.name}
                          onClick={() => void sendEmoji(emoji.name)}
                          className="flex items-center justify-center rounded-lg p-1 transition-colors hover:bg-brand-100 dark:hover:bg-brand-700/60"
                        >
                          <img
                            src={proxiedImageSrc(
                              emoji.url || emojiDisplayURL(emoji.name),
                            )}
                            alt={emoji.name}
                            className="h-9 w-9 object-contain"
                          />
                        </button>
                      ))}
                    </div>
                  )}

                  {/* tab 1：未萌贴纸包列表 */}
                  {emojiTab === 1 && (
                    <div className="grid max-h-48 grid-cols-3 gap-2 overflow-y-auto sm:grid-cols-4">
                      {stickerPacks.map(pack => (
                        <button
                          key={pack.id}
                          type="button"
                          onClick={() => void openStickerPack(pack)}
                          className="flex items-center gap-2 rounded-xl border border-brand-200/70 p-1.5 text-left transition-colors hover:bg-brand-100 dark:border-brand-700/70 dark:hover:bg-brand-700/60"
                        >
                          <img
                            src={proxiedImageSrc(pack.cover)}
                            alt=""
                            className="h-10 w-10 shrink-0 rounded-lg object-cover"
                          />
                          <span className="min-w-0 flex-1 truncate text-[11px] font-medium text-brand-700 dark:text-brand-200">
                            {pack.title}
                          </span>
                        </button>
                      ))}
                    </div>
                  )}

                  {/* tab 2：包内表情网格（点击直接发 URL） */}
                  {emojiTab === 2 && activeStickerPack && (
                    <div className="grid max-h-48 grid-cols-6 gap-1 overflow-y-auto sm:grid-cols-8">
                      {stickerURLs.map(url => (
                        <button
                          key={url}
                          type="button"
                          onClick={() => void sendEmoji(url)}
                          className="flex items-center justify-center rounded-lg p-1 transition-colors hover:bg-brand-100 dark:hover:bg-brand-700/60"
                        >
                          <img
                            src={proxiedImageSrc(url)}
                            alt=""
                            className="h-9 w-9 object-contain"
                          />
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* 输入区 */}
              <div className="flex items-center gap-2 border-t border-brand-200/80 p-3 dark:border-brand-700/80">
                <button
                  type="button"
                  onClick={() => {
                    setEmojiPanelOpen(open => !open);
                    void loadEmojiPanelData();
                  }}
                  aria-label={t("friendsChat.emojiPicker")}
                  className={`shrink-0 rounded-lg p-2 transition-colors ${
                    emojiPanelOpen
                      ? "bg-primary-500/15 text-primary-600 dark:text-primary-300"
                      : "text-brand-500 hover:bg-brand-100 hover:text-brand-700 dark:text-brand-400 dark:hover:bg-brand-700"
                  }`}
                >
                  <span className="i-mdi-emoticon-outline text-lg" />
                </button>
                <button
                  type="button"
                  onClick={() => void pickAndSendImage()}
                  disabled={isUploadingImage}
                  aria-label={t("friendsChat.sendImage")}
                  className="shrink-0 rounded-lg p-2 text-brand-500 transition-colors hover:bg-brand-100 hover:text-brand-700 disabled:opacity-50 dark:text-brand-400 dark:hover:bg-brand-700"
                >
                  <span
                    className={`${isUploadingImage ? "i-mdi-loading animate-spin" : "i-mdi-image-outline"} text-lg`}
                  />
                </button>
                <BetterInput
                  value={draft.text}
                  onChange={e => setDraft({ text: e.target.value })}
                  placeholder={t("friendsChat.inputPlaceholder")}
                  fullWidth
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && !e.shiftKey) {
                      e.preventDefault();
                      void handleSend();
                    }
                  }}
                />
                <BetterButton
                  variant="primary"
                  className="shrink-0"
                  icon="i-mdi-send"
                  isLoading={isSending}
                  disabled={!draft.text.trim()}
                  onClick={() => void handleSend()}
                />
              </div>
            </div>
          )}
        </div>
      </div>
    </ModalPortal>
  );
}
