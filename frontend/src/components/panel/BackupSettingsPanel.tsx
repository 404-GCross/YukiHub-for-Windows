import type { appconf, vo } from "../../../src/bindings/models";
import { useEffect, useState } from "react";
import toast from "react-hot-toast";
import { useTranslation } from "react-i18next";
import {
  CreateDBBackup,
  DeleteDBBackup,
  GetDBBackups,
  ScheduleDBRestore,
} from "../../../bindings/yukihub/internal/service/backupservice";
import { SafeQuit } from "../../../bindings/yukihub/internal/service/configservice";
import { formatFileSize } from "../../utils/size";
import { formatLocalDateTime } from "../../utils/time";
import { ConfirmModal } from "../modal/ConfirmModal";
import { BetterButton } from "../ui/better/BetterButton";
import { BetterNumberInput } from "../ui/better/BetterNumberInput";
import { SettingSwitchRow } from "../ui/SettingSwitchRow";

interface BackupSettingsPanelProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

/**
 * 「备份」——整个备份功能就这一个面板。
 *
 * 刻意做得很小：一个自动备份开关、一个保留份数、一个立即备份按钮，外加本机
 * 备份列表（可恢复/删除）。历史上这里曾经是四个独立分区（云配置 / 同步与备份 /
 * 数据库备份 / 全量数据备份）叠在一起，用户根本分不清该点哪个。
 *
 * 云端备份已移除：等账号系统接进来之后单独做，不再塞进本地设置里。
 */
export function BackupSettingsPanel({
  formData,
  onChange,
}: BackupSettingsPanelProps) {
  const { t } = useTranslation();
  const [dbBackups, setDbBackups] = useState<vo.DBBackupStatus | null>(null);
  const [isBackingUp, setIsBackingUp] = useState(false);
  const [restoringBackup, setRestoringBackup] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [confirmConfig, setConfirmConfig] = useState<{
    isOpen: boolean;
    title: string;
    message: string;
    type: "danger" | "info";
    onConfirm: () => void;
  }>({
    isOpen: false,
    title: "",
    message: "",
    type: "info",
    onConfirm: () => {},
  });

  // 自动备份是一个总开关：数据库与游戏存档一起开/关。
  // 拆成两个开关只是历史包袱，用户要的是「开还是不开」。
  const autoBackupEnabled = Boolean(
    formData.auto_backup_db || formData.auto_backup_game_save,
  );
  const retention = formData.local_db_backup_retention || 5;

  const loadBackups = async () => {
    setLoading(true);
    try {
      setDbBackups(await GetDBBackups());
    }
    catch (error) {
      console.error("Failed to load DB backups:", error);
    }
    finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadBackups();
  }, []);

  const handleAutoBackupChange = (checked: boolean) => {
    onChange({
      ...formData,
      auto_backup_db: checked,
      auto_backup_game_save: checked,
    } as appconf.AppConfig);
  };

  const handleRetentionChange = (value: number) => {
    const next = Math.max(1, value);
    onChange({
      ...formData,
      // 数据库与游戏存档用同一个份数，避免又多一个「存档保留几份」的选项
      local_backup_retention: next,
      local_db_backup_retention: next,
    } as appconf.AppConfig);
  };

  const handleCreateBackup = async () => {
    if (isBackingUp) {
      return;
    }
    setIsBackingUp(true);
    try {
      await CreateDBBackup();
      await loadBackups();
      toast.success(t("settings.backup.toast.created"));
    }
    catch (error: any) {
      toast.error(t("settings.backup.toast.createFailed", { error }));
    }
    finally {
      setIsBackingUp(false);
    }
  };

  const handleRestore = (backupPath: string) => {
    setConfirmConfig({
      isOpen: true,
      title: t("settings.backup.modal.restoreTitle"),
      message: t("settings.backup.modal.restoreMsg"),
      type: "info",
      onConfirm: async () => {
        setRestoringBackup(backupPath);
        try {
          await ScheduleDBRestore(backupPath);
          toast.success(t("settings.backup.toast.restoreScheduled"));
          // 恢复在下次启动时执行，所以这里要正常退出一次
          setTimeout(() => SafeQuit(), 1500);
        }
        catch (error: any) {
          toast.error(t("settings.backup.toast.restoreFailed", { error }));
          setRestoringBackup(null);
        }
      },
    });
  };

  const handleDelete = (backupPath: string) => {
    setConfirmConfig({
      isOpen: true,
      title: t("settings.backup.modal.deleteTitle"),
      message: t("settings.backup.modal.deleteMsg"),
      type: "danger",
      onConfirm: async () => {
        try {
          await DeleteDBBackup(backupPath);
          await loadBackups();
          toast.success(t("settings.backup.toast.deleted"));
        }
        catch (error: any) {
          toast.error(t("settings.backup.toast.deleteFailed", { error }));
        }
      },
    });
  };

  const backups = dbBackups?.backups ?? [];

  return (
    <>
      <div className="space-y-4">
        <SettingSwitchRow
          id="auto_backup_db"
          label={t("settings.backup.autoBackup")}
          hint={t("settings.backup.autoBackupHint")}
          checked={autoBackupEnabled}
          onCheckedChange={handleAutoBackupChange}
        />

        <div className="space-y-2">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.backup.retention")}
          </label>
          <BetterNumberInput
            value={retention}
            min={1}
            max={50}
            unit={t("settings.backup.retentionUnit")}
            onValueChange={handleRetentionChange}
          />
          <p className="text-xs text-brand-500 dark:text-brand-400">
            {t("settings.backup.retentionHint")}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3 border-t border-brand-200/70 pt-4 dark:border-brand-700/60">
          <BetterButton
            type="button"
            variant="primary"
            icon="i-mdi-database-plus-outline"
            isLoading={isBackingUp}
            onClick={() => void handleCreateBackup()}
          >
            {isBackingUp
              ? t("settings.backup.backingUp")
              : t("settings.backup.backupNow")}
          </BetterButton>
          {dbBackups?.last_backup_time && (
            <span className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.backup.lastBackup")}
              {formatLocalDateTime(dbBackups.last_backup_time)}
            </span>
          )}
        </div>

        <div className="space-y-2">
          <div className="text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.backup.listTitle")}
          </div>
          {loading ? (
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("common.loading")}
            </p>
          ) : backups.length === 0 ? (
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.backup.empty")}
            </p>
          ) : (
            <ul className="flex flex-col gap-2">
              {backups.map(backup => (
                <li
                  key={backup.path}
                  className="flex items-center gap-3 rounded-lg border border-brand-200/80 bg-white/50 px-3 py-2 dark:border-brand-700/70 dark:bg-brand-900/25"
                >
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-xs font-medium text-brand-800 dark:text-white/90">
                      {formatLocalDateTime(backup.created_at)}
                    </div>
                    <div className="text-[11px] text-brand-500 dark:text-brand-400">
                      {formatFileSize(backup.size)}
                    </div>
                  </div>
                  <BetterButton
                    type="button"
                    size="sm"
                    variant="secondary"
                    isLoading={restoringBackup === backup.path}
                    onClick={() => handleRestore(backup.path)}
                  >
                    {t("settings.backup.restore")}
                  </BetterButton>
                  <BetterButton
                    type="button"
                    size="sm"
                    variant="ghost"
                    icon="i-mdi-delete-outline"
                    aria-label={t("settings.backup.delete")}
                    onClick={() => handleDelete(backup.path)}
                  />
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>

      <ConfirmModal
        isOpen={confirmConfig.isOpen}
        title={confirmConfig.title}
        message={confirmConfig.message}
        type={confirmConfig.type}
        onConfirm={confirmConfig.onConfirm}
        onClose={() => setConfirmConfig(prev => ({ ...prev, isOpen: false }))}
      />
    </>
  );
}
