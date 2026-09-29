import { useEffect, useRef, useState } from "react";

import type { FocusDirection } from "./focusEngine";

/** 大屏模式的输入意图：键盘与手柄翻译成同一套语义后再分发 */
export type BigScreenIntent
  = | { direction: FocusDirection; type: "move" }
    | { delta: number; type: "category" }
    | { type: "back" }
    | { type: "confirm" }
    | { type: "details" }
    | { type: "favorite" };

/** 最近一次使用的输入设备，决定提示条显示手柄图标还是键盘按键 */
export type BigScreenInputDevice = "gamepad" | "keyboard";

type PadAction
  = | "a"
    | "b"
    | "down"
    | "lb"
    | "left"
    | "rb"
    | "right"
    | "up"
    | "x"
    | "y";

type UseGamepadOptions = {
  enabled?: boolean;
  onIntent: (intent: BigScreenIntent) => void;
};

/** 长按多久后开始连发，对齐手机端 InputRouter 的 400ms */
const REPEAT_DELAY_MS = 400;

/** 连发间隔，对齐手机端 InputRouter 的 80ms */
const REPEAT_INTERVAL_MS = 80;

/** 左摇杆判定阈值 */
const AXIS_THRESHOLD = 0.5;

/** 标准映射（Standard Gamepad）的按钮下标 → 动作 */
const BUTTON_ACTIONS: Record<number, PadAction> = {
  0: "a",
  1: "b",
  2: "x",
  3: "y",
  4: "lb",
  5: "rb",
  12: "up",
  13: "down",
  14: "left",
  15: "right",
};

/** 只有方向与切分类需要长按连发；确认/返回/收藏/详情连发会误触 */
const REPEATABLE_ACTIONS = new Set<PadAction>([
  "down",
  "lb",
  "left",
  "rb",
  "right",
  "up",
]);

function actionToIntent(action: PadAction): BigScreenIntent {
  switch (action) {
    case "a": {
      return { type: "confirm" };
    }
    case "b": {
      return { type: "back" };
    }
    case "down": {
      return { direction: "down", type: "move" };
    }
    case "lb": {
      return { delta: -1, type: "category" };
    }
    case "left": {
      return { direction: "left", type: "move" };
    }
    case "rb": {
      return { delta: 1, type: "category" };
    }
    case "right": {
      return { direction: "right", type: "move" };
    }
    case "up": {
      return { direction: "up", type: "move" };
    }
    case "x": {
      return { type: "favorite" };
    }
    case "y": {
      return { type: "details" };
    }
  }
}

/** 当前帧处于按下状态的动作集合：按钮按下（含扳机）与左摇杆推到底 */
function readActiveActions(pad: Gamepad) {
  const active = new Set<PadAction>();

  pad.buttons.forEach((button, index) => {
    const action = BUTTON_ACTIONS[index];
    if (action && (button.pressed || button.value > AXIS_THRESHOLD)) {
      active.add(action);
    }
  });

  const [axisX = 0, axisY = 0] = pad.axes;
  if (Math.abs(axisX) > Math.abs(axisY)) {
    if (axisX > AXIS_THRESHOLD) {
      active.add("right");
    }
    else if (axisX < -AXIS_THRESHOLD) {
      active.add("left");
    }
  }
  else if (axisY > AXIS_THRESHOLD) {
    active.add("down");
  }
  else if (axisY < -AXIS_THRESHOLD) {
    active.add("up");
  }

  return active;
}

function findConnectedGamepad() {
  return Array.from(navigator.getGamepads()).find((item): item is Gamepad =>
    Boolean(item?.connected),
  );
}

/**
 * 手柄输入，对齐手机端 `InputRouter` 的按键映射与长按连发。
 *
 * 用 `requestAnimationFrame` 轮询 Gamepad API（Web 侧没有按键事件），上升沿立刻
 * 触发一次，按住超过 400ms 后每 80ms 连发；只在方向与切分类上连发。
 */
export function useGamepad({ enabled = true, onIntent }: UseGamepadOptions) {
  const intentRef = useRef(onIntent);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    intentRef.current = onIntent;
  }, [onIntent]);

  useEffect(() => {
    if (!enabled || typeof navigator.getGamepads !== "function") {
      return;
    }

    const holds = new Map<PadAction, number>();
    let connectedNow = false;
    let frame = 0;

    const poll = () => {
      const pad = findConnectedGamepad();
      if (Boolean(pad) !== connectedNow) {
        connectedNow = Boolean(pad);
        setConnected(connectedNow);
      }

      const now = performance.now();
      const active = pad ? readActiveActions(pad) : new Set<PadAction>();

      active.forEach((action) => {
        const nextRepeatAt = holds.get(action);
        if (nextRepeatAt === undefined) {
          holds.set(action, now + REPEAT_DELAY_MS);
          intentRef.current(actionToIntent(action));
          return;
        }
        if (REPEATABLE_ACTIONS.has(action) && now >= nextRepeatAt) {
          holds.set(action, now + REPEAT_INTERVAL_MS);
          intentRef.current(actionToIntent(action));
        }
      });

      holds.forEach((_, action) => {
        if (!active.has(action)) {
          holds.delete(action);
        }
      });

      frame = window.requestAnimationFrame(poll);
    };

    frame = window.requestAnimationFrame(poll);
    return () => {
      window.cancelAnimationFrame(frame);
    };
  }, [enabled]);

  return { connected };
}
