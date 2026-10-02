import { Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { GetChatUnreadCount } from "../../../bindings/yukihub/internal/service/accountservice";
import { onWailsEvent } from "../../../src/bindings/runtime";
import { useAccountStatus } from "../../hooks/useAccountStatus";
import { useAppStore } from "../../store";
import { SnowflakeMark } from "../branding/SnowflakeMark";
import { FriendsChatModal } from "../modal/FriendsChatModal";

interface SideBarProps {
  bgEnabled?: boolean;
  bgOpacity?: number;
}

export function SideBar({ bgEnabled = false, bgOpacity = 0.85 }: SideBarProps) {
  const { t } = useTranslation();
  const isSidebarOpen = useAppStore(state => state.isSidebarOpen);
  const toggleSidebar = useAppStore(state => state.toggleSidebar);
  const [activeDownloads, setActiveDownloads] = useState(0);
  const accountStatus = useAccountStatus();
  const isLoggedIn = Boolean(accountStatus?.logged_in);
  const [chatUnread, setChatUnread] = useState(0);
  const [chatOpen, setChatOpen] = useState(false);

  const [prevSidebarOpen, setPrevSidebarOpen] = useState(isSidebarOpen);
  const [delayedOpen, setDelayedOpen] = useState(isSidebarOpen);
  const [isFading, setIsFading] = useState(false);

  // 在渲染阶段直接更新派生状态，React 官方推荐做法，可避免 Effect 带来的多余重绘并解决 ESLint 警告
  if (isSidebarOpen !== prevSidebarOpen) {
    setPrevSidebarOpen(isSidebarOpen);
    setIsFading(true);
  }

  useEffect(() => {
    if (isFading) {
      const timer = setTimeout(() => {
        setDelayedOpen(isSidebarOpen);
        setIsFading(false);
      }, 150); // 与 300ms 过渡的一半时间相对应
      return () => clearTimeout(timer);
    }
  }, [isFading, isSidebarOpen]);

  // 监听下载进度事件，统计进行中的任务数
  useEffect(() => {
    const counts: Record<string, string> = {};
    const unsubscribe = onWailsEvent(
      "download:progress",
      (evt: { id: string; status: string }) => {
        counts[evt.id] = evt.status;
        const active = Object.values(counts).filter(
          s => s === "downloading" || s === "pending" || s === "extracting",
        ).length;
        setActiveDownloads(active);
      },
    );
    return unsubscribe;
  }, []);

  // 聊天未读数：登录后每 45 秒拉一次（对齐手机版侧栏徽标；
  // 打开聊天弹窗时不拉，避免覆盖弹窗内的会话已读状态）
  useEffect(() => {
    if (!isLoggedIn || chatOpen) {
      setChatUnread(0);
      return;
    }
    let cancelled = false;
    const refresh = () => {
      GetChatUnreadCount()
        .then((count) => {
          if (!cancelled) {
            setChatUnread(count);
          }
        })
        .catch(() => {
          // 静默：网络抖动不该让侧栏报错
        });
    };
    refresh();
    const timer = window.setInterval(refresh, 45_000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [isLoggedIn, chatOpen]);

  const navItems = [
    { to: "/", label: t("sideBar.home"), icon: "i-mdi-home" },
    {
      to: "/library",
      label: t("sideBar.library"),
      icon: "i-mdi-gamepad-variant",
    },
    { to: "/stats", label: t("sideBar.stats"), icon: "i-mdi-chart-bar" },
    {
      to: "/categories",
      label: t("sideBar.categories"),
      icon: "i-mdi-format-list-bulleted",
    },
  ];

  const sidebarBgClass = bgEnabled
    ? "border-r border-white/20 dark:border-white/10"
    : "bg-white dark:bg-yh-sidebar border-r border-brand-200 dark:border-brand-700";

  const sidebarStyle = bgEnabled
    ? {
        backgroundColor: `rgba(var(--sidebar-bg-rgb), ${bgOpacity})`,
        transition: "width 300ms ease",
        width: isSidebarOpen ? "16rem" : "4rem",
      }
    : {
        transition: "width 300ms ease",
        width: isSidebarOpen ? "16rem" : "4rem",
      };
  // 选中态对齐手机版 bg_sidebar_item：半透明蓝底 + 左侧强调条，
  // 而不是上游那种"整块实心灰底"。
  const navItemClass
    = "flex items-center rounded-lg border-l-2 border-transparent p-2 text-brand-600 no-underline transition-colors hover:bg-brand-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-400/70 dark:text-brand-300 dark:hover:bg-brand-700/70 [&.active]:border-primary-500 [&.active]:bg-primary-500/12 [&.active]:font-medium [&.active]:text-primary-700 dark:[&.active]:border-primary-300 dark:[&.active]:bg-primary-300/15 dark:[&.active]:text-primary-200 data-glass:hover:bg-white/10 data-glass:hover:dark:bg-black/10 data-glass:[&.active]:bg-white/20 data-glass:[&.active]:dark:bg-black/20";
  const footerActionClass
    = "relative flex items-center justify-center rounded-lg border-l-2 border-transparent p-2.5 text-brand-600 transition-colors hover:bg-brand-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-400/70 dark:text-brand-300 dark:hover:bg-brand-700/70 [&.active]:border-primary-500 [&.active]:bg-primary-500/12 [&.active]:text-primary-700 dark:[&.active]:border-primary-300 dark:[&.active]:bg-primary-300/15 dark:[&.active]:text-primary-200 data-glass:hover:bg-white/10 data-glass:hover:dark:bg-black/10";
  return (
    <aside
      className={`relative z-30 flex shrink-0 flex-col ${sidebarBgClass}`}
      style={sidebarStyle}
    >
      <div
        className={`flex items-center h-16 px-3 ${bgEnabled ? "border-white/20 dark:border-white/10" : "border-brand-200 dark:border-brand-700"}`}
      >
        <div
          className={`flex items-center gap-1 select-none overflow-hidden transition-all duration-300 ${isSidebarOpen ? "opacity-100 max-w-[200px] pl-1" : "opacity-0 max-w-0 pl-0"}`}
        >
          <SnowflakeMark className="h-8 w-8 shrink-0 text-primary-500 dark:text-primary-300" />
          <span className="shrink-0 text-[15px] font-medium tracking-wide text-brand-900 dark:text-brand-100">
            YukiHub
          </span>
        </div>
        <div
          className={`flex-1 flex items-center min-w-[40px] ${isSidebarOpen ? "justify-end" : "justify-center"}`}
        >
          <button
            type="button"
            onClick={toggleSidebar}
            className="shrink-0 rounded-xl p-2 text-brand-700 transition-colors hover:bg-brand-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-400/70 dark:text-brand-300 dark:hover:bg-brand-700 select-none data-glass:hover:bg-white/10 data-glass:hover:dark:bg-black/10"
            aria-label={t("sideBar.toggle")}
            onDragStart={e => e.preventDefault()}
          >
            <div
              className="i-mdi-menu text-xl pointer-events-none"
              aria-hidden="true"
            />
          </button>
        </div>
      </div>

      <nav className="flex-1 py-4">
        <ul className="space-y-2 px-2">
          {navItems.map(item => (
            <li key={item.to}>
              <Link
                to={item.to}
                className={`${navItemClass} select-none`}
                onDragStart={e => e.preventDefault()}
              >
                <div className="relative shrink-0 flex items-center justify-center w-8 h-8">
                  <div
                    className={`${item.icon} text-xl pointer-events-none`}
                    aria-hidden="true"
                  />
                </div>
                <div
                  className={`overflow-hidden transition-all duration-300 flex items-center ${isSidebarOpen ? "w-[120px] ml-2 opacity-100" : "w-0 ml-0 opacity-0"}`}
                >
                  <span className="pointer-events-none whitespace-nowrap shrink-0 break-keep">
                    {item.label}
                  </span>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      </nav>

      <div
        className={`p-4 ${bgEnabled ? "border-white/20 dark:border-white/10" : "border-brand-200 dark:border-brand-700"}`}
      >
        <div
          className={`flex items-center transition-opacity duration-150 ${delayedOpen ? "justify-end gap-1" : "flex-col gap-2"} ${isFading ? "opacity-0" : "opacity-100"}`}
        >
          {/* 云同步状态已并入首页用户区（AccountModal），侧栏不再放第二入口 */}

          <button
            type="button"
            onClick={() => setChatOpen(true)}
            aria-label={t("sideBar.chat")}
            className={`${footerActionClass} select-none`}
          >
            <div className="relative shrink-0">
              <div
                className="i-mdi-chat-processing-outline text-xl pointer-events-none"
                aria-hidden="true"
              />
              {chatUnread > 0 && (
                <span className="absolute -top-1.5 -right-1.5 min-w-[16px] h-4 px-1 flex items-center justify-center bg-error-500 text-white text-[10px] font-bold rounded-full leading-none pointer-events-none">
                  {chatUnread > 99 ? "99+" : chatUnread}
                </span>
              )}
            </div>
            {isSidebarOpen && delayedOpen && (
              <span className="ml-2 overflow-hidden whitespace-nowrap text-sm">
                {t("sideBar.chat")}
              </span>
            )}
          </button>

          <Link
            to="/downloads"
            className={`${footerActionClass} no-underline select-none data-glass:[&.active]:bg-white/20 data-glass:[&.active]:dark:bg-black/20`}
            onDragStart={e => e.preventDefault()}
          >
            <div className="relative shrink-0">
              <div
                className="i-mdi-download text-xl pointer-events-none"
                aria-hidden="true"
              />
              {activeDownloads > 0 && (
                <span className="absolute -top-1.5 -right-1.5 min-w-[16px] h-4 px-1 flex items-center justify-center bg-blue-500 text-white text-[10px] font-bold rounded-full leading-none pointer-events-none">
                  {activeDownloads > 99 ? "99+" : activeDownloads}
                </span>
              )}
            </div>
          </Link>
          <Link
            to="/settings"
            className={`${footerActionClass} no-underline select-none data-glass:[&.active]:bg-white/20 data-glass:[&.active]:dark:bg-black/20`}
            onDragStart={e => e.preventDefault()}
          >
            <div
              className="i-mdi-cog text-xl pointer-events-none"
              aria-hidden="true"
            />
          </Link>
        </div>
      </div>

      <FriendsChatModal
        isOpen={chatOpen}
        isLoggedIn={isLoggedIn}
        onClose={() => setChatOpen(false)}
      />
    </aside>
  );
}
