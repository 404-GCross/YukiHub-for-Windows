import { useCallback, useEffect, useRef, useState } from "react";

import type {
  FocusDirection,
  FocusEngineOptions,
  FocusPosition,
  FocusZone,
} from "./focusEngine";

import { FocusEngine } from "./focusEngine";

export type UseFocusEngineOptions = {
  onBoundary?: FocusEngineOptions["onBoundary"];
  /** 需要按引用稳定（模块常量或 useMemo） */
  verticalOrder?: string[];
  /** 需要按引用稳定（useMemo），否则每次渲染都会触发区域同步 */
  zones: FocusZone[];
};

export type UseFocusEngineResult = {
  focus: (zoneId: string, index: number) => void;
  move: (direction: FocusDirection) => void;
  position: FocusPosition;
  restoreMemory: (key: string) => boolean;
  saveMemory: (key: string) => void;
};

/**
 * 把 FocusEngine 接到 React：引擎实例只创建一次，区域数据与回调都走 ref，
 * 避免把引擎状态放进渲染路径。
 */
export function useFocusEngine(
  options: UseFocusEngineOptions,
): UseFocusEngineResult {
  const boundaryRef = useRef(options.onBoundary);
  const setPositionRef = useRef<(next: FocusPosition) => void>(() => {});
  const engineRef = useRef<FocusEngine | null>(null);

  if (!engineRef.current) {
    engineRef.current = new FocusEngine({
      onBoundary: (zoneId, direction, current) =>
        boundaryRef.current?.(zoneId, direction, current) ?? false,
      onFocusChange: next => setPositionRef.current(next),
      verticalOrder: options.verticalOrder,
      zones: options.zones,
    });
  }

  const engine = engineRef.current;
  const [position, setPosition] = useState<FocusPosition>(() =>
    engine.getPosition(),
  );

  useEffect(() => {
    boundaryRef.current = options.onBoundary;
  }, [options.onBoundary]);

  useEffect(() => {
    setPositionRef.current = setPosition;
  }, [setPosition]);

  useEffect(() => {
    engine.setZones(options.zones);
  }, [engine, options.zones]);

  useEffect(() => {
    engine.setVerticalOrder(options.verticalOrder ?? []);
  }, [engine, options.verticalOrder]);

  const move = useCallback(
    (direction: FocusDirection) => {
      engine.move(direction);
    },
    [engine],
  );

  const focus = useCallback(
    (zoneId: string, index: number) => {
      engine.focus(zoneId, index);
    },
    [engine],
  );

  const saveMemory = useCallback(
    (key: string) => {
      engine.saveMemory(key);
    },
    [engine],
  );

  const restoreMemory = useCallback(
    (key: string) => engine.restoreMemory(key),
    [engine],
  );

  return { focus, move, position, restoreMemory, saveMemory };
}
