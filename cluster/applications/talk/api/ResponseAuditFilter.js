const DEFAULT_DEBOUNCE_TEXT_LENGTH = 100;

function createAttemptState({ blocked = false } = {}) {
  return {
    audioBuffer: [],
    transcriptBuffer: "",
    lastAuditIndex: 0,
    blocked,
  };
}

export class ResponseAuditFilter {
  constructor({
    onRelease,
    onReject,
    debounceTextLength = DEFAULT_DEBOUNCE_TEXT_LENGTH,
  }) {
    this.onRelease = onRelease;
    this.onReject = onReject;
    this.debounceTextLength = debounceTextLength;
    this._attempt = createAttemptState();
  }

  get rejected() {
    return this._attempt.blocked;
  }

  startAttempt() {
    this._attempt = createAttemptState();
  }

  bufferAudio(delta) {
    if (this._attempt.blocked) return;
    this._attempt.audioBuffer.push(delta);
  }

  bufferTranscript(delta) {
    if (this._attempt.blocked) return;
    this._attempt.transcriptBuffer += delta;
  }

  async checkTranscript() {
    if (this._attempt.blocked) return;
    if (!this._shouldRunAudit()) return;

    const transcript = this._attempt.transcriptBuffer;
    const result = await this.audit(transcript);
    if (!result.passed) {
      this._attempt = createAttemptState({ blocked: true });
      await this.onReject(transcript, result.reason);
    }
  }

  async finalize(transcript) {
    if (this._attempt.blocked) return;

    const result = await this.audit(transcript);
    if (!result.passed) {
      this._attempt = createAttemptState({ blocked: true });
      await this.onReject(transcript, result.reason);
      return;
    }

    const buffered = [...this._attempt.audioBuffer];
    this._attempt = createAttemptState();
    await this.onRelease(buffered, transcript);
  }

  async audit(_text) {
    return { passed: true, reason: "" };
  }

  _shouldRunAudit() {
    if (this.debounceTextLength <= 0) return false;

    const currentIndex = Math.floor(
      this._attempt.transcriptBuffer.length / this.debounceTextLength,
    );
    if (currentIndex <= this._attempt.lastAuditIndex) return false;

    this._attempt.lastAuditIndex = currentIndex;
    return true;
  }
}

export class NGWordFilter extends ResponseAuditFilter {
  constructor({ ngWords, ...options }) {
    super(options);
    this.ngWords = ngWords;
  }

  async audit(text) {
    const lower = text.toLowerCase();
    const matched = this.ngWords.filter((word) =>
      lower.includes(word.toLowerCase()),
    );

    if (matched.length === 0) {
      return { passed: true, reason: "" };
    }

    return { passed: false, reason: `matched [${matched.join(", ")}]` };
  }
}
