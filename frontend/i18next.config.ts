import { defineConfig } from "i18next-cli";

export default defineConfig({
  locales: ["en-US", "zh-CN", "zh-TW", "ja-JP"],
  extract: {
    input: "src/**/*.{js,jsx,ts,tsx}",
    output: "src/locales/{{language}}.json",
    defaultNS: false,
    removeUnusedKeys: true,
    disablePlurals: true,
    sort: false,
    preservePatterns: [
      "bigScreen.categoryAll",
      "bigScreen.categoryRecent",
      "common.allStatus",
      "common.name",
      "common.company",
      "common.lastPlayedAt",
      "common.rating",
      "common.releaseDate",
      "gameProgress.spoilerBoundaryOpts.*",
      "gameLaunch.steamLaunchOptionsPresets.*",
      "gameStats.periodStatsLabel.*",
      "settings.portableSetup.toast.*",
      "metadataUpdateFields.*",
      "settings.appearance.gameCardLayout_*",
      // 快捷键的文案键由 GLOBAL_SHORTCUTS 的 id 拼出来，提取器看不到
      "shortcuts.*",
    ],
  },
});
