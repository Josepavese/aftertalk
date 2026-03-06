export declare class AudioManager {
    private sampleRate;
    private stream;
    constructor(sampleRate?: number);
    start(): Promise<void>;
    stop(): void;
    getSampleRate(): number;
}
//# sourceMappingURL=audio.d.ts.map