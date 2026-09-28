"""Assert that rendered audio says what it was asked to say.

Takes (wav, expected text) pairs and transcribes each with a small ASR model.
The pipeline's own tests can only check that samples are shaped like audio;
reading the words back is what distinguishes speech from a plausible-looking
buzz. Matching is per-word so ASR slips on a single word do not fail the run.

Numbers do not survive the round trip as text: the talker spells "42" as
"forty-two" and the ASR writes it back as "42", so neither form matches the
other here. Keep digits out of the expected text.
"""

import re
import sys

import librosa
import numpy
import transformers

ASR_MODEL = "openai/whisper-base.en"
ASR_SAMPLE_RATE = 16000
# Below this, the transcript is a different utterance rather than a misheard one.
MINIMUM_WORD_RECALL = 0.7


def words(text: str) -> list[str]:
    return re.findall(r"[a-z]+", text.lower())


def main(pairs: list[tuple[str, str]]) -> int:
    recognizer = transformers.pipeline(
        "automatic-speech-recognition", model=ASR_MODEL, device=-1
    )

    failures = 0
    for wav_path, expected in pairs:
        samples, _ = librosa.load(wav_path, sr=ASR_SAMPLE_RATE, mono=True)
        transcript = recognizer(samples.astype(numpy.float32))["text"]

        expected_words = words(expected)
        heard = set(words(transcript))
        recalled = [word for word in expected_words if word in heard]
        recall = len(recalled) / len(expected_words)

        status = "ok" if recall >= MINIMUM_WORD_RECALL else "FAILED"
        if recall < MINIMUM_WORD_RECALL:
            failures += 1
        print(f"{status}: {wav_path} recall={recall:.2f}")
        print(f"  expected: {expected}")
        print(f"  heard:    {transcript.strip()}")

    return 1 if failures else 0


if __name__ == "__main__":
    arguments = sys.argv[1:]
    if not arguments or len(arguments) % 2 != 0:
        print("usage: transcribe.py <wav> <expected text> [<wav> <expected text> ...]")
        sys.exit(1)
    sys.exit(main(list(zip(arguments[::2], arguments[1::2]))))
