import type { vo } from "../../../src/bindings/models";

import { useCallback, useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";

import { enums } from "../../../src/bindings/models";
import { getMetadataSourceIcon } from "../../utils/metadataSources";
import {
  disconnectNextMoeAuthorization,
  fetchNextMoeAuthStatus,
  mergeNextMoeAuthStatus,
  startNextMoeAuthorization,
} from "../../utils/nextMoeAuth";
import { ConfirmModal } from "../modal/ConfirmModal";
import { BetterButton } from "../ui/better/BetterButton";

interface NextMoeAccountSettingsProps {
  isContentVisible: boolean;
  isExpanded: boolean;
  onConfigRefresh: () => Promise<void>;
  onExpand: () => void;
}

export function NextMoeAccountSettings({
  isContentVisible,
  isExpanded,
  onConfigRefresh,
  onExpand,
}: NextMoeAccountSettingsProps) {
  const { t } = useTranslation();
  const [snapshot, setSnapshot] = useState<vo.NextMoeAuthStatus | null>(null);
  const [isStatusLoading, setIsStatusLoading] = useState(false);
  const [isAuthorizing, setIsAuthorizing] = useState(false);
  const [isDisconnecting, setIsDisconnecting] = useState(false);
  const [showDisconnectConfirm, setShowDisconnectConfirm] = useState(false);

  const logoUrl = getMetadataSourceIcon(enums.SourceType.NextMoe) ?? "";
  const auth = mergeNextMoeAuthStatus(null, snapshot);
  const isAuthorized = auth.state === "authorized";
  const accountLabel = auth.accountLabel;

  const refreshStatus = useCallback(async () => {
    setIsStatusLoading(true);
    try {
      setSnapshot(await fetchNextMoeAuthStatus());
    }
    catch (error) {
      console.error("Failed to fetch NextMoe auth status:", error);
      setSnapshot(null);
    }
    finally {
      setIsStatusLoading(false);
    }
  }, []);

  useEffect(() => {
    void refreshStatus();
  }, [refreshStatus]);

  const handleAuthorize = async () => {
    setIsAuthorizing(true);
    try {
      await startNextMoeAuthorization();
      await onConfigRefresh();
      await refreshStatus();
      toast.success(t("settings.basic.nextMoeAuthSuccess"));
    }
    catch (error) {
      toast.error(
        t("settings.basic.nextMoeAuthActionFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
      await refreshStatus();
    }
    finally {
      setIsAuthorizing(false);
    }
  };

  const handleDisconnect = async () => {
    setIsDisconnecting(true);
    try {
      await disconnectNextMoeAuthorization();
      await onConfigRefresh();
      await refreshStatus();
      toast.success(t("settings.basic.nextMoeDisconnectSuccess"));
    }
    catch (error) {
      toast.error(
        t("settings.basic.nextMoeAuthActionFailed", {
          error: error instanceof Error ? error.message : String(error),
        }),
      );
    }
    finally {
      setIsDisconnecting(false);
    }
  };

  return (
    <>
      <div
        className={`glass-panel relative isolate min-h-[132px] min-w-0 overflow-hidden rounded-2xl border transition-colors duration-200 sm:h-[190px] lg:h-[160px] ${
          isExpanded
            ? "border-brand-300/90 bg-brand-50/70 shadow-sm dark:border-brand-600/90 dark:bg-brand-900/35"
            : "border-brand-200/80 bg-white/55 hover:border-brand-300/80 dark:border-brand-700/80 dark:bg-brand-900/25 dark:hover:border-brand-600/80"
        }`}
      >
        <img
          src={logoUrl}
          alt=""
          aria-hidden="true"
          className="pointer-events-none absolute bottom-4 right-4 z-0 h-auto w-48 object-contain opacity-30 dark:opacity-25"
        />

        {isExpanded ? (
          <div
            className={`account-choice-content-transition relative z-10 flex h-full flex-col gap-3 overflow-y-auto p-3 motion-reduce:transition-none ${
              isContentVisible ? "opacity-100" : "pointer-events-none opacity-0"
            }`}
          >
            <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div className="min-w-0 flex flex-1 items-center gap-3">
                <div className="h-12 w-12 shrink-0 overflow-hidden rounded-2xl border border-brand-200/80 dark:border-brand-700/80">
                  <img
                    src={logoUrl}
                    alt=""
                    width={48}
                    height={48}
                    className="h-full w-full object-cover"
                  />
                </div>

                <div className="min-w-0 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <div className="truncate text-sm font-semibold text-brand-800 dark:text-brand-100">
                      {t("gameEdit.sourceNextMoe")}
                    </div>
                    {isStatusLoading ? (
                      <span
                        aria-hidden="true"
                        className="i-mdi-loading animate-spin text-brand-400"
                      />
                    ) : null}
                  </div>

                  {isAuthorized ? (
                    <p className="truncate text-xs text-brand-500 dark:text-brand-400">
                      {accountLabel || t("settings.basic.nextMoeAuthorized")}
                    </p>
                  ) : (
                    <p className="text-xs text-brand-500 dark:text-brand-400">
                      {t("settings.basic.nextMoeAuthHint")}
                    </p>
                  )}
                </div>
              </div>

              <div className="flex self-end gap-2 lg:self-auto">
                {isAuthorized ? (
                  <BetterButton
                    variant="danger"
                    size="sm"
                    icon="i-mdi-link-off"
                    isLoading={isDisconnecting}
                    className="!rounded-full"
                    aria-label={t("settings.basic.nextMoeDisconnect")}
                    onClick={() => setShowDisconnectConfirm(true)}
                  />
                ) : (
                  <BetterButton
                    variant="primary"
                    icon="i-mdi-account-key-outline"
                    isLoading={isAuthorizing}
                    onClick={handleAuthorize}
                  >
                    {t("settings.basic.nextMoeAuthorize")}
                  </BetterButton>
                )}
              </div>
            </div>
          </div>
        ) : (
          <button
            type="button"
            className={`account-choice-content-transition relative z-10 flex h-full min-h-[132px] w-full items-center p-3 text-left motion-reduce:transition-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-brand-400 ${
              isContentVisible ? "opacity-100" : "pointer-events-none opacity-0"
            }`}
            aria-expanded="false"
            onClick={onExpand}
          >
            <span className="flex min-w-0 flex-1 items-center gap-3">
              <span className="h-12 w-12 shrink-0 overflow-hidden rounded-2xl border border-brand-200/80 dark:border-brand-700/80">
                <img
                  src={logoUrl}
                  alt=""
                  width={48}
                  height={48}
                  className="h-full w-full object-cover"
                />
              </span>

              <span className="min-w-0 flex-1 space-y-1">
                <span className="flex min-w-0 items-center gap-2">
                  <span className="truncate text-sm font-semibold text-brand-800 dark:text-brand-100">
                    {t("gameEdit.sourceNextMoe")}
                  </span>
                  {isStatusLoading ? (
                    <span
                      aria-hidden="true"
                      className="i-mdi-loading shrink-0 animate-spin text-brand-400"
                    />
                  ) : null}
                </span>

                <span className="block text-xs text-brand-500 dark:text-brand-400">
                  {isAuthorized
                    ? accountLabel || t("settings.basic.nextMoeAuthorized")
                    : t("settings.basic.nextMoeAuthUnauthorized")}
                </span>
              </span>

              <span
                aria-hidden="true"
                className="i-mdi-chevron-right shrink-0 text-lg text-brand-400"
              />
            </span>
          </button>
        )}
      </div>

      <ConfirmModal
        isOpen={showDisconnectConfirm}
        title={t("settings.basic.nextMoeDisconnectConfirmTitle")}
        message={t("settings.basic.nextMoeDisconnectConfirmMsg")}
        confirmText={t("settings.basic.nextMoeDisconnect")}
        type="danger"
        onClose={() => setShowDisconnectConfirm(false)}
        onConfirm={() => {
          void handleDisconnect();
        }}
      />
    </>
  );
}
