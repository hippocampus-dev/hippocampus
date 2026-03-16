export class ResponseAuditFilter {
  constructor({ onRelease, onReject }) {
    this.onRelease = onRelease;
    this.onReject = onReject;
    this._buffer = [];
  }

  reset() {
    this._buffer = [];
  }

  buffer(delta) {
    this._buffer.push(delta);
  }

  finalize(transcript) {
    const buffered = [...this._buffer];

    this.reset();

    const result = this.audit(transcript);

    if (result.passed) {
      this.onRelease(buffered);
    } else {
      this.onReject(transcript, result.reason);
    }
  }

  audit(_text) {
    return { passed: true, reason: "" };
  }
}

const DEFAULT_NG_WORDS = [
  // Add blocked words/phrases here
];

export class NGWordResponseAuditFilter extends ResponseAuditFilter {
  constructor({ onRelease, onReject, ngWords }) {
    super({ onRelease, onReject });
    this.ngWords = ngWords || DEFAULT_NG_WORDS;
  }

  audit(text) {
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
