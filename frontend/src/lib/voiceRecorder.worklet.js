/* AudioWorklet scope: audio stays in memory until the user presses Stop. */
class VoiceRecorderProcessor extends AudioWorkletProcessor {
  constructor() {
    super();
    this.samples = new Float32Array(2048);
    this.offset = 0;
    this.stopped = false;
    this.port.onmessage = ({ data }) => {
      if (data === 'stop') {
        this.stopped = true;
        this.flush();
        this.port.postMessage({ stopped: true });
      }
    };
  }
  flush() {
    if (!this.offset) return;
    const samples = this.samples.slice(0, this.offset);
    this.port.postMessage({ samples }, [samples.buffer]);
    this.offset = 0;
  }
  process(inputs) {
    if (this.stopped) return false;
    const channel = inputs[0]?.[0];
    if (channel) {
      for (let i = 0; i < channel.length; i++) {
        this.samples[this.offset++] = channel[i];
        if (this.offset === this.samples.length) this.flush();
      }
    }
    return true;
  }
}
registerProcessor('helpin-voice-recorder', VoiceRecorderProcessor);
