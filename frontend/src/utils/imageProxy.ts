const IMAGE_PROXY_PATH = "/proxy/image";

/**
 * 本次会话里已知加载失败的图片地址 → 过期时间戳。
 *
 * 存在的理由：图片加载失败（国内直连不通的 vndb / bgm.tv 域名、代理超时等）
 * 之后切换页面会重新挂载 `<img>`，没有记忆就会把同一串超时再跑一遍，用户的
 * 观感就是「每切一次页面，封面都要重新加载一次」。带 TTL 是为了不把一次
 * 偶发失败永久钉死。
 */
const FAILED_IMAGE_TTL_MS = 5 * 60 * 1000;
const FAILED_IMAGE_LIMIT = 512;
const failedImageSources = new Map<string, number>();

/** 记录一个加载失败的地址。 */
export function markImageSourceFailed(src: string): void {
  const value = src?.trim();
  if (!value) {
    return;
  }
  // Map 保持插入顺序，超限时丢掉最早的一条即可。
  if (failedImageSources.size >= FAILED_IMAGE_LIMIT) {
    const oldest = failedImageSources.keys().next();
    if (!oldest.done) {
      failedImageSources.delete(oldest.value);
    }
  }
  failedImageSources.set(value, Date.now() + FAILED_IMAGE_TTL_MS);
}

/** 该地址是否在最近的失败记忆里。 */
export function isImageSourceFailed(src: string | null | undefined): boolean {
  const value = src?.trim();
  if (!value) {
    return false;
  }
  const expiresAt = failedImageSources.get(value);
  if (expiresAt === undefined) {
    return false;
  }
  if (expiresAt <= Date.now()) {
    failedImageSources.delete(value);
    return false;
  }
  return true;
}

/**
 * 清空失败记忆。用户主动「刷新」或重新下载封面时调用，
 * 否则刚刚修好的地址会被旧记忆挡住。
 */
export function clearFailedImageSources(): void {
  failedImageSources.clear();
}

export function shouldProxyImageSrc(
  src: string | null | undefined,
): src is string {
  const value = src?.trim();
  if (!value) {
    return false;
  }

  if (!/^https?:\/\//i.test(value)) {
    return false;
  }

  try {
    const url = new URL(value);
    return url.hostname.toLowerCase() !== "wails.localhost";
  }
  catch {
    return false;
  }
}

export function proxiedImageSrc(src: string | null | undefined): string {
  const value = src?.trim() ?? "";
  if (!shouldProxyImageSrc(value)) {
    return value;
  }

  const params = new URLSearchParams({ url: value });
  return `${IMAGE_PROXY_PATH}?${params.toString()}`;
}

export function imageSourceCandidates(
  src: string | null | undefined,
  fallbackSrc?: string | null | undefined,
): string[] {
  const sources: string[] = [];
  const addSource = (value: string | null | undefined) => {
    const normalizedValue = value?.trim() ?? "";
    if (!normalizedValue) {
      return;
    }

    const proxyValue = proxiedImageSrc(normalizedValue);
    if (proxyValue && !sources.includes(proxyValue)) {
      sources.push(proxyValue);
    }
    if (normalizedValue !== proxyValue && !sources.includes(normalizedValue)) {
      sources.push(normalizedValue);
    }
  };

  addSource(src);
  addSource(fallbackSrc);
  return sources;
}

/**
 * 从候选地址里剔除最近失败的，避免每次挂载都重跑一遍超时。
 *
 * 全部候选都失败过时返回空数组——此时本轮不该再发请求，由调用方直接渲染占位。
 */
export function loadableImageSources(candidates: string[]): string[] {
  return candidates.filter(candidate => !isImageSourceFailed(candidate));
}

export type ImageDimensions = {
  width: number;
  height: number;
};

function loadImageDimensions(src: string): Promise<ImageDimensions> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.referrerPolicy = "no-referrer";
    image.onload = () => {
      resolve({
        width: image.naturalWidth,
        height: image.naturalHeight,
      });
    };
    image.onerror = () => reject(new Error(`Failed to load image: ${src}`));
    image.src = src;
  });
}

export async function preloadImageDimensions(
  src: string | null | undefined,
  fallbackSrc?: string | null | undefined,
): Promise<ImageDimensions | null> {
  const candidates = imageSourceCandidates(src, fallbackSrc);
  for (const candidate of candidates) {
    try {
      return await loadImageDimensions(candidate);
    }
    catch {
      // Continue with the same fallback order used by ProxyImage.
    }
  }
  return null;
}
