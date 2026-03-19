import { AftertalkError } from '../errors.js';

export interface AudioManagerOptions {
  constraints?: MediaTrackConstraints;
}

export class AudioManager {
  private stream?: MediaStream;
  private _muted = false;

  get muted(): boolean {
    return this._muted;
  }

  get active(): boolean {
    return this.stream?.active ?? false;
  }

  async acquire(constraints?: MediaTrackConstraints): Promise<MediaStream> {
    // Stop any existing stream before acquiring a new one to avoid resource leak.
    this.release();

    const audioConstraints: MediaTrackConstraints = {
      echoCancellation: true,
      noiseSuppression: true,
      sampleRate: 48_000,
      ...constraints,
    };

    try {
      this.stream = await navigator.mediaDevices.getUserMedia({ audio: audioConstraints, video: false });
      return this.stream;
    } catch (err) {
      if (err instanceof DOMException) {
        if (err.name === 'NotAllowedError' || err.name === 'PermissionDeniedError') {
          throw new AftertalkError('audio_permission_denied', {
            message: 'Microphone access denied',
            details: err,
          });
        }
        if (err.name === 'NotFoundError' || err.name === 'DevicesNotFoundError') {
          throw new AftertalkError('audio_device_not_found', {
            message: 'No audio input device found',
            details: err,
          });
        }
      }
      throw new AftertalkError('audio_permission_denied', { details: err });
    }
  }

  setMuted(muted: boolean): void {
    this._muted = muted;
    if (this.stream) {
      for (const track of this.stream.getAudioTracks()) {
        track.enabled = !muted;
      }
    }
  }

  release(): void {
    if (this.stream) {
      for (const track of this.stream.getTracks()) {
        track.stop();
      }
      this.stream = undefined;
    }
  }
}
