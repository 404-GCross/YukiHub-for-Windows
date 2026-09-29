import { useTranslation } from "react-i18next";
import { formatDurationCompact } from "../../utils/time";

interface HomeTodayStatsCardProps {
  completedGames: number | null;
  libraryGames: number | null;
  todayPlayTimeSec: number;
  weeklyPlayTimeSec: number;
}

export function HomeTodayStatsCard({
  completedGames,
  libraryGames,
  todayPlayTimeSec,
  weeklyPlayTimeSec,
}: HomeTodayStatsCardProps) {
  const { t } = useTranslation();

  const items = [
    {
      icon: "i-mdi-timer-outline",
      label: t("home.todayPlayTime"),
      value: formatDurationCompact(todayPlayTimeSec, t),
    },
    {
      icon: "i-mdi-calendar-week-outline",
      label: t("home.weeklyPlayTime"),
      value: formatDurationCompact(weeklyPlayTimeSec, t),
    },
    {
      icon: "i-mdi-gamepad-square-outline",
      label: t("home.libraryGames"),
      value: libraryGames === null ? "--" : String(libraryGames),
    },
    {
      icon: "i-mdi-trophy-outline",
      label: t("home.completedGames"),
      value: completedGames === null ? "--" : String(completedGames),
    },
  ];

  return (
    <section className="yh-glass flex flex-col gap-3 p-4">
      <div className="flex items-center gap-2">
        <span className="i-mdi-chart-line text-base text-brand-700 dark:text-white/85" />
        <h3 className="text-sm font-bold text-brand-900 dark:text-white">
          {t("home.todayTitle")}
        </h3>
      </div>

      <div className="grid grid-cols-2 gap-2">
        {items.map(item => (
          <div
            key={item.label}
            className="yh-glass-chip flex min-w-0 flex-col gap-1 px-3 py-2.5"
          >
            <span
              className={`${item.icon} text-lg text-brand-600 dark:text-primary-300`}
              aria-hidden="true"
            />
            <span className="truncate text-lg font-bold leading-tight text-brand-900 dark:text-white">
              {item.value}
            </span>
            <span className="truncate text-[11px] text-brand-600 dark:text-white/70">
              {item.label}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}
