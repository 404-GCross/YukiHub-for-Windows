import { useEffect, useLayoutEffect, useRef } from "react";

export interface ContextMenuItem {
  key: string;
  label: string;
  /** UnoCSS 图标类，如 i-mdi-account-details-outline */
  icon?: string;
  danger?: boolean;
  disabled?: boolean;
  onSelect: () => void;
}

interface ContextMenuProps {
  items: ContextMenuItem[];
  /** 鼠标位置；null 表示不显示 */
  position: { x: number; y: number } | null;
  onClose: () => void;
}

const VIEWPORT_PADDING = 8;

/**
 * 通用右键菜单。
 *
 * 与聊天消息菜单（ChatMessageMenu）视觉一致，但不做业务耦合 —— 好友列表项、
 * 聊天成员等任何「右键出一组动作」的地方都能用。
 *
 * 比 ChatMessageMenu 多做了**贴边翻转**：菜单贴到视口右下角时会自动往回收，
 * 否则在靠近右边缘的列表里点右键，菜单会有一半跑到屏幕外。
 */
export function ContextMenu({ items, position, onClose }: ContextMenuProps) {
  const menuRef = useRef<HTMLDivElement | null>(null);

  // 点外部 / Esc 关闭
  useEffect(() => {
    if (!position) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      if (menuRef.current?.contains(event.target as Node)) {
        return;
      }
      onClose();
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("mousedown", handlePointerDown);
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("mousedown", handlePointerDown);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [position, onClose]);

  // 贴边翻转。直接改元素样式而不是存进 state：翻转只发生在 layout 阶段，
  // 存 state 会多一次渲染，还容易撞上 no-direct-set-state-in-use-effect。
  useLayoutEffect(() => {
    const element = menuRef.current;
    if (!element || !position) {
      return;
    }
    // 先回到鼠标位置再测量：连续打开时，上一次翻转留下的偏移会干扰判断
    element.style.left = `${position.x}px`;
    element.style.top = `${position.y}px`;

    const { offsetWidth: width, offsetHeight: height } = element;
    let left = position.x;
    let top = position.y;
    if (left + width + VIEWPORT_PADDING > window.innerWidth) {
      left = Math.max(
        VIEWPORT_PADDING,
        window.innerWidth - width - VIEWPORT_PADDING,
      );
    }
    if (top + height + VIEWPORT_PADDING > window.innerHeight) {
      top = Math.max(
        VIEWPORT_PADDING,
        window.innerHeight - height - VIEWPORT_PADDING,
      );
    }
    element.style.left = `${left}px`;
    element.style.top = `${top}px`;
  }, [position, items.length]);

  if (!position || items.length === 0) {
    return null;
  }

  return (
    <div
      ref={menuRef}
      role="menu"
      className="fixed z-[80] min-w-[156px] overflow-hidden rounded-xl border border-brand-200/90 bg-white py-1 shadow-xl dark:border-brand-700/90 dark:bg-brand-800"
      style={{ left: position.x, top: position.y }}
    >
      {items.map(item => (
        <button
          key={item.key}
          type="button"
          role="menuitem"
          disabled={item.disabled}
          onClick={() => {
            if (item.disabled) {
              return;
            }
            onClose();
            item.onSelect();
          }}
          className={`flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors ${
            item.disabled
              ? "cursor-not-allowed text-brand-400 dark:text-brand-500"
              : item.danger
                ? "text-error-600 hover:bg-brand-100 dark:text-error-400 dark:hover:bg-brand-700/70"
                : "text-brand-800 hover:bg-brand-100 dark:text-brand-100 dark:hover:bg-brand-700/70"
          }`}
        >
          {item.icon && (
            <span className={`${item.icon} text-base`} aria-hidden="true" />
          )}
          <span className="min-w-0 flex-1 truncate">{item.label}</span>
        </button>
      ))}
    </div>
  );
}
