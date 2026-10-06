import type { vo } from "../../../src/bindings/models";
import { useCallback, useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";

import {
  GetSelfSyncConfig,
  SaveSelfSyncConfig,
  SyncSelfHostedNow,
  TestSelfSyncConnection,
} from "../../../bindings/yukihub/internal/service/selfsyncservice";
import { invalidateAllGameLists } from "../../cache/gameCache";
import { formatLocalDateTime } from "../../utils/time";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterInput } from "../ui/better/BetterInput";
import { ModalPortal } from "../ui/ModalPortal";
import { SettingSwitchRow } from "../ui/SettingSwitchRow";

interface ConflictState {
  localBytes: number;
  remoteBytes: number;
}

/**
 * 「自持同步（WebDAV）」—— 把与手机版完全相同的游戏库快照同步到用户自己的 WebDAV 网盘。
 *
 * 对齐手机版的 `WebDavSettingsDialog`：服务器 / 用户名 / 密码三个输入框、自动同步开关、
 * 测试连接、保存配置、立即同步、上次同步时间，以及冲突时的四选一（智能合并 / 用云端 /
 * 用本地 / 取消）。云端文件名也一致（`YukiHub/YukiHub_sync.json`，gzip），
 * 所以两端可以互相同步同一份数据。
 *
 * 与「账号云同步」（yukihub.zh.kg）是两条并列通道，快照格式完全相同，配一个就够。
 */
export function SelfSyncPanel() {
  const { t } = useTranslation();

  const [config, setConfig] = useState<vo.SelfSyncConfig | null>(null);
  const [server, setServer] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [autoSync, setAutoSync] = useState(false);
  const [status, setStatus] = useState<{
    text: string;
    tone: "ok" | "error" | "info";
  } | null>(null);
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [conflict, setConflict] = useState<ConflictState | null>(null);

  const loadConfig = useCallback(async () => {
    try {
      const loaded = await GetSelfSyncConfig();
      setConfig(loaded);
      setServer(loaded.server_url ?? "");
      setUsername(loaded.username ?? "");
      setPassword(loaded.password ?? "");
      setAutoSync(Boolean(loaded.auto_sync));
    }
    catch (error) {
      console.error("Failed to load self sync config:", error);
    }
  }, []);

  useEffect(() => {
    void loadConfig();
  }, [loadConfig]);

  const reportFailure = (key: string, error: unknown) => {
    const message = error instanceof Error ? error.message : String(error);
    setStatus({ text: t(key, { error: message }), tone: "error" });
  };

  const handleTest = async () => {
    if (testing) {
      return;
    }
    setTesting(true);
    try {
      await TestSelfSyncConnection({
        server_url: server,
        username,
        password,
        auto_sync: autoSync,
        configured: true,
      } as vo.SelfSyncConfig);
      setStatus({ text: t("settings.selfSync.testSuccess"), tone: "ok" });
    }
    catch (error) {
      reportFailure("settings.selfSync.testFailed", error);
    }
    finally {
      setTesting(false);
    }
  };

  const handleSave = async () => {
    if (saving) {
      return;
    }
    setSaving(true);
    try {
      await SaveSelfSyncConfig({
        server_url: server,
        username,
        password,
        auto_sync: autoSync,
        configured: true,
      } as vo.SelfSyncConfig);
      await loadConfig();
      setStatus({ text: t("settings.selfSync.saved"), tone: "ok" });
      toast.success(t("settings.selfSync.saved"));
    }
    catch (error) {
      reportFailure("settings.selfSync.saveFailed", error);
    }
    finally {
      setSaving(false);
    }
  };

  const describeResult = (result: vo.SelfSyncResult): string => {
    switch (result.action) {
      case "uploaded":
        return t("settings.selfSync.resultUploaded");
      case "downloaded":
        return t("settings.selfSync.resultDownloaded");
      case "merged":
        return t("settings.selfSync.resultMerged");
      case "cancelled":
        return t("settings.selfSync.resultCancelled");
      default:
        return t("settings.selfSync.resultNoop");
    }
  };

  const runSync = async (resolution: string) => {
    setSyncing(true);
    try {
      const result = await SyncSelfHostedNow(resolution);
      if (result.action === "conflict") {
        setConflict({
          localBytes: result.local_bytes,
          remoteBytes: result.remote_bytes,
        });
        return;
      }
      if (result.action === "downloaded" || result.action === "merged") {
        // 本地库被改了 —— 立即失效缓存，否则界面还停在旧数据上。
        invalidateAllGameLists();
      }
      setStatus({ text: describeResult(result), tone: "ok" });
      await loadConfig();
    }
    catch (error) {
      reportFailure("settings.selfSync.syncFailed", error);
    }
    finally {
      setSyncing(false);
    }
  };

  const resolveConflict = async (resolution: string) => {
    setConflict(null);
    await runSync(resolution);
  };

  const statusClass
    = status?.tone === "error"
      ? "text-error-600 dark:text-error-400"
      : status?.tone === "ok"
        ? "text-success-600 dark:text-success-400"
        : "text-brand-500 dark:text-brand-400";

  return (
    <>
      <div className="space-y-4">
        <div className="space-y-2">
          <label
            className="block text-sm font-medium text-brand-700 dark:text-brand-300"
            htmlFor="self_sync_server"
          >
            {t("settings.selfSync.server")}
          </label>
          <BetterInput
            id="self_sync_server"
            value={server}
            placeholder={t("settings.selfSync.serverPlaceholder")}
            onChange={event => setServer(event.target.value)}
          />
          <p className="text-xs text-brand-500 dark:text-brand-400">
            {t("settings.selfSync.serverHint")}
          </p>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <label
              className="block text-sm font-medium text-brand-700 dark:text-brand-300"
              htmlFor="self_sync_username"
            >
              {t("settings.selfSync.username")}
            </label>
            <BetterInput
              id="self_sync_username"
              value={username}
              autoComplete="off"
              onChange={event => setUsername(event.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label
              className="block text-sm font-medium text-brand-700 dark:text-brand-300"
              htmlFor="self_sync_password"
            >
              {t("settings.selfSync.password")}
            </label>
            <BetterInput
              id="self_sync_password"
              type="password"
              value={password}
              autoComplete="new-password"
              onChange={event => setPassword(event.target.value)}
            />
          </div>
        </div>

        <SettingSwitchRow
          id="self_sync_auto"
          label={t("settings.selfSync.autoSync")}
          hint={t("settings.selfSync.autoSyncHint")}
          checked={autoSync}
          onCheckedChange={setAutoSync}
        />

        <div className="flex flex-wrap items-center gap-3 border-t border-brand-200/70 pt-4 dark:border-brand-700/60">
          <BetterButton
            type="button"
            variant="secondary"
            icon="i-mdi-lan-connect"
            isLoading={testing}
            onClick={() => void handleTest()}
          >
            {testing
              ? t("settings.selfSync.testing")
              : t("settings.selfSync.testConnection")}
          </BetterButton>
          <BetterButton
            type="button"
            variant="secondary"
            icon="i-mdi-content-save-outline"
            isLoading={saving}
            onClick={() => void handleSave()}
          >
            {t("settings.selfSync.save")}
          </BetterButton>
          <BetterButton
            type="button"
            variant="primary"
            icon="i-mdi-sync"
            isLoading={syncing}
            disabled={!config?.configured}
            onClick={() => void runSync("")}
          >
            {syncing
              ? t("settings.selfSync.syncing")
              : t("settings.selfSync.syncNow")}
          </BetterButton>
        </div>

        <div className="space-y-1">
          <p className="text-xs text-brand-500 dark:text-brand-400">
            {config?.last_sync_at
              ? t("settings.selfSync.lastSync", {
                  time: formatLocalDateTime(config.last_sync_at),
                })
              : t("settings.selfSync.lastSyncNever")}
          </p>
          {status && <p className={`text-xs ${statusClass}`}>{status.text}</p>}
        </div>
      </div>

      {conflict && (
        <ModalPortal>
          <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
            <div className="w-full max-w-lg rounded-xl border border-brand-200 bg-white p-6 shadow-xl dark:border-brand-700 dark:bg-brand-800">
              <div className="flex items-start gap-4">
                <div className="rounded-full bg-warning-100 p-2 text-warning-600 dark:bg-warning-900/30 dark:text-warning-400">
                  <div className="i-mdi-alert-circle text-2xl" />
                </div>
                <div className="flex-1 space-y-2">
                  <h3 className="text-xl font-bold text-brand-900 dark:text-white">
                    {t("settings.selfSync.conflictTitle")}
                  </h3>
                  <p className="text-sm leading-relaxed text-brand-600 dark:text-brand-400">
                    {t("settings.selfSync.conflictMessage", {
                      local: Math.round(conflict.localBytes / 1024),
                      remote: Math.round(conflict.remoteBytes / 1024),
                    })}
                  </p>
                </div>
              </div>

              <div className="mt-6 flex flex-wrap justify-end gap-3">
                <BetterButton
                  type="button"
                  variant="secondary"
                  onClick={() => void resolveConflict("cancel")}
                >
                  {t("common.cancel")}
                </BetterButton>
                <BetterButton
                  type="button"
                  variant="secondary"
                  onClick={() => void resolveConflict("local")}
                >
                  {t("settings.selfSync.useLocal")}
                </BetterButton>
                <BetterButton
                  type="button"
                  variant="secondary"
                  onClick={() => void resolveConflict("remote")}
                >
                  {t("settings.selfSync.useRemote")}
                </BetterButton>
                <BetterButton
                  type="button"
                  variant="primary"
                  onClick={() => void resolveConflict("merge")}
                >
                  {t("settings.selfSync.merge")}
                </BetterButton>
              </div>
            </div>
          </div>
        </ModalPortal>
      )}
    </>
  );
}
