/**
 * 事件名常量，用来解耦「谁触发的」与「谁响应」。
 *
 * 分两类：
 * - **后端事件**（下面的 `*_EVENT`）：Wails 从 Go 侧广播，前端用
 *   `onWailsEvent` 订阅；名字必须与 Go 侧常量一致，改动时两边一起改。
 * - 窗口自定义事件（`window.dispatchEvent`）：同一窗口内跨组件通信用，
 *   目前没有用例，需要时再加。
 */

/** 打开好友面板 —— Go 侧 `service.OpenFriendsPanelEvent`。 */
export const OPEN_FRIENDS_PANEL_EVENT = "friend:open-panel";

/** 好友列表有变化 —— Go 侧 `friendListUpdatedEvent`，推的是整份列表。 */
export const FRIEND_LIST_UPDATED_EVENT = "friend:list-updated";

/** 好友开始玩游戏 —— Go 侧 `service.FriendPlayNoticeEvent`。 */
export const FRIEND_PLAY_EVENT = "friend:playing";
