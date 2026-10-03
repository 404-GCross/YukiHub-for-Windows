import React from "react";
import { createRoot } from "react-dom/client";
import "@unocss/reset/tailwind.css";
import "virtual:uno.css";
import "./style.css";
import "./i18n";

const container = document.getElementById("root");

const root = createRoot(container!);

const pathname = window.location.pathname;
const isStartupWindow = pathname.startsWith("/startup");
// 游戏内好友栏是独立的置顶窗口（Go 侧 main.go 创建），走自己的轻量入口：
// 它不需要整个应用外壳，只画一个好友列表。
const isOverlayWindow = pathname.startsWith("/overlay");
// 好友通知浮层同理，也是 Go 侧创建的独立置顶窗口（URL 走 /notice），
// 只画一张卡片。
const isNoticeWindow = pathname.startsWith("/notice");

async function mountApplication() {
  const { default: ApplicationRoot } = isStartupWindow
    ? await import("./components/startup/StartupWindow")
    : isOverlayWindow
      ? await import("./components/overlay/FriendsOverlay")
      : isNoticeWindow
        ? await import("./components/notice/FriendPlayNoticeWindow")
        : await import("./App");

  root.render(
    <React.StrictMode>
      <ApplicationRoot />
    </React.StrictMode>,
  );

  container?.classList.add("ready");
}

void mountApplication();
