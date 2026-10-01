import type { ReactNode } from "react";

interface SettingsSubSectionProps {
  title: string;
  hint?: string;
  icon?: string;
  children: ReactNode;
}

/**
 * 设置分区内部的子分组标题。
 *
 * 用途：把原本各自占一个顶层分区的面板收进同一个分区里（例如「同步与备份」
 * 下面再分「云服务 / 自动备份 / 数据库备份」）。顶层分区从 4 个备份变成 1 个，
 * 用户不会再纠结「我到底该点哪一个来备份」。
 */
export function SettingsSubSection({
  title,
  hint,
  icon,
  children,
}: SettingsSubSectionProps) {
  return (
    <div className="space-y-3">
      <div className="border-t border-brand-200/70 pt-4 first:border-t-0 first:pt-0 dark:border-brand-700/60">
        <h3 className="flex items-center gap-2 text-sm font-bold text-brand-800 dark:text-white">
          {icon && (
            <span
              className={`${icon} text-base text-primary-500 dark:text-primary-300`}
              aria-hidden="true"
            />
          )}
          {title}
        </h3>
        {hint && (
          <p className="mt-0.5 text-xs text-brand-500 dark:text-brand-400">
            {hint}
          </p>
        )}
      </div>
      <div className="space-y-4">{children}</div>
    </div>
  );
}
