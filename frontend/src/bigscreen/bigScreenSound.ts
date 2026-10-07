/**
 * 大屏界面音效，对齐手机端 `bsSound`。
 *
 * 仓库里没有任何音频资源文件（也不打算为几个按键音新增二进制），所以这里直接用
 * Web Audio 合成：一个振荡器 + 指数衰减包络，够短够轻，不会盖住游戏本身的声音。
 * AudioContext 懒创建，并且在每次发声前尝试 resume —— 首次交互前浏览器可能把它留在
 * suspended 状态。
 */

export type BigScreenSound = "back" | "confirm" | "move" | "toggle";

type SoundPreset = {
  duration: number;
  frequency: number;
  type: OscillatorType;
};

const SOUND_PRESETS: Record<BigScreenSound, SoundPreset> = {
  back: { duration: 0.1, frequency: 320, type: "sine" },
  confirm: { duration: 0.12, frequency: 720, type: "triangle" },
  move: { duration: 0.06, frequency: 480, type: "sine" },
  toggle: { duration: 0.08, frequency: 880, type: "square" },
};

/**
 * 音量上限：手机端把用户音量换算后压到 0.8 —— 按键音只是反馈，
 * 不该盖过游戏本身的声音（`BigScreenSound` L64-70）。
 */
const MAX_VOLUME = 0.8;

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
