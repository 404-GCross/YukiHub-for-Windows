import { useEffect, useState } from "react";

import type { vo } from "../bindings/models";

import { GetAccountStatus } from "../../bindings/yukihub/internal/service/accountservice";
import { onWailsEvent } from "../bindings/runtime";

/**
 * 订阅 YukiHub 账号状态。
 *
 * 后端在登录态变化 / 同步完成后会推 `yukihub-account:status-changed` 事件，
 * 首页用户区、聊天弹窗、设置页共用这一个 hook，保证各处显示一致。
 */
export function useAccountStatus() {
  const [status, setStatus] = useState<vo.AccountStatus | null>(null);

  useEffect(() => {
    let cancelled = false;
    GetAccountStatus()
      .then((value) => {
        if (!cancelled) {
          setStatus(value);
        }
      })
      .catch((error) => {
        console.error("Failed to load YukiHub account status:", error);
      });
    const off = onWailsEvent<vo.AccountStatus>(
      "yukihub-account:status-changed",
      next => setStatus(next),
    );
    return () => {
      cancelled = true;
      off();
    };
  }, []);

  return status;
}
