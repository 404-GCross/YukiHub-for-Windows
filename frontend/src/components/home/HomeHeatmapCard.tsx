import { useTranslation } from "react-i18next";
import { PlayHeatmap } from "../chart/PlayHeatmap";

interface HomeHeatmapCardProps {
  cells: { date: string; duration: number }[];
  hasFailed: boolean;
  isLoading: boolean;
  onRetry: () => void;
  onViewStats: () => void;
}

export function HomeHeatmapCard({
  cells,
  hasFailed,
  isLoading,
  onRetry,
  onViewStats,
}: HomeHeatmapCardProps) {
  const { t } = useTranslation();

  return (
    <section className="yh-glass flex min-w-0 flex-1 flex-col gap-3 p-4">
      <div className="flex items-center gap-2">
        <span className="i-mdi-calendar-month-outline text-base text-brand-700 dark:text-white/85" />
        <h3 className="min-w-0 flex-1 truncate text-sm font-bold text-brand-900 dark:text-white">
          {t("stats.heatmap.title")}
        </h3>
        <button
          type="button"
          onClick={onViewStats}
          className="shrink-0 rounded-full px-2 py-1 text-xs font-medium text-brand-700 transition-colors hover:bg-white/50 dark:text-white/80 dark:hover:bg-white/10"
        >
          {t("home.viewAll")}
        </button>
      </div>

      <div className="flex min-h-0 flex-1 items-center justify-center overflow-x-auto">
        {hasFailed ? (
          <button
            type="button"
            onClick={onRetry}
            className="flex items-center gap-2 rounded-lg border border-white/50 bg-white/40 px-4 py-2 text-sm font-medium text-brand-700 transition-colors hover:bg-white/65 dark:border-white/12 dark:bg-white/8 dark:text-white/85 dark:hover:bg-white/16"
          >
            <span className="i-mdi-refresh text-base" />
            {t("stats.toast.loadStatsFailed")}
          </button>
        ) : isLoading ? (
          <div className="flex items-center gap-2 text-sm text-brand-600 dark:text-white/70">
            <span className="i-mdi-loading animate-spin text-lg" />
            {t("common.loading")}
          </div>
        ) : (
          <PlayHeatmap cells={cells} />
        )}
      </div>
    </section>
  );
}
