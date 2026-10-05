import asyncio
import base64
import collections
import hashlib
import json
import typing
import uuid

import numpy

FRAME_MILLISECONDS = 20

_PCM16_BYTES_PER_SAMPLE = 2

# Beyond professional audio rates the frame never fills, so AudioEnvelope.pending would grow without bound.
_MAXIMUM_SAMPLE_RATE = 192000

# Unrelated music at the same tempo peaks as high as 0.99.
_SHARPNESS_EXCLUSION_FRAMES = 10


class _Alignment(typing.NamedTuple):
    strength: float
    lag_frames: int
    sharpness: float


class AudioEnvelope:
    sample_rate: int
    frames: collections.deque[float]
    frame_index: int
    samples_per_frame: int
    pending: bytearray

    def __init__(self, sample_rate: int, window_frames: int) -> None:
        self.sample_rate = sample_rate
        self.frames = collections.deque(maxlen=window_frames)
        self.frame_index = 0
        self.samples_per_frame = sample_rate * FRAME_MILLISECONDS // 1000
        self.pending = bytearray()

    def append(self, audio: str) -> None:
        self.pending.extend(base64.b64decode(audio))
        frame_bytes = self.samples_per_frame * _PCM16_BYTES_PER_SAMPLE
        while len(self.pending) >= frame_bytes:
            samples = numpy.frombuffer(
                bytes(self.pending[:frame_bytes]), dtype=numpy.int16
            ).astype(numpy.float64)
            del self.pending[:frame_bytes]
            self.frames.append(float(numpy.sqrt(numpy.mean(samples * samples))))
            self.frame_index += 1

    def curve(self) -> numpy.ndarray:
        return numpy.asarray(self.frames, dtype=numpy.float64)

    def is_full(self) -> bool:
        return len(self.frames) == self.frames.maxlen


class RealtimeGroup:
    group_id: str
    signature: str
    envelope: AudioEnvelope
    subscribers: dict[str, asyncio.Queue[tuple[int, str, str] | None]]
    queue_size: int
    is_open: bool

    def __init__(
        self, signature: str, envelope: AudioEnvelope, queue_size: int
    ) -> None:
        self.group_id = uuid.uuid4().hex
        self.signature = signature
        self.envelope = envelope
        self.subscribers = {}
        self.queue_size = queue_size
        self.is_open = True

    def publish(self, event_type: str, message: str) -> None:
        entry = (self.envelope.frame_index, event_type, message)
        for subscriber_id, queue in list(self.subscribers.items()):
            try:
                queue.put_nowait(entry)
            except asyncio.QueueFull:
                self.detach(subscriber_id)

    def attach(self, subscriber_id: str) -> asyncio.Queue[tuple[int, str, str] | None]:
        queue: asyncio.Queue[tuple[int, str, str] | None] = asyncio.Queue(
            maxsize=self.queue_size
        )
        self.subscribers[subscriber_id] = queue
        return queue

    def detach(self, subscriber_id: str) -> None:
        queue = self.subscribers.pop(subscriber_id, None)
        if queue is None:
            return
        while not queue.empty():
            queue.get_nowait()
        queue.put_nowait(None)

    def close(self) -> None:
        self.is_open = False
        for subscriber_id in list(self.subscribers):
            self.detach(subscriber_id)


class RealtimeGroupRegistry:
    groups: dict[str, RealtimeGroup]
    window_frames: int
    queue_size: int
    match_threshold: float
    sharpness_threshold: float

    def __init__(
        self,
        window_seconds: int,
        backlog_seconds: int,
        match_threshold: float,
        sharpness_threshold: float,
    ) -> None:
        self.groups = {}
        self.window_frames = window_seconds * 1000 // FRAME_MILLISECONDS
        self.queue_size = backlog_seconds * 1000 // FRAME_MILLISECONDS
        self.match_threshold = match_threshold
        self.sharpness_threshold = sharpness_threshold

    def build_envelope(self, event: dict[str, typing.Any]) -> AudioEnvelope | None:
        section: typing.Any = event
        for key in ("session", "audio", "input", "format"):
            section = (section or {}).get(key)
        audio_format = section or {}
        if audio_format.get("type") != "audio/pcm":
            return None
        sample_rate = audio_format.get("rate")
        if not isinstance(sample_rate, int):
            return None
        # A zero-sample frame would spin AudioEnvelope.append forever.
        if sample_rate * FRAME_MILLISECONDS // 1000 < 1:
            return None
        if sample_rate > _MAXIMUM_SAMPLE_RATE:
            return None
        return AudioEnvelope(sample_rate, self.window_frames)

    def find(
        self,
        signature: str,
        envelope: AudioEnvelope,
        exclude: RealtimeGroup | None = None,
    ) -> tuple[RealtimeGroup, int] | None:
        candidate = envelope.curve()
        for group in list(self.groups.values()):
            if group is exclude:
                continue
            if not group.is_open or group.signature != signature:
                continue
            if not group.envelope.is_full():
                continue
            alignment = _align(group.envelope.curve(), candidate)
            # A connection ahead of the feeder has nothing to read yet, so it feeds its own upstream.
            if alignment.lag_frames > 0:
                continue
            # Level connections would otherwise each pick the other and both lose their feeder.
            if (
                alignment.lag_frames == 0
                and exclude is not None
                and group.group_id > exclude.group_id
            ):
                continue
            if alignment.strength < self.match_threshold:
                continue
            if alignment.sharpness < self.sharpness_threshold:
                continue
            return group, offset_of(group.envelope, envelope, alignment.lag_frames)
        return None

    def open(self, signature: str, envelope: AudioEnvelope) -> RealtimeGroup:
        group = RealtimeGroup(signature, envelope, self.queue_size)
        self.groups[group.group_id] = group
        return group

    def close(self, group: RealtimeGroup) -> None:
        self.groups.pop(group.group_id, None)
        group.close()


def signature_of(model: str, event: dict[str, typing.Any]) -> str:
    payload = json.dumps(
        {"model": model, "session": event.get("session", {})}, sort_keys=True
    )
    return hashlib.sha256(payload.encode()).hexdigest()


def _align(reference: numpy.ndarray, candidate: numpy.ndarray) -> _Alignment:
    a = _standardize(reference)
    b = _standardize(candidate)
    correlation = numpy.correlate(a, b, mode="full") / len(a)
    peak = int(numpy.argmax(correlation))
    distant = (
        numpy.abs(numpy.arange(len(correlation)) - peak) >= _SHARPNESS_EXCLUSION_FRAMES
    )
    runner_up = float(correlation[distant].max()) if distant.any() else 0.0
    strength = float(correlation[peak])
    return _Alignment(
        strength=strength,
        lag_frames=peak - (len(b) - 1),
        sharpness=strength - runner_up,
    )


# Lag and offset are duals of the same relation, so this converts either one into the other.
def offset_of(feeder: AudioEnvelope, follower: AudioEnvelope, lag_frames: int) -> int:
    return follower.frame_index - feeder.frame_index - lag_frames


def strength_at(
    reference: numpy.ndarray, candidate: numpy.ndarray, lag_frames: int
) -> float:
    a = _standardize(reference)
    b = _standardize(candidate)
    start = max(0, -lag_frames)
    stop = min(len(b), len(a) - lag_frames)
    if stop <= start:
        return 0.0
    return float(
        numpy.dot(a[start + lag_frames : stop + lag_frames], b[start:stop]) / len(a)
    )


def _standardize(values: numpy.ndarray) -> numpy.ndarray:
    deviation = values.std()
    if deviation == 0:
        return numpy.zeros_like(values)
    return (values - values.mean()) / deviation
