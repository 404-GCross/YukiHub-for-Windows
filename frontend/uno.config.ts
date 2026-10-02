import mdiIcons from "@iconify-json/mdi/icons.json";
import { defineConfig, presetIcons, presetWind3 } from "unocss";

export default defineConfig({
  presets: [
    presetWind3({
      dark: "class",
    }),
    // pnpm 严格 node_modules 布局下，preset-icons 的 node loader 无法从
    // @iconify/utils 的位置解析到 @iconify-json/mdi，图标会全部空白。
    // 显式注册 mdi：值必须是函数，loader 会对函数求值后按集合查找。
    presetIcons({
      collections: {
        mdi: () => mdiIcons,
      },
    }),
  ],

  // 启动错误窗通过 main.tsx 动态加载。开发态子窗口首次打开时，
  // 预扫描该文件可确保专属工具类已经进入初始 UnoCSS 样式表。
  content: {
    filesystem: ["src/components/startup/StartupWindow.tsx"],

    pipeline: {
      // **必须把 `ts` 加进来。** UnoCSS 默认的 pipeline include 是
      // `/\.(vue|svelte|[jt]sx|vine.ts|mdx?|astro|elm|php|phtml|marko|html)($|\?)/`
      // ——只有 jsx/tsx，**没有纯 .ts**。写在 .ts 里的工具类会被静默丢掉：
      // 不报错、不告警，构建也成功，运行时表现为「样式没生效」。
      //
      // 踩过的坑：`src/consts/gameStatusBadge.ts` 的状态徽标配色
      // （bg-brand-900/55、bg-warning-600/80 …）一个都没进产物，游戏详情页
      // 选中的状态胶囊因此只剩 activeChipClass 里的 `text-white`，落在浅色
      // 玻璃背景上整个看不见（用户报「选中状态和背景融合了」）；
      // `src/utils/cloudSync.ts` 的 ring-* 同样缺失。
      //
      // 注意 `content.filesystem` 对 vite 插件是**空操作**（它只读 pipeline），
      // 想放开扫描范围只能改这里。下面前缀保留 UnoCSS 的默认值再加 ts。
      include: [
        /\.(vue|svelte|[jt]sx|ts|vine\.ts|mdx?|astro|elm|php|phtml|marko|html)($|\?)/,
      ],
    },
  },

  rules: [
    [
      "scrollbar-hide",
      {
        "scrollbar-width": "none",
        "-ms-overflow-style": "none",
      },
    ],
    // 弹窗内列表用的细滚动条（全局 9px 在小弹窗里太粗）
    [
      "scrollbar-thin",
      {
        "scrollbar-width": "thin",
        "scrollbar-color": "var(--scrollbar-thumb) var(--scrollbar-track)",
      },
    ],
    [
      "scrollbar-stable",
      {
        "scrollbar-gutter": "stable",
      },
    ],
    [
      "nsfw-cover-blur",
      {
        filter: "blur(18px)",
      },
    ],
    [
      "nsfw-cover-reveal",
      {
        filter: "none",
      },
    ],
    [
      "settings-section-render",
      {
        "content-visibility": "auto",
        "contain-intrinsic-size": "auto 56px",
      },
    ],
    [
      "settings-section-transition",
      {
        "transition-property": "grid-template-rows, opacity, visibility",
      },
    ],
    [
      "backdrop-filter-off",
      {
        "-webkit-backdrop-filter": "none",
        "backdrop-filter": "none",
      },
    ],
    [
      "paint-containment",
      {
        contain: "paint",
      },
    ],
    [
      "app-toast-stack-item",
      {
        "position": "absolute",
        "width": "100%",
        "transition-property": "transform, height, opacity",
        "transition-duration": "500ms, 180ms, 280ms",
        "transition-timing-function": "cubic-bezier(.22,1,.36,1)",
      },
    ],
    [
      "account-choice-transition",
      {
        "transition-property": "grid-template-columns",
        "transition-duration": "180ms",
        "transition-timing-function": "ease",
      },
    ],
    [
      "account-choice-content-transition",
      {
        "transition-property": "opacity",
        "transition-duration": "100ms",
        "transition-timing-function": "ease",
      },
    ],
    // 手机版 YukiHub 的签名渐变：主按钮（bg_button_primary.xml）
    [
      "yh-primary-gradient",
      {
        "background-image": "linear-gradient(90deg, #8AB4FF 0%, #B48AFF 100%)",
      },
    ],
    // 手机版首页横幅渐变（bg_home_gradient.xml）
    [
      "yh-hero-gradient",
      {
        "background-image":
          "linear-gradient(90deg, #27336F 0%, #554DA0 50%, #9A68C7 100%)",
      },
    ],
    // 首页横幅渐变的亮色版：同色系抬高明度，供亮色模式使用
    [
      "yh-hero-gradient-light",
      {
        "background-image":
          "linear-gradient(120deg, #E3E9FC 0%, #E9E0F9 50%, #F5E3F1 100%)",
      },
    ],
    // 手机版主按钮胶囊（bg_home_primary_pill.xml）：紫→淡紫，带白色描边
    [
      "yh-primary-pill",
      {
        "background-image": "linear-gradient(0deg, #795BE8 0%, #AA7AF3 100%)",
        "border": "1px solid rgba(255, 255, 255, 0.5)",
      },
    ],
    // 手机版侧栏/导航选中态（bg_home_nav_active.xml）
    [
      "yh-nav-active",
      {
        "background-image":
          "linear-gradient(0deg, rgba(122, 99, 224, 0.33) 0%, rgba(127, 106, 229, 0.4) 100%)",
        "border": "1px solid rgba(255, 255, 255, 0.28)",
      },
    ],
  ],

  // 自定义 variants - 支持 data-glass 属性
  variants: [
    // Headless UI transition states
    (matcher) => {
      const match = matcher.match(/^data-(closed|enter|leave):(.*)$/);
      if (!match)
        return matcher;

      const [, state, utility] = match;
      return {
        matcher: utility,
        selector: selector => `${selector}[data-${state}]`,
      };
    },
    // data-glass variant: 当元素或父元素有 data-glass="true" 时生效
    (matcher) => {
      if (!matcher.startsWith("data-glass:"))
        return matcher;

      return {
        matcher: matcher.slice(11), // 移除 'data-glass:' 前缀
        selector: s =>
          `[data-glass="true"] ${s}:not([data-glass="false"] *), ${s}[data-glass="true"]`,
      };
    },
    // Wails 在 macOS 与 Linux 使用原生 WebKit，禁用滚动玻璃表面的局部背景滤镜。
    (matcher) => {
      if (!matcher.startsWith("native-webkit:"))
        return matcher;

      return {
        matcher: matcher.slice(14),
        selector: s =>
          `[data-native-webkit="true"][data-glass="true"] ${s}:not([data-glass="false"] *)`,
      };
    },
  ],

  shortcuts: [
    // 玻璃态效果基础类
    {
      "app-toast-card":
        "relative flex w-full flex-col text-brand-900 dark:text-brand-50",
      "app-toast-stack-surface":
        "h-full overflow-hidden rounded-2xl border border-brand-200/90 bg-white text-brand-900 shadow-lg shadow-brand-900/8 dark:border-brand-700/70 dark:bg-brand-800 dark:text-brand-50 dark:shadow-black/25 data-glass:bg-white/85 data-glass:dark:bg-brand-800/85 data-glass:backdrop-blur-xl motion-reduce:transition-none",
      "startup-backdrop":
        "bg-brand-100 dark:bg-brand-900 ring-1 ring-inset ring-brand-300 dark:ring-brand-700",
      "glass": "backdrop-filter backdrop-blur-12 backdrop-saturate-180",
      "glass-border": "border border-white/18 dark:border-white/10",
      "glass-text":
        "drop-shadow-[0_1px_2px_rgba(0,0,0,0.3)] drop-shadow-[0_0_8px_rgba(0,0,0,0.2)]",
    },

    // YukiHub 首页玻璃卡（对齐手机版 bg_home_glass：白色半透明 + 白描边 + 16dp 圆角）
    {
      "yh-glass":
        "rounded-2xl border border-white/70 bg-white/55 backdrop-blur-xl dark:border-white/15 dark:bg-white/8",
      "yh-glass-inner":
        "rounded-xl border border-white/60 bg-white/45 dark:border-white/10 dark:bg-white/6",
      "yh-glass-chip":
        "rounded-lg border border-white/55 bg-white/45 dark:border-white/10 dark:bg-white/6",
      // 手机版胶囊标签（bg_chip：深蓝半透明 + 蓝描边 + 999 圆角）
      "yh-chip":
        "inline-flex items-center gap-1 rounded-full border border-primary-200/70 bg-white/80 px-2 py-0.5 text-[10px] font-bold leading-none text-brand-700 backdrop-blur-sm dark:border-primary-300/45 dark:bg-[#1D2B3E]/75 dark:text-white/90",
    },

    // 玻璃态层级系统（从不透明到透明）

    // 1. glass-aside - 侧边栏（最不透明，需要清晰的导航）
    [
      /^glass-aside$/,
      () =>
        "data-glass:bg-white/12 data-glass:dark:bg-black/15 data-glass:backdrop-blur-28 data-glass:backdrop-saturate-180 data-glass:border-r data-glass:border-white/20 data-glass:dark:border-white/12",
    ],

    // 2. glass-btn - 按钮（保持可见，需要明确的交互反馈）
    [
      /^glass-btn-(.*)$/,
      ([, color]) => {
        const colorMap: Record<string, string> = {
          neutral:
            "data-glass:bg-white/30 data-glass:dark:bg-black/30 data-glass:text-neutral-900 data-glass:dark:text-neutral-100 data-glass:hover:bg-white/45 data-glass:dark:hover:bg-black/45",
          error:
            "data-glass:bg-error-500/70 data-glass:text-white data-glass:hover:bg-error-500/85",
          success:
            "data-glass:bg-success-600/70 data-glass:text-white data-glass:hover:bg-success-600/85",
          primary:
            "data-glass:bg-neutral-600/70 data-glass:text-white data-glass:hover:bg-neutral-600/90 ",
        };
        return `data-glass:border data-glass:border-white/30 data-glass:dark:border-white/15 ${colorMap[color] || colorMap.neutral}`;
      },
    ],

    // 3. glass-card - 卡片（统计卡、列表项等，中等透明）
    [
      /^glass-card$/,
      () =>
        "data-glass:bg-white/8 data-glass:dark:bg-black/12 data-glass:backdrop-blur-12 data-glass:backdrop-saturate-180 data-glass:border data-glass:border-white/22 data-glass:dark:border-white/12 data-glass:shadow-none native-webkit:backdrop-filter-off",
    ],

    // 4. glass-panel - 面板容器（较透明，轻量感）
    [
      /^glass-panel$/,
      () =>
        "data-glass:bg-white/5 data-glass:dark:bg-black/8 data-glass:backdrop-blur-12 data-glass:backdrop-saturate-180 data-glass:border data-glass:border-white/18 data-glass:dark:border-white/10 data-glass:shadow-none native-webkit:backdrop-filter-off",
    ],

    // 5. glass-input - 输入框（最透明，突出内容）
    [
      /^glass-input$/,
      () =>
        "data-glass:bg-white/8 data-glass:dark:bg-black/10 data-glass:border data-glass:border-white/25 data-glass:dark:border-white/18",
    ],

    // 6. glass-btn-none - 透明按钮（仅保留交互反馈）
    [
      /^glass-btn-none$/,
      () =>
        "data-glass:bg-transparent data-glass:hover:bg-white/10 data-glass:dark:hover:bg-black/12",
    ],
  ],

  theme: {
    animation: {
      counts: {
        "app-toast-enter": "1",
        "app-toast-leave": "1",
        "app-toast-progress": "1",
        "tooltip-enter": "1",
        "playing-island-marquee": "infinite",
        "playing-island-enter": "1",
        "playing-island-leave": "1",
        "playing-island-content-in": "1",
        "playing-island-content-out": "1",
        "bigscreen-bg-in": "1",
        "bigscreen-kenburns": "1",
        "bigscreen-hint-dim": "1",
        "bigscreen-enter": "1",
      },
      durations: {
        "app-toast-enter": "450ms",
        "app-toast-leave": "280ms",
        "app-toast-progress": "4000ms",
        "tooltip-enter": "120ms",
        "playing-island-marquee": "8s",
        "playing-island-enter": "360ms",
        "playing-island-leave": "220ms",
        "playing-island-content-in": "260ms",
        "playing-island-content-out": "220ms",
        "bigscreen-bg-in": "600ms",
        "bigscreen-kenburns": "22s",
        "bigscreen-hint-dim": "4000ms",
        "bigscreen-enter": "320ms",
      },
      keyframes: {
        "app-toast-enter":
          "{0%{opacity:0;transform:translate3d(0,var(--app-toast-enter-y),0) scale(.96)}100%{opacity:1;transform:translate3d(0,0,0) scale(1)}}",
        "app-toast-leave":
          "{0%{transform:translate3d(0,0,0) scale(1)}100%{transform:translate3d(0,var(--app-toast-leave-y),0) scale(.96)}}",
        "app-toast-progress":
          "{0%{stroke-dashoffset:0}100%{stroke-dashoffset:100}}",
        "tooltip-enter":
          "{0%{opacity:0;transform:scale(.96)}100%{opacity:1;transform:scale(1)}}",
        "playing-island-marquee":
          "{0%,16%{transform:translateX(0)}84%,100%{transform:translateX(-50%)}}",
        "playing-island-enter":
          "{0%{opacity:0;transform:scaleX(.18) scaleY(.72)}62%{opacity:1;transform:scaleX(1.05) scaleY(1.02)}100%{opacity:1;transform:scaleX(1) scaleY(1)}}",
        "playing-island-leave":
          "{0%{opacity:1;transform:scaleX(1) scaleY(1)}100%{opacity:0;transform:scaleX(.22) scaleY(.74)}}",
        "playing-island-content-in":
          "{0%{opacity:0;filter:blur(2px)}100%{opacity:1;filter:blur(0)}}",
        "playing-island-content-out":
          "{0%{opacity:1;filter:blur(0)}100%{opacity:0;filter:blur(2px)}}",
        // 大屏模式的背景交叉淡入与 KenBurns 缓慢推进
        "bigscreen-bg-in": "{0%{opacity:0}100%{opacity:1}}",
        "bigscreen-kenburns":
          "{0%{transform:scale(1.04) translate3d(0,0,0)}100%{transform:scale(1.14) translate3d(-1.5%,-1%,0)}}",
        // 底栏按键提示 4s 后淡到 28%
        "bigscreen-hint-dim": "{0%,86%{opacity:1}100%{opacity:.28}}",
        // 入场：卡片 / 侧栏条目自下而上淡入，配合 42ms×idx 的错峰延迟
        "bigscreen-enter":
          "{0%{opacity:0;transform:translate3d(0,18px,0) scale(.96)}100%{opacity:1;transform:translate3d(0,0,0) scale(1)}}",
      },
      properties: {
        "app-toast-enter": {
          "animation-fill-mode": "both",
        },
        "app-toast-leave": {
          "animation-fill-mode": "both",
        },
        "app-toast-progress": {
          "animation-fill-mode": "forwards",
        },
        "tooltip-enter": {
          "animation-fill-mode": "both",
          "transform-origin": "center",
        },
        "playing-island-enter": {
          "animation-fill-mode": "both",
          "transform-origin": "center",
        },
        "playing-island-leave": {
          "animation-fill-mode": "both",
          "transform-origin": "center",
        },
        "playing-island-content-in": {
          "animation-fill-mode": "both",
        },
        "playing-island-content-out": {
          "animation-fill-mode": "both",
        },
        "bigscreen-bg-in": {
          "animation-fill-mode": "both",
        },
        "bigscreen-kenburns": {
          "animation-fill-mode": "both",
          "transform-origin": "center",
        },
        "bigscreen-hint-dim": {
          "animation-fill-mode": "forwards",
        },
        "bigscreen-enter": {
          "animation-fill-mode": "both",
          "transform-origin": "center",
        },
      },
      timingFns: {
        "app-toast-enter": "cubic-bezier(.22,1,.36,1)",
        "app-toast-leave": "cubic-bezier(.4,0,1,1)",
        "app-toast-progress": "linear",
        "tooltip-enter": "cubic-bezier(.16,1,.3,1)",
        "playing-island-enter": "cubic-bezier(.16,1,.3,1)",
        "playing-island-leave": "cubic-bezier(.4,0,1,1)",
        "playing-island-content-in": "cubic-bezier(.2,.9,.18,1)",
        "playing-island-content-out": "cubic-bezier(.4,0,.2,1)",
        "bigscreen-bg-in": "ease-out",
        "bigscreen-kenburns": "ease-out",
        "bigscreen-hint-dim": "ease-out",
        "bigscreen-enter": "cubic-bezier(.2,.9,.18,1)",
      },
    },
    colors: {
      // 基础中性色板 —— 冷调深蓝，锚点取自手机版 YukiHub 的 yh_bg / yh_card / yh_line。
      // 暗端（700~900）是界面的主背景与卡片，直接决定"像不像 YukiHub"；
      // 亮端（50~300）是同色系的浅色版，供亮色模式使用。
      brand: {
        50: "#FAFBFF",
        100: "#F1F3F9",
        150: "#E7EAF4",
        200: "#DDE2F0",
        300: "#C7CFE3",
        400: "#9AA4BF", // = yh_text_muted
        500: "#6E7A9B",
        600: "#4A5578",
        700: "#2D3658", // = yh_line
        750: "#222B49", // = yh_card_2
        800: "#171E33", // = yh_card
        900: "#0B1020", // = yh_bg
      },
      // 主色调 (primary) - YukiHub 柔和蓝。
      // 300 是手机版 yh_primary(#8AB4FF) 本身，供暗色模式的前景/强调文字使用；
      // 500/600 压深一档，保证亮色模式下"白字蓝底"按钮仍有足够对比度。
      primary: {
        50: "#F3F7FF",
        100: "#E5EEFF",
        200: "#CBDEFF",
        300: "#8AB4FF",
        400: "#7FA6F2",
        500: "#6E96E8",
        600: "#5A7CC9",
        700: "#46629F",
        800: "#35497A",
        900: "#2E4173",
      },
      // 次色调 (secondary) - YukiHub 樱粉 yh_secondary，与主色拉开色相差，用于强调与选中态
      secondary: {
        50: "#FFF4F8",
        100: "#FFE8F1",
        200: "#FFD0E1",
        300: "#FFB4CE",
        400: "#FF9FC0",
        500: "#FF8AB3",
        600: "#E8739C",
        700: "#C95B81",
        800: "#A34666",
        900: "#7A3049",
      },
      // 手机版原始令牌，按名字直取，便于对照手机版源码
      yh: {
        "bg": "#0B1020",
        "bg2": "#111936",
        "sidebar": "#10172A",
        "card": "#171E33",
        "card2": "#222B49",
        "primary": "#8AB4FF",
        "secondary": "#FF8AB3",
        "text": "#F5F7FF",
        "text-muted": "#9AA4BF",
        "line": "#2D3658",
        "success": "#34C759",
        "warning": "#FFCC00",
      },
      // 强调色 (Accent) - 星光蓝
      accent: {
        50: "#EFF6FF",
        100: "#DBEAFE",
        200: "#BFDBFE",
        300: "#93C5FD",
        400: "#60A5FA",
        500: "#3B82F6",
        600: "#2563EB",
        700: "#1D4ED8",
        800: "#1E40AF",
        900: "#1E3A8A",
      },
      // 中性色 (Neutral) - 月夜灰
      neutral: {
        50: "#F8FAFC",
        100: "#F1F5F9",
        200: "#E2E8F0",
        300: "#CBD5E1",
        400: "#94A3B8",
        500: "#64748B",
        600: "#475569",
        700: "#334155",
        800: "#1E293B",
        900: "#0F172A",
      },
      // 成功色 (Success) - 极光绿
      success: {
        50: "#ECFDF5",
        100: "#D1FAE5",
        200: "#A7F3D0",
        300: "#6EE7B7",
        400: "#34D399",
        500: "#10B981",
        600: "#059669",
        700: "#047857",
        800: "#065F46",
        900: "#064E3B",
      },
      // 警告色 (Warning) - 晨曦金
      warning: {
        50: "#FFFBEB",
        100: "#FEF3C7",
        200: "#FDE68A",
        300: "#FCD34D",
        400: "#FBBF24",
        500: "#F59E0B",
        600: "#D97706",
        700: "#B45309",
        800: "#92400E",
        900: "#78350F",
      },
      // 错误色 (Error) - 玫瑰红
      error: {
        50: "#FEF2F2",
        100: "#FEE2E2",
        200: "#FECACA",
        300: "#FCA5A5",
        400: "#F87171",
        500: "#EF4444",
        600: "#DC2626",
        700: "#B91C1C",
        800: "#991B1B",
        900: "#7F1D1D",
      },
      // 信息色 (Info) - 冰蓝
      info: {
        50: "#F0F9FF",
        100: "#E0F2FE",
        200: "#BAE6FD",
        300: "#7DD3FC",
        400: "#38BDF8",
        500: "#0EA5E9",
        600: "#0284C7",
        700: "#0369A1",
        800: "#075985",
        900: "#0C181D",
      },
    },
  },
});
