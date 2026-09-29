import type { BigScreenEffectLevel } from "./constants";
import { useEffect, useRef } from "react";

interface BigScreenAtmosphereProps {
  /** 当前焦点封面地址，用于提取粒子主色 */
  coverUrl: string;
  /** 特效档位，off 时整层不渲染 */
  level: BigScreenEffectLevel;
  /** 焦点游戏标识，取不到封面主色时用它派生兜底色相 */
  seed: string;
}

interface Particle {
  alpha: number;
  drift: number;
  phase: number;
  radius: number;
  speed: number;
  x: number;
  y: number;
}

interface ParticleConfig {
  count: number;
  maxRadius: number;
  minRadius: number;
  speed: number;
}

const PARTICLE_CONFIGS: Record<
  Exclude<BigScreenEffectLevel, "off">,
  ParticleConfig
> = {
  high: { count: 96, maxRadius: 3.4, minRadius: 1, speed: 1.1 },
  low: { count: 38, maxRadius: 2.6, minRadius: 0.8, speed: 0.7 },
};

/** 封面取色时的采样边长，够小以避开耗时的解码 */
const COVER_SAMPLE_SIZE = 24;

/** 按 seed 派生色相，作为取色失败时的兜底强调色 */
function seedAccent(seed: string) {
  let hue = 0;
  for (let index = 0; index < seed.length; index += 1) {
    hue = (hue * 31 + seed.charCodeAt(index)) % 360;
  }
  return `hsl(${hue}, 85%, 74%)`;
}

/**
 * 取封面平均色并往亮端抬一档：暗封面直接取均值会发灰，提亮后才适合当粒子色。
 * 跨域封面会让 canvas 变成 tainted，此时返回 null 由调用方回落。
 */
function extractAccent(url: string): Promise<string | null> {
  if (!url) {
    return Promise.resolve(null);
  }

  return new Promise((resolve) => {
    const image = new Image();
    image.crossOrigin = "anonymous";

    image.onload = () => {
      try {
        const canvas = document.createElement("canvas");
        canvas.width = COVER_SAMPLE_SIZE;
        canvas.height = COVER_SAMPLE_SIZE;
        const context = canvas.getContext("2d");
        if (!context) {
          resolve(null);
          return;
        }

        context.drawImage(image, 0, 0, COVER_SAMPLE_SIZE, COVER_SAMPLE_SIZE);
        const { data } = context.getImageData(
          0,
          0,
          COVER_SAMPLE_SIZE,
          COVER_SAMPLE_SIZE,
        );

        let red = 0;
        let green = 0;
        let blue = 0;
        let samples = 0;
        for (let index = 0; index < data.length; index += 4) {
          red += data[index];
          green += data[index + 1];
          blue += data[index + 2];
          samples += 1;
        }
        if (samples === 0) {
          resolve(null);
          return;
        }

        const lift = (value: number) =>
          Math.min(255, Math.round((value / samples) * 0.55 + 96));
        resolve(`rgb(${lift(red)}, ${lift(green)}, ${lift(blue)})`);
      }
      catch {
        resolve(null);
      }
    };
    image.onerror = () => resolve(null);
    image.src = url;
  });
}

/**
 * 大屏氛围层，对齐手机端 `bsSnow`：随焦点封面主色变化的缓降粒子。
 *
 * 桌面端比手机端多一档强度设置（`off` / `low` / `high`），粒子上行还是下行与
 * 手机端一致为**下落**并带左右摆动；主色由封面取平均色得到，跨域取不到时
 * 按 gameId 派生色相兜底，因此整层不依赖任何额外资源。
 */
export function BigScreenAtmosphere({
  coverUrl,
  level,
  seed,
}: BigScreenAtmosphereProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const glowRef = useRef<HTMLDivElement | null>(null);
  const accentRef = useRef(seedAccent(seed));

  useEffect(() => {
    let active = true;

    const applyAccent = (color: string) => {
      accentRef.current = color;
      const glow = glowRef.current;
      if (glow) {
        glow.style.background = `radial-gradient(circle at 28% 18%, ${color}, transparent 62%)`;
      }
    };

    applyAccent(seedAccent(seed));
    void extractAccent(coverUrl).then((color) => {
      if (active && color) {
        applyAccent(color);
      }
    });

    return () => {
      active = false;
    };
  }, [coverUrl, seed]);

  useEffect(() => {
    if (level === "off") {
      return;
    }

    const canvas = canvasRef.current;
    const context = canvas?.getContext("2d");
    if (!canvas || !context) {
      return;
    }

    const config = PARTICLE_CONFIGS[level];
    let width = 0;
    let height = 0;
    let particles: Particle[] = [];
    let frame = 0;

    const createParticles = () => {
      particles = Array.from({ length: config.count }, () => ({
        alpha: 0.12 + Math.random() * 0.3,
        drift: 0.12 + Math.random() * 0.3,
        phase: Math.random() * Math.PI * 2,
        radius:
          config.minRadius
          + Math.random() * (config.maxRadius - config.minRadius),
        speed: config.speed * (0.6 + Math.random() * 0.8),
        x: Math.random() * width,
        y: Math.random() * height,
      }));
    };

    const resize = () => {
      const ratio = Math.min(window.devicePixelRatio || 1, 2);
      width = canvas.clientWidth;
      height = canvas.clientHeight;
      canvas.width = Math.round(width * ratio);
      canvas.height = Math.round(height * ratio);
      context.setTransform(ratio, 0, 0, ratio, 0, 0);
      createParticles();
    };

    const step = () => {
      context.clearRect(0, 0, width, height);
      context.fillStyle = accentRef.current;
      context.shadowColor = accentRef.current;
      context.shadowBlur = 8;

      for (const particle of particles) {
        particle.y += particle.speed;
        particle.phase += 0.012;
        particle.x += Math.sin(particle.phase) * particle.drift;

        if (particle.y - particle.radius > height) {
          particle.y = -particle.radius;
          particle.x = Math.random() * width;
        }
        if (particle.x < -10) {
          particle.x = width + 10;
        }
        else if (particle.x > width + 10) {
          particle.x = -10;
        }

        context.globalAlpha = particle.alpha;
        context.beginPath();
        context.arc(particle.x, particle.y, particle.radius, 0, Math.PI * 2);
        context.fill();
      }

      context.globalAlpha = 1;
      frame = window.requestAnimationFrame(step);
    };

    resize();
    window.addEventListener("resize", resize);
    frame = window.requestAnimationFrame(step);

    return () => {
      window.cancelAnimationFrame(frame);
      window.removeEventListener("resize", resize);
    };
  }, [level]);

  if (level === "off") {
    return null;
  }

  return (
    <div
      className="pointer-events-none absolute inset-0 overflow-hidden"
      aria-hidden="true"
    >
      {level === "high" && (
        <div
          ref={(node) => {
            glowRef.current = node;
            if (node) {
              node.style.background = `radial-gradient(circle at 28% 18%, ${accentRef.current}, transparent 62%)`;
            }
          }}
          className="absolute inset-0 opacity-25"
        />
      )}
      <canvas ref={canvasRef} className="absolute inset-0 h-full w-full" />
    </div>
  );
}
