import type { enums } from "../../src/bindings/models";

import { enums as modelEnums } from "../../src/bindings/models";
import bangumiIconUrl from "../assets/providers/bangumi-icon.png";
import bangumiLogoUrl from "../assets/providers/bangumi-logo.png";
import hikarinagiIconUrl from "../assets/providers/hikarinagi-icon.webp";
import hikarinagiLogoUrl from "../assets/providers/hikarinagi-logo.svg";
import vndbLogoUrl from "../assets/providers/vndb-logo.svg";
import ymgalLogoUrl from "../assets/providers/ymgal-logo.png";

// 未萌暂无独立图标资源，用内联 SVG 生成一个「N」字标，避免新增二进制资源。
const NEXTMOE_ICON_URL = `data:image/svg+xml,${encodeURIComponent(
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#7C5CFF"/><stop offset="1" stop-color="#3B82F6"/></linearGradient></defs><rect width="64" height="64" rx="14" fill="url(#g)"/><path d="M21 45V19l22 26V19" fill="none" stroke="#fff" stroke-width="7" stroke-linecap="round" stroke-linejoin="round"/></svg>`,
)}`;

// 与手机版保持一致的可选来源集合。
export const ALL_METADATA_SOURCES: readonly enums.SourceType[] = [
  modelEnums.SourceType.VNDB,
  modelEnums.SourceType.Bangumi,
  modelEnums.SourceType.BangumiMirror,
  modelEnums.SourceType.Ymgal,
  modelEnums.SourceType.Hikarinagi,
  modelEnums.SourceType.NextMoe,
];

export const DEFAULT_ENABLED_METADATA_SOURCES: readonly enums.SourceType[] = [
  modelEnums.SourceType.VNDB,
  modelEnums.SourceType.Bangumi,
  modelEnums.SourceType.Ymgal,
  modelEnums.SourceType.Hikarinagi,
];

const VALID_METADATA_SOURCE_SET = new Set<string>(ALL_METADATA_SOURCES);

const METADATA_SOURCE_ICONS: Readonly<
  Partial<Record<enums.SourceType, string>>
> = {
  [modelEnums.SourceType.Bangumi]: bangumiLogoUrl,
  [modelEnums.SourceType.BangumiMirror]: bangumiLogoUrl,
  [modelEnums.SourceType.VNDB]: vndbLogoUrl,
  [modelEnums.SourceType.Ymgal]: ymgalLogoUrl,
  [modelEnums.SourceType.Hikarinagi]: hikarinagiLogoUrl,
  [modelEnums.SourceType.NextMoe]: NEXTMOE_ICON_URL,
};

const METADATA_SOURCE_COMPACT_ICONS: Readonly<
  Partial<Record<enums.SourceType, string>>
> = {
  ...METADATA_SOURCE_ICONS,
  [modelEnums.SourceType.Bangumi]: bangumiIconUrl,
  [modelEnums.SourceType.Hikarinagi]: hikarinagiIconUrl,
};

export function getMetadataSourceIcon(
  source: enums.SourceType,
  variant: "logo" | "compact" = "logo",
): string | undefined {
  return variant === "compact"
    ? METADATA_SOURCE_COMPACT_ICONS[source]
    : METADATA_SOURCE_ICONS[source];
}

export function getMetadataSourceURL(
  source: string | undefined,
  sourceId: string | undefined,
): string {
  const id = sourceId?.trim();
  if (!source || !id) {
    return "";
  }

  const encodedId = encodeURIComponent(id);
  switch (source) {
    case "vndb":
      return `https://vndb.org/${encodedId}`;
    // 镜像站与主站条目页一致，使用同一 subject id。
    case "bangumi":
    case "bangumi_mirror":
      return `https://bgm.tv/subject/${encodedId}`;
    case "ymgal":
      return `https://www.ymgal.games/ga/${encodedId}`;
    case "hikarinagi":
      return `https://www.hikarinagi.org/galgames/${encodedId}`;
    // nextmoe 的目录平台尚未开放 Web 前台，没有可跳转的作品页，故不给链接。
    default:
      return "";
  }
}

export function normalizeEnabledMetadataSources(
  sources: readonly string[] | undefined,
): enums.SourceType[] {
  if (!sources || sources.length === 0) {
    return [...DEFAULT_ENABLED_METADATA_SOURCES];
  }

  const normalized: enums.SourceType[] = [];
  const seen = new Set<string>();
  for (const source of sources) {
    const value = source.toLowerCase().trim();
    if (!VALID_METADATA_SOURCE_SET.has(value) || seen.has(value)) {
      continue;
    }
    seen.add(value);
    normalized.push(value as enums.SourceType);
  }

  return normalized.length > 0
    ? normalized
    : [...DEFAULT_ENABLED_METADATA_SOURCES];
}
