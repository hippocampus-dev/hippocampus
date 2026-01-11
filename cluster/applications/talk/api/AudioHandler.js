export class AudioHandler {
    constructor() {
        this.audioContext = null;
        this.audioWorkletNode = null;
        this.audioQueue = [];
        this.isPlaying = false;
        this.nextStartTime = 0;
        this.onAudioData = null;
        this.currentSource = null;
        this.playbackRate = 1.0;
        this.grainSources = [];
        this.grainSize = 0.05;
        this.overlapFactor = 0.5;
    }

    async initialize(localStream) {
        this.audioContext = new (window.AudioContext || window.webkitAudioContext)({
            sampleRate: 24000
        });

        const source = this.audioContext.createMediaStreamSource(localStream);

        if (this.audioContext.audioWorklet) {
            try {
                const processorCode = `
                    class AudioProcessor extends AudioWorkletProcessor {
                        constructor() {
                            super();
                            this.bufferSize = 2400; // 100ms at 24kHz
                            this.buffer = new Float32Array(this.bufferSize);
                            this.bufferIndex = 0;
                        }
                        
                        process(inputs, outputs) {
                            const input = inputs[0];
                            if (input.length > 0) {
                                const inputChannel = input[0];
                                
                                for (let i = 0; i < inputChannel.length; i++) {
                                    this.buffer[this.bufferIndex++] = inputChannel[i];
                                    
                                    if (this.bufferIndex >= this.bufferSize) {
                                        this.port.postMessage({
                                            type: "audio",
                                            buffer: this.buffer.slice()
                                        });
                                        this.bufferIndex = 0;
                                    }
                                }
                            }
                            return true;
                        }
                    }
                    registerProcessor("audio-processor", AudioProcessor);
                `;

                const blob = new Blob([processorCode], {type: "application/javascript"});
                const processorUrl = URL.createObjectURL(blob);
                await this.audioContext.audioWorklet.addModule(processorUrl);
                URL.revokeObjectURL(processorUrl);

                this.audioWorkletNode = new AudioWorkletNode(this.audioContext, "audio-processor");

                this.audioWorkletNode.port.onmessage = (event) => {
                    if (event.data.type === "audio" && this.onAudioData) {
                        const float32Buffer = event.data.buffer;
                        const pcm16Buffer = this.convertFloat32ToPCM16(float32Buffer);
                        const base64Audio = btoa(String.fromCharCode(...new Uint8Array(pcm16Buffer)));
                        this.onAudioData(base64Audio);
                    }
                };

                source.connect(this.audioWorkletNode);
                this.audioWorkletNode.connect(this.audioContext.destination);

            } catch (error) {
                throw new Error("AudioWorklet is required for audio streaming");
            }
        } else {
            throw new Error("AudioWorklet is not supported in this browser");
        }
    }

    convertFloat32ToPCM16(float32Array) {
        const buffer = new ArrayBuffer(float32Array.length * 2);
        const view = new DataView(buffer);

        let offset = 0;
        for (let i = 0; i < float32Array.length; i++) {
            const sample = Math.max(-1, Math.min(1, float32Array[i]));
            view.setInt16(offset, sample * 0x7FFF, true);
            offset += 2;
        }

        return buffer;
    }

    decodeAudioData(base64Audio) {
        try {
            const binaryString = atob(base64Audio);
            const bytes = new Uint8Array(binaryString.length);
            for (let i = 0; i < binaryString.length; i++) {
                bytes[i] = binaryString.charCodeAt(i);
            }

            const pcm16 = new Int16Array(bytes.buffer);
            const float32 = new Float32Array(pcm16.length);
            for (let i = 0; i < pcm16.length; i++) {
                float32[i] = pcm16[i] / 0x7FFF;
            }

            const audioBuffer = this.audioContext.createBuffer(1, float32.length, this.audioContext.sampleRate);
            audioBuffer.copyToChannel(float32, 0);

            return audioBuffer;
        } catch (error) {
            console.error(`Failed to decode audio: ${error.message}`);
            return null;
        }
    }

    playAudioDelta(base64Audio) {
        const audioBuffer = this.decodeAudioData(base64Audio);
        if (audioBuffer) {
            this.audioQueue.push(audioBuffer);
            this.processAudioQueue();
        }
    }

    processAudioQueue() {
        if (!this.audioContext || this.audioQueue.length === 0 || this.isPlaying) {
            return;
        }

        this.isPlaying = true;

        const processNextBuffer = () => {
            if (this.audioQueue.length === 0) {
                this.isPlaying = false;
                this.nextStartTime = 0;
                return;
            }

            const audioBuffer = this.audioQueue.shift();
            if (!audioBuffer) {
                processNextBuffer();
                return;
            }

            if (this.playbackRate === 1.0) {
                const source = this.audioContext.createBufferSource();
                source.buffer = audioBuffer;
                source.connect(this.audioContext.destination);

                const currentTime = this.audioContext.currentTime;
                const startTime = Math.max(currentTime, this.nextStartTime);

                this.nextStartTime = startTime + audioBuffer.duration;

                source.onended = () => {
                    this.currentSource = null;
                    processNextBuffer();
                };

                this.currentSource = source;
                source.start(startTime);
            } else {
                this.playGranularSynthesis(audioBuffer, () => {
                    processNextBuffer();
                });
            }
        };

        processNextBuffer();
    }

    playGranularSynthesis(audioBuffer, onComplete) {
        const grainDuration = this.grainSize;
        const overlap = grainDuration * this.overlapFactor;
        const hopSize = grainDuration - overlap;
        const stretch = 1.0 / this.playbackRate;

        const outputDuration = audioBuffer.duration / this.playbackRate;
        const grainCount = Math.ceil(outputDuration / hopSize);

        let grainsScheduled = 0;
        let grainsCompleted = 0;

        const currentTime = this.audioContext.currentTime;
        const startTime = Math.max(currentTime, this.nextStartTime);

        for (let i = 0; i < grainCount; i++) {
            const grainStartTime = startTime + (i * hopSize);
            const sourcePosition = (i * hopSize) / stretch;

            if (sourcePosition + grainDuration > audioBuffer.duration) {
                break;
            }

            const source = this.audioContext.createBufferSource();
            source.buffer = audioBuffer;

            const gainNode = this.audioContext.createGain();
            const fadeInTime = grainDuration * 0.1;
            const fadeOutTime = grainDuration * 0.1;

            gainNode.gain.setValueAtTime(0, grainStartTime);
            gainNode.gain.linearRampToValueAtTime(1, grainStartTime + fadeInTime);
            gainNode.gain.setValueAtTime(1, grainStartTime + grainDuration - fadeOutTime);
            gainNode.gain.linearRampToValueAtTime(0, grainStartTime + grainDuration);

            source.connect(gainNode);
            gainNode.connect(this.audioContext.destination);

            source.onended = () => {
                grainsCompleted++;
                const index = this.grainSources.indexOf(source);
                if (index > -1) {
                    this.grainSources.splice(index, 1);
                }
                if (grainsCompleted === grainsScheduled) {
                    this.currentSource = null;
                    if (onComplete) onComplete();
                }
            };

            this.grainSources.push(source);
            source.start(grainStartTime, sourcePosition, grainDuration);
            grainsScheduled++;
        }

        this.nextStartTime = startTime + outputDuration;
        this.currentSource = this.grainSources[0];
    }

    stopPlayback() {
        if (this.currentSource) {
            try {
                this.currentSource.stop();
            } catch (e) {
            }
            this.currentSource = null;
        }

        this.grainSources.forEach(source => {
            try {
                source.stop();
            } catch (e) {
            }
        });
        this.grainSources = [];

        this.audioQueue = [];
        this.isPlaying = false;
        this.nextStartTime = 0;
    }

    setPlaybackRate(rate) {
        this.playbackRate = Math.max(0.5, Math.min(2.0, rate));

        if (this.currentSource && this.playbackRate === 1.0) {
            this.currentSource.playbackRate.value = this.playbackRate;
        }
    }

    cleanup() {
        this.stopPlayback();

        if (this.audioWorkletNode) {
            this.audioWorkletNode.disconnect();
            this.audioWorkletNode = null;
        }

        if (this.audioContext) {
            this.audioContext.close();
            this.audioContext = null;
        }
    }
}
