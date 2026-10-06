import type { Dispatch, SetStateAction } from "react";

import { Window } from "@wailsio/runtime";
import { useEffect, useRef } from "react";
import { toast } from "react-hot-toast";
import { useTranslation } from "react-i18next";

import type { appconf, vo } from "../../src/bindings/models";
import type { FetchHomeDataOptions, GameRuntimeChangedEvent } from "../store";

import { ShouldShowMainWindowOnReady } from "../../bindings/yukihub/internal/service/configservice";
import { GetPendingInstall } from "../../bindings/yukihub/internal/service/downloadservice";
import { onWailsEvent } from "../../src/bindings/runtime";
import { invalidateAllGameLists } from "../cache/gameCache";
import { useAppStore } from "../store";

export type QuitSyncRequest = {
  reason: string;
  requestedAt: number;
};

type BangumiStatusPushFailureEvent = {
  game_id?: string;
  game_name?: string;
  subject_id?: string;
  local_status?: string;
  error?: string;
};

type HikarinagiStatusPushFailureEvent = {
  game_id?: string;
  game_name?: string;
  work_id?: string;
  local_status?: string;
  error?: string;
};

type ScheduledDBBackupEvent = {
  status?: "started" | "completed" | "failed";
  error?: string;
};

type UseAppRuntimeEffectsOptions = {
  config: appconf.AppConfig | null;
  refreshConfig: () => Promise<void>;
  refreshHomeData: (options?: FetchHomeDataOptions) => Promise<void>;
  setInstallRequest: Dispatch<SetStateAction<vo.InstallRequest | null>>;
  setQuitSyncRequest: Dispatch<SetStateAction<QuitSyncRequest | null>>;
};

const WAILS_RESIZE_BORDER_THICKNESS = 5;

export function useAppRuntimeEffects({
  config,
  refreshConfig,
  refreshHomeData,
  setInstallRequest,
  setQuitSyncRequest,
}: UseAppRuntimeEffectsOptions) {
  const { t } = useTranslation();
  const applyGameRuntimeEvent = useAppStore(
    state => state.applyGameRuntimeEvent,
  );
  const initialWindowReadyCheckedRef = useRef(false);
  const skipNextLaunchHomeRefreshRef = useRef(false);

  useEffect(() => {
    if (window.wails?.flags) {
      window.wails.flags.borderThickness = WAILS_RESIZE_BORDER_THICKNESS;
    }
  }, []);

  useEffect(() => {
    if (!config || initialWindowReadyCheckedRef.current) {
      return;
    }

    initialWindowReadyCheckedRef.current = true;
    let cancelled = false;

    void ShouldShowMainWindowOnReady()
      .then((shouldShow) => {
        if (cancelled || !shouldShow) {
          return;
        }
        void Window.Show();
      })
      .catch((error) => {
        console.error("Failed to resolve initial window visibility:", error);
        if (!cancelled) {
          void Window.Show();
        }
      });

    GetPendingInstall().then((req) => {
      if (cancelled || !req) {
        return;
      }

      setInstallRequest(req);
      void Window.Show();
    });

    return () => {
      cancelled = true;
    };
  }, [config, setInstallRequest]);

  useEffect(() => {
    const unsubscribe = onWailsEvent(
      "install:pending",
      (req: vo.InstallRequest) => {
        setInstallRequest(req);
        void Window.Show();
      },
    );

    return unsubscribe;
  }, [setInstallRequest]);

  useEffect(() => {
    const unsubscribe = onWailsEvent(
      "app:quit-sync-requested",
      (payload?: { reason?: string }) => {
        setQuitSyncRequest({
          reason: payload?.reason ?? "unknown",
          requestedAt: Date.now(),
        });
      },
    );

    return unsubscribe;
  }, [setQuitSyncRequest]);

  useEffect(() => {
    const unsubscribe = onWailsEvent(
      "protocol-launch:error",
      (payload?: {
        message?: string;
        detail?: string;
        game_id?: string;
        kind?: string;
        config_key?: string;
      }) => {
        const message
          = payload?.message?.trim() || t("protocolLaunch.launchFailed");
        const detail = payload?.detail?.trim();
        void Window.Show();
        toast.error(detail ? `${message}\n${detail}` : message, {
          id: "protocol-launch-error",
        });
      },
    );

    return unsubscribe;
  }, [t]);

  useEffect(() => {
    const unsubscribe = onWailsEvent("bangumi:auth-status-changed", () => {
      void refreshConfig();
    });

    return unsubscribe;
  }, [refreshConfig]);

  useEffect(() => {
    const unsubscribe = onWailsEvent("hikarinagi:auth-status-changed", () => {
      void refreshConfig();
    });

    return unsubscribe;
  }, [refreshConfig]);

  useEffect(() => {
    const unsubscribe = onWailsEvent(
      "bangumi:status-push-failed",
      (payload?: BangumiStatusPushFailureEvent) => {
        const gameName
          = payload?.game_name?.trim()
            || t("settings.basic.bangumiStatusPushFailedUnknownGame");
        const error
          = payload?.error?.trim()
            || t("settings.basic.bangumiStatusPushFailedUnknownReason");
        toast.error(
          t("settings.basic.bangumiStatusPushFailed", {
            game: gameName,
            error,
          }),
          {
            id: `bangumi-status-push-failed-${payload?.game_id || "unknown"}`,
          },
        );
      },
    );

    return unsubscribe;
  }, [t]);

  useEffect(() => {
    const unsubscribe = onWailsEvent(
      "hikarinagi:status-push-failed",
      (payload?: HikarinagiStatusPushFailureEvent) => {
        const gameName
          = payload?.game_name?.trim()
            || t("settings.basic.hikarinagiStatusPushFailedUnknownGame");
        const error
          = payload?.error?.trim()
            || t("settings.basic.hikarinagiStatusPushFailedUnknownReason");
        toast.error(
          t("settings.basic.hikarinagiStatusPushFailed", {
            game: gameName,
            error,
          }),
          {
            id: `hikarinagi-status-push-failed-${payload?.game_id || "unknown"}`,
          },
        );
      },
    );

    return unsubscribe;
  }, [t]);

  useEffect(() => {
    const unsubscribe = onWailsEvent("app:main-window-shown", () => {
      void refreshHomeData();
    });

    return unsubscribe;
  }, [refreshHomeData]);

  useEffect(() => {
    const unsubscribe = onWailsEvent(
      "database-backup:scheduled",
      (event?: ScheduledDBBackupEvent) => {
        const toastID = "scheduled-database-backup";
        if (event?.status === "started") {
          toast.loading(t("settings.autoBackup.scheduledDbBackupStarted"), {
            duration: Infinity,
            id: toastID,
          });
          return;
        }

        if (event?.status === "completed") {
          toast.success(t("settings.autoBackup.scheduledDbBackupCompleted"), {
            id: toastID,
          });
          return;
        }

        if (event?.status === "failed") {
          toast.error(
            t("settings.autoBackup.scheduledDbBackupFailed", {
              error: event.error || t("settings.autoBackup.unknownError"),
            }),
            { id: toastID },
          );
        }
      },
    );

    return unsubscribe;
  }, [t]);

  useEffect(() => {
    // 同步（账号云同步 / WebDAV 自持同步）把数据写回本地库后，必须让游戏库与首页
    // 立即看到新数据 —— 否则用户以为「同步了没效果」，其实只是界面没刷新。
    const unsubscribe = onWailsEvent("yukihub-sync:applied", () => {
      invalidateAllGameLists();
      void refreshHomeData({ showLoading: false, syncRuntime: false });
      void refreshConfig();
    });

    return unsubscribe;
  }, [refreshConfig, refreshHomeData]);

  useEffect(() => {
    const unsubscribe = onWailsEvent("home:refresh-requested", () => {
      if (skipNextLaunchHomeRefreshRef.current) {
        skipNextLaunchHomeRefreshRef.current = false;
        return;
      }

      void refreshHomeData({ showLoading: false, syncRuntime: false });
    });

    return unsubscribe;
  }, [refreshHomeData]);

  useEffect(() => {
    const unsubscribe = onWailsEvent(
      "game-runtime:changed",
      (event?: GameRuntimeChangedEvent) => {
        if (!event) {
          return;
        }

        applyGameRuntimeEvent(event);

        if (event.state === "launching" && event.reason === "launched") {
          skipNextLaunchHomeRefreshRef.current = true;
          return;
        }

        if (event.state === "playing" || event.state === "ending") {
          return;
        }

        void refreshHomeData({ showLoading: false, syncRuntime: false });
      },
    );

    return unsubscribe;
  }, [applyGameRuntimeEvent, refreshHomeData]);
}
