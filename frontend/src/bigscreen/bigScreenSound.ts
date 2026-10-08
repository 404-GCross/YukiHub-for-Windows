/**
 * 大屏界面音效，对齐手机端 `bsSound`。
 *
 * 仓库里没有任何音频资源文件（也不打算为几个按键音新增二进制），所以这里直接用
 * Web Audio 合成：一个振荡器 + 指数衰减包络，够短够轻，不会盖住游戏本身的声音。
 * AudioContext 懒创建，并且在每次发声前尝试 resume —— 首次交互前浏览器可能把它留在
 * suspended 状态。
 */

/**
 * 音效种类，对齐手机端 `BigScreenSound.Sfx` 的三分类：
 * 焦点移动 / 确认（含收藏）/ 打开浮层（菜单、详情、返回）。
 */
export type BigScreenSound = "confirm" | "focus" | "open";

type SoundPreset = {
  duration: number;
  frequency: number;
  type: OscillatorType;
};

const SOUND_PRESETS: Record<BigScreenSound, SoundPreset> = {
  confirm: { duration: 0.12, frequency: 720, type: "triangle" },
  focus: { duration: 0.06, frequency: 480, type: "sine" },
  open: { duration: 0.1, frequency: 380, type: "sine" },
};

/**
 * 音量上限：手机端把用户音量换算后压到 0.8 —— 按键音只是反馈，
 * 不该盖过游戏本身的声音（`BigScreenSound` L64-70）。
 */
const MAX_VOLUME = 0.8;

/**
 * 焦点音的最小间隔，对齐手机端 `FOCUS_MIN_INTERVAL_MS = 55ms`。
 *
 * 摇杆/长按连发时（80ms 一次）如果每次都发声，会连成一片"嘟嘟嘟"；
 * 手机端用 55ms 节流滤掉这种密集重放，只保留手感的"点"。
 */
const FOCUS_MIN_INTERVAL_MS = 55;
let lastFocusSoundAt = 0;

let audioContext: AudioContext | null = null;
function getAudioContext(): AudioContext | null {
  if (
    typeof window === "undefined"
    || typeof window.AudioContext !== "function"
  ) {
    return null;
  }

  if (!audioContext) {
    try {
      audioContext = new AudioContext();
    }
    catch {
      return null;
    }
  }

  if (audioContext.state === "suspended") {
    void audioContext.resume();
  }
  return audioContext;
}

/**
 * @param kind 音效种类
 * @param volume 0–1 的相对音量（来自 `bigscreen_sound_volume`，默认 65%）。
 *   与手机端一致地把上限压到 0.8：按键音只是反馈，不该盖过游戏本身的声音。
 */
export function playBigScreenSound(kind: BigScreenSound, volume = 1) {
  const context = getAudioContext();
  if (!context) {
    return;
  }

  if (kind === "focus") {
    const now = performance.now();
    if (now - lastFocusSoundAt < FOCUS_MIN_INTERVAL_MS) {
      return;
    }
    lastFocusSoundAt = now;
  }

  const preset = SOUND_PRESETS[kind];
  const peak = MAX_VOLUME * clamp01(volume);
  if (peak <= 0) {
    return;
  }

  const startAt = context.currentTime;
  const oscillator = context.createOscillator();
  const gain = context.createGain();

  oscillator.type = preset.type;
  oscillator.frequency.setValueAtTime(preset.frequency, startAt);
  gain.gain.setValueAtTime(0.0001, startAt);
  gain.gain.exponentialRampToValueAtTime(peak, startAt + 0.01);
  gain.gain.exponentialRampToValueAtTime(0.0001, startAt + preset.duration);

  oscillator.connect(gain);
  gain.connect(context.destination);
  oscillator.start(startAt);
  oscillator.stop(startAt + preset.duration + 0.02);
}

function clamp01(value: number) {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.min(1, Math.max(0, value));
}
