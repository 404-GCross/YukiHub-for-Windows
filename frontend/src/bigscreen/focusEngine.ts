/**
 * TS 侧复刻手机端 bigscreen 的 FocusEngine 逻辑模型。
 *
 * 与手机端一致：引擎不持有任何视图，只维护「区域 → 每行条目数 → 焦点下标」这套纯逻辑，
 * 滚动、动效与音效全部由上层的 React 组件负责。支持网格与「每行不等长」两种排布、
 * 跨行夹紧列（回到长行时恢复原列）、边界回调与按分类记忆焦点。
 */

export type FocusDirection = "down" | "left" | "right" | "up";

export type FocusPosition = {
  /** 区域内的扁平下标（跨行累计） */
  index: number;
  zoneId: string;
};

export type FocusZone = {
  /** 区域标识，例如内容货架 / 侧栏 / 按钮排 */
  id: string;
  /** 每一行的条目数，数组长度即行数 */
  rowLengths: number[];
};

export type FocusEngineOptions = {
  /**
   * 在区域内容边界继续移动时触发。
   * 返回 true 表示调用方已接管该次移动，引擎不再执行默认的夹紧行为。
   */
  onBoundary?: (
    zoneId: string,
    direction: FocusDirection,
    position: FocusPosition,
  ) => boolean;
  onFocusChange?: (next: FocusPosition, previous: FocusPosition | null) => void;
  /** 上下方向在区域之间穿梭的顺序，缺省为区域的注册顺序 */
  verticalOrder?: string[];
  zones: FocusZone[];
};

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

function countItems(zone: FocusZone) {
  return zone.rowLengths.reduce(
    (total, length) => total + Math.max(0, length),
    0,
  );
}

function rowLengthAt(zone: FocusZone, row: number) {
  return Math.max(0, zone.rowLengths[row] ?? 0);
}

function lastNonEmptyRow(zone: FocusZone) {
  for (let row = zone.rowLengths.length - 1; row >= 0; row -= 1) {
    if (rowLengthAt(zone, row) > 0) {
      return row;
    }
  }
  return -1;
}

/** 扁平下标 → 行列；下标越界时落到最后一个有内容的行尾 */
function locate(zone: FocusZone, index: number) {
  let remaining = Math.max(0, index);

  for (let row = 0; row < zone.rowLengths.length; row += 1) {
    const length = rowLengthAt(zone, row);
    if (remaining < length) {
      return { column: remaining, row };
    }
    remaining -= length;
  }

  const row = lastNonEmptyRow(zone);
  return row === -1
    ? { column: 0, row: 0 }
    : { column: rowLengthAt(zone, row) - 1, row };
}

function flatten(zone: FocusZone, row: number, column: number) {
  let base = 0;
  for (
    let current = 0;
    current < row && current < zone.rowLengths.length;
    current += 1
  ) {
    base += rowLengthAt(zone, current);
  }
  return base + clamp(column, 0, Math.max(0, rowLengthAt(zone, row) - 1));
}

function cloneZone(zone: FocusZone): FocusZone {
  return { id: zone.id, rowLengths: [...zone.rowLengths] };
}

export class FocusEngine {
  private desiredColumns = new Map<string, number>();
  private memory = new Map<string, FocusPosition>();
  private state: FocusPosition;

  private onBoundary: FocusEngineOptions["onBoundary"];
  private onFocusChange: FocusEngineOptions["onFocusChange"];

  private verticalOrder: string[];
  private zones: FocusZone[];

  constructor(options: FocusEngineOptions) {
    this.zones = options.zones.map(cloneZone);
    this.verticalOrder = [
      ...(options.verticalOrder ?? this.zones.map(zone => zone.id)),
    ];
    this.onBoundary = options.onBoundary;
    this.onFocusChange = options.onFocusChange;
    this.state = this.firstPosition();
  }

  /** 清空按分类记忆的焦点 */
  clearMemory() {
    this.memory.clear();
  }

  /** 把焦点直接放到指定区域的指定下标（鼠标悬停/点击用） */
  focus(zoneId: string, index: number) {
    const zone = this.findZone(zoneId);
    if (!zone || countItems(zone) === 0) {
      return;
    }

    const next = { index: clamp(index, 0, countItems(zone) - 1), zoneId };
    this.desiredColumns.set(zoneId, locate(zone, next.index).column);
    this.apply(next);
  }

  getPosition(): FocusPosition {
    return { ...this.state };
  }

  getZones(): readonly FocusZone[] {
    return this.zones;
  }

  /** 指定区域内的条目总数 */
  itemCount(zoneId: string) {
    const zone = this.findZone(zoneId);
    return zone ? countItems(zone) : 0;
  }

  move(direction: FocusDirection): FocusPosition {
    const zone = this.findZone(this.state.zoneId);
    if (!zone || countItems(zone) === 0) {
      return this.getPosition();
    }

    const { column, row } = locate(zone, this.state.index);
    if (direction === "left" || direction === "right") {
      this.moveHorizontally(zone, row, column, direction);
    }
    else {
      this.moveVertically(zone, row, column, direction);
    }

    return this.getPosition();
  }

  /** 恢复某个分类上次的焦点位置；无记忆时返回 false */
  restoreMemory(key: string) {
    const saved = this.memory.get(key);
    if (!saved) {
      return false;
    }

    const zone = this.findZone(saved.zoneId);
    if (!zone || countItems(zone) === 0) {
      return false;
    }

    const located = locate(zone, saved.index);
    this.desiredColumns.set(zone.id, located.column);
    this.apply({
      index: clamp(saved.index, 0, countItems(zone) - 1),
      zoneId: saved.zoneId,
    });
    return true;
  }

  /** 记住当前焦点，供切换分类后回位 */
  saveMemory(key: string) {
    this.memory.set(key, this.getPosition());
  }

  /** 更新上下穿梭的区域顺序 */
  setVerticalOrder(verticalOrder: string[]) {
    this.verticalOrder = [...verticalOrder];
  }

  /** 数据变化后同步各区域的行长度，并把越界焦点夹回有效范围 */
  setZones(zones: FocusZone[]) {
    this.zones = zones.map(cloneZone);

    const zone = this.findZone(this.state.zoneId);
    if (!zone || countItems(zone) === 0) {
      this.apply(this.firstPosition());
      return;
    }

    const located = locate(zone, this.state.index);
    this.apply({
      index: flatten(zone, located.row, located.column),
      zoneId: this.state.zoneId,
    });
  }

  private apply(next: FocusPosition) {
    const previous = this.state;
    if (previous.index === next.index && previous.zoneId === next.zoneId) {
      return;
    }

    this.state = next;
    this.onFocusChange?.({ ...next }, previous);
  }

  private findZone(zoneId: string) {
    return this.zones.find(zone => zone.id === zoneId);
  }

  private firstPosition(): FocusPosition {
    for (const zoneId of this.orderedZoneIds()) {
      const zone = this.findZone(zoneId);
      if (zone && countItems(zone) > 0) {
        return { index: 0, zoneId };
      }
    }
    return { index: 0, zoneId: this.orderedZoneIds()[0] ?? "" };
  }

  private moveHorizontally(
    zone: FocusZone,
    row: number,
    column: number,
    direction: FocusDirection,
  ) {
    const nextColumn = column + (direction === "right" ? 1 : -1);
    if (nextColumn < 0 || nextColumn >= rowLengthAt(zone, row)) {
      // 行内已到边界：交给上层（大屏模式下用于左右切换分类）
      this.onBoundary?.(zone.id, direction, this.getPosition());
      return;
    }

    this.desiredColumns.set(zone.id, nextColumn);
    this.apply({ index: flatten(zone, row, nextColumn), zoneId: zone.id });
  }

  private moveVertically(
    zone: FocusZone,
    row: number,
    column: number,
    direction: FocusDirection,
  ) {
    const desired = this.desiredColumns.get(zone.id) ?? column;
    const nextRow = row + (direction === "down" ? 1 : -1);

    if (
      nextRow >= 0
      && nextRow < zone.rowLengths.length
      && rowLengthAt(zone, nextRow) > 0
    ) {
      this.apply({
        index: flatten(
          zone,
          nextRow,
          Math.min(desired, rowLengthAt(zone, nextRow) - 1),
        ),
        zoneId: zone.id,
      });
      return;
    }

    const neighbor = this.verticalNeighbor(zone.id, direction);
    if (neighbor) {
      const targetRow
        = direction === "down"
          ? this.firstNonEmptyRow(neighbor)
          : lastNonEmptyRow(neighbor);
      if (targetRow !== -1) {
        this.desiredColumns.set(neighbor.id, desired);
        this.apply({
          index: flatten(
            neighbor,
            targetRow,
            Math.min(desired, rowLengthAt(neighbor, targetRow) - 1),
          ),
          zoneId: neighbor.id,
        });
        return;
      }
    }

    this.onBoundary?.(zone.id, direction, this.getPosition());
  }

  private firstNonEmptyRow(zone: FocusZone) {
    for (let row = 0; row < zone.rowLengths.length; row += 1) {
      if (rowLengthAt(zone, row) > 0) {
        return row;
      }
    }
    return -1;
  }

  private orderedZoneIds() {
    const ordered = this.verticalOrder.filter(zoneId =>
      this.zones.some(zone => zone.id === zoneId),
    );
    for (const zone of this.zones) {
      if (!ordered.includes(zone.id)) {
        ordered.push(zone.id);
      }
    }
    return ordered;
  }

  /**
   * 允许上下穿梭的区域顺序。
   *
   * **只含显式登记在 `verticalOrder` 里的区域**：侧栏与详情层不参与上下穿梭 ——
   * 侧栏由货架的「←」边界显式进入（对齐手机端的 `ZONE_RAIL`），详情层自己处理 ↑↓。
   * 早期实现会把未登记的区域追加到末尾，于是「货架按 ↓」被引擎丢进侧栏里。
   */
  private verticalOrderIds() {
    return this.verticalOrder.filter(zoneId =>
      this.zones.some(zone => zone.id === zoneId),
    );
  }

  private verticalNeighbor(zoneId: string, direction: FocusDirection) {
    const order = this.verticalOrderIds();
    const currentIndex = order.indexOf(zoneId);
    if (currentIndex === -1) {
      return undefined;
    }

    const neighborId = order[currentIndex + (direction === "down" ? 1 : -1)];
    if (!neighborId) {
      return undefined;
    }

    const neighbor = this.findZone(neighborId);
    return neighbor && countItems(neighbor) > 0 ? neighbor : undefined;
  }
}
