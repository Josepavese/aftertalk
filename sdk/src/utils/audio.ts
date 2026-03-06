export class AudioManager {
  private stream: MediaStream | null = null;

  constructor(private sampleRate: number = 16000) {}

  async start(): Promise<void> {
    this.stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  }

  stop(): void {
    this.stream?.getTracks().forEach(t => t.stop());
  }

  getSampleRate(): number {
    return this.sampleRate;
  }
}
