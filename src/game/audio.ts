import type { AudioTrack } from "./types";

type ProceduralMusicMood = Extract<AudioTrack["mood"], "rush" | "danger" | "settlement">;

type ProceduralMusicStep = {
  timeOffset: number;
  duration: number;
  frequency: number;
  type: OscillatorType;
  gain: number;
};

export class AudioDirector {
  private enabled = false;
  private currentTrack: HTMLAudioElement | null = null;
  private audioContext: AudioContext | null = null;
  private currentMood: AudioTrack["mood"] | null = null;
  private proceduralMood: AudioTrack["mood"] | null = null;
  private proceduralTimer: number | null = null;
  private proceduralNodes: AudioScheduledSourceNode[] = [];

  constructor(private readonly tracks: AudioTrack[]) {}

  dispose(): void {
    /*
     * AudioDirector 绑定的是一份内容包里的音轨列表。当前端从离线/内存兜底恢复到
     * PostgreSQL 内容包时，会销毁旧 Phaser 游戏并重建新的 AudioDirector。旧对象必须把
     * 正在播放的 HTMLAudioElement、合成音乐定时器和 AudioContext 一起停掉，否则新旧两套
     * 声音可能同时播放，调试时也很难判断声音来自哪一份内容包。
     */
    this.currentTrack?.pause();
    this.currentTrack = null;
    this.stopProceduralMusic();
    if (this.audioContext) {
      void this.audioContext.close().catch(() => undefined);
      this.audioContext = null;
    }
  }

  async unlock(): Promise<void> {
    if (!this.audioContext) {
      this.audioContext = new AudioContext();
    }

    if (this.audioContext.state === "suspended") {
      await this.audioContext.resume();
    }
  }

  setEnabled(enabled: boolean): void {
    this.enabled = enabled;

    if (!enabled) {
      this.currentTrack?.pause();
      this.stopProceduralMusic();
      return;
    }

    if (this.currentTrack) {
      void this.currentTrack.play().catch(() => undefined);
      return;
    }

    if (this.currentMood) {
      this.playMusic(this.currentMood);
    }
  }

  playMusic(mood: AudioTrack["mood"]): void {
    this.currentMood = mood;
    const track = this.tracks.find((candidate) => candidate.mood === mood && candidate.src);

    if (!this.enabled) {
      return;
    }

    if (!track) {
      this.startProceduralMusic(mood);
      return;
    }

    if (this.currentTrack?.src.endsWith(track.src) && this.proceduralTimer === null) {
      return;
    }

    this.stopProceduralMusic();
    this.currentTrack?.pause();
    this.currentTrack = new Audio(track.src);
    this.currentTrack.loop = true;
    this.currentTrack.volume = 0.58;
    void this.currentTrack.play().catch(() => undefined);
  }

  playPaymentTone(kind: "spend" | "income" | "danger"): void {
    if (!this.enabled || !this.audioContext) {
      return;
    }

    const now = this.audioContext.currentTime;
    const oscillator = this.audioContext.createOscillator();
    const gain = this.audioContext.createGain();
    const frequency = kind === "income" ? 520 : kind === "danger" ? 92 : 220;

    oscillator.type = kind === "income" ? "triangle" : "sawtooth";
    oscillator.frequency.setValueAtTime(frequency, now);
    oscillator.frequency.exponentialRampToValueAtTime(frequency * 2.4, now + 0.08);
    gain.gain.setValueAtTime(0.0001, now);
    gain.gain.exponentialRampToValueAtTime(0.12, now + 0.01);
    gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.16);

    oscillator.connect(gain);
    gain.connect(this.audioContext.destination);
    oscillator.start(now);
    oscillator.stop(now + 0.18);
  }

  private startProceduralMusic(mood: AudioTrack["mood"]): void {
    if (!this.audioContext || mood === "menu") {
      return;
    }

    if (this.proceduralTimer !== null && this.currentTrack === null && this.proceduralMood === mood) {
      return;
    }

    this.currentTrack?.pause();
    this.currentTrack = null;
    this.stopProceduralMusic();
    this.proceduralMood = mood;
    this.scheduleProceduralBar(mood);
    this.proceduralTimer = window.setInterval(() => this.scheduleProceduralBar(mood), this.proceduralBarMs(mood));
  }

  private stopProceduralMusic(): void {
    if (this.proceduralTimer !== null) {
      window.clearInterval(this.proceduralTimer);
      this.proceduralTimer = null;
    }
    this.proceduralMood = null;

    for (const node of this.proceduralNodes) {
      try {
        node.stop();
      } catch {
        // Oscillator nodes throw if they have already stopped. Stopping best-effort is enough here.
      }
    }
    this.proceduralNodes = [];
  }

  private scheduleProceduralBar(mood: ProceduralMusicMood): void {
    if (!this.audioContext || !this.enabled) {
      return;
    }

    /*
     * 这里的“音乐”不是下载文件，而是用 Web Audio 按小节临时合成。它解决的是当前阶段
     * 没有真实音乐素材时的体验空洞：玩家打开声音后，rush 提供收银节奏，danger 在后段
     * 提供更紧的低频压力，settlement 给结算页一个短循环。后端 audioTracks 仍然保留，
     * 以后拿到美术和音乐资源时，只要把 src 写成真实文件地址，就会优先走真实音频。
     */
    const baseTime = this.audioContext.currentTime + 0.03;
    const master = this.audioContext.createGain();
    master.gain.setValueAtTime(mood === "danger" ? 0.075 : mood === "settlement" ? 0.055 : 0.065, baseTime);
    master.connect(this.audioContext.destination);

    for (const step of this.proceduralSteps(mood)) {
      const oscillator = this.audioContext.createOscillator();
      const gain = this.audioContext.createGain();
      const startAt = baseTime + step.timeOffset;
      const endAt = startAt + step.duration;

      oscillator.type = step.type;
      oscillator.frequency.setValueAtTime(step.frequency, startAt);
      gain.gain.setValueAtTime(0.0001, startAt);
      gain.gain.exponentialRampToValueAtTime(step.gain, startAt + 0.012);
      gain.gain.exponentialRampToValueAtTime(0.0001, endAt);
      oscillator.connect(gain);
      gain.connect(master);
      oscillator.start(startAt);
      oscillator.stop(endAt + 0.02);
      oscillator.addEventListener("ended", () => {
        oscillator.disconnect();
        gain.disconnect();
        this.proceduralNodes = this.proceduralNodes.filter((node) => node !== oscillator);
      });
      this.proceduralNodes.push(oscillator);
    }

    window.setTimeout(() => master.disconnect(), this.proceduralBarMs(mood) + 180);
  }

  private proceduralBarMs(mood: AudioTrack["mood"]): number {
    return mood === "settlement" ? 1_600 : mood === "danger" ? 1_200 : 1_400;
  }

  private proceduralSteps(mood: ProceduralMusicMood): ProceduralMusicStep[] {
    if (mood === "settlement") {
      return [
        { timeOffset: 0, duration: 0.12, frequency: 330, type: "triangle", gain: 0.22 },
        { timeOffset: 0.22, duration: 0.12, frequency: 392, type: "triangle", gain: 0.2 },
        { timeOffset: 0.44, duration: 0.18, frequency: 523, type: "triangle", gain: 0.18 },
        { timeOffset: 0.88, duration: 0.2, frequency: 262, type: "sine", gain: 0.16 }
      ];
    }

    if (mood === "danger") {
      return [
        { timeOffset: 0, duration: 0.09, frequency: 82, type: "square", gain: 0.32 },
        { timeOffset: 0.18, duration: 0.06, frequency: 164, type: "sawtooth", gain: 0.2 },
        { timeOffset: 0.36, duration: 0.09, frequency: 92, type: "square", gain: 0.28 },
        { timeOffset: 0.54, duration: 0.06, frequency: 220, type: "sawtooth", gain: 0.16 },
        { timeOffset: 0.78, duration: 0.12, frequency: 73, type: "square", gain: 0.26 }
      ];
    }

    return [
      { timeOffset: 0, duration: 0.08, frequency: 110, type: "square", gain: 0.22 },
      { timeOffset: 0.18, duration: 0.06, frequency: 220, type: "triangle", gain: 0.13 },
      { timeOffset: 0.35, duration: 0.08, frequency: 147, type: "square", gain: 0.18 },
      { timeOffset: 0.52, duration: 0.06, frequency: 294, type: "triangle", gain: 0.12 },
      { timeOffset: 0.88, duration: 0.09, frequency: 123, type: "square", gain: 0.2 },
      { timeOffset: 1.06, duration: 0.06, frequency: 247, type: "triangle", gain: 0.12 }
    ];
  }
}
