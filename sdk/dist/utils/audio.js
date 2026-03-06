export class AudioManager {
    sampleRate;
    stream = null;
    constructor(sampleRate = 16000) {
        this.sampleRate = sampleRate;
    }
    async start() {
        this.stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    }
    stop() {
        this.stream?.getTracks().forEach(t => t.stop());
    }
    getSampleRate() {
        return this.sampleRate;
    }
}
//# sourceMappingURL=audio.js.map