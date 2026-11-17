import base64
import typing
import unittest

import api.realtime_group
import numpy

SAMPLE_RATE = 24000
SAMPLES_PER_FRAME = SAMPLE_RATE * api.realtime_group.FRAME_MILLISECONDS // 1000
WINDOW_SECONDS = 10
WINDOW_FRAMES = WINDOW_SECONDS * 1000 // api.realtime_group.FRAME_MILLISECONDS


def session_update(
    audio_format: dict[str, typing.Any] | None = None,
) -> dict[str, typing.Any]:
    if audio_format is None:
        audio_format = {"type": "audio/pcm", "rate": SAMPLE_RATE}
    return {
        "type": "session.update",
        "session": {"audio": {"input": {"format": audio_format}}},
    }


def speech(frames: int, seed: int) -> numpy.ndarray:
    generator = numpy.random.default_rng(seed)
    levels = numpy.repeat(generator.uniform(200, 8000, frames), SAMPLES_PER_FRAME)
    noise = generator.normal(size=frames * SAMPLES_PER_FRAME)
    return numpy.clip(levels * noise, -32000, 32000).astype(numpy.int16)


def music(frames: int, seed: int, beat: int = 25) -> numpy.ndarray:
    generator = numpy.random.default_rng(seed)
    loud = numpy.arange(frames) % beat < beat // 3
    level = numpy.where(loud, 8000.0, 1500.0) * generator.uniform(0.9, 1.1, frames)
    noise = generator.normal(size=frames * SAMPLES_PER_FRAME)
    return numpy.clip(
        numpy.repeat(level, SAMPLES_PER_FRAME) * noise, -32000, 32000
    ).astype(numpy.int16)


def encode(samples: numpy.ndarray, gain: float = 1.0) -> str:
    return base64.b64encode((samples * gain).astype(numpy.int16).tobytes()).decode()


def registry() -> api.realtime_group.RealtimeGroupRegistry:
    return api.realtime_group.RealtimeGroupRegistry(
        window_seconds=WINDOW_SECONDS,
        backlog_seconds=30,
        match_threshold=0.7,
        sharpness_threshold=0.35,
    )


class TestCase(unittest.TestCase):
    def test_envelope_counts_frames_and_bounds_the_window(self):
        envelope = api.realtime_group.AudioEnvelope(SAMPLE_RATE, WINDOW_FRAMES)
        self.assertEqual(envelope.samples_per_frame, SAMPLES_PER_FRAME)

        envelope.append(encode(speech(10, seed=1)))
        self.assertEqual(envelope.frame_index, 10)
        self.assertFalse(envelope.is_full())

        envelope.append(encode(speech(WINDOW_FRAMES, seed=2)))
        self.assertEqual(envelope.frame_index, WINDOW_FRAMES + 10)
        self.assertTrue(envelope.is_full())
        self.assertEqual(len(envelope.curve()), WINDOW_FRAMES)

    def test_envelope_carries_partial_frames_across_appends(self):
        envelope = api.realtime_group.AudioEnvelope(SAMPLE_RATE, WINDOW_FRAMES)
        samples = speech(2, seed=3)

        envelope.append(encode(samples[: SAMPLES_PER_FRAME + 100]))
        self.assertEqual(envelope.frame_index, 1)

        envelope.append(encode(samples[SAMPLES_PER_FRAME + 100 :]))
        self.assertEqual(envelope.frame_index, 2)

    def test_build_envelope_rejects_unusable_audio_settings(self):
        groups = registry()

        self.assertIsNotNone(groups.build_envelope(session_update()))

        for rate in (0, 49, 192001, "24000", None):
            with self.subTest(rate=rate):
                event = session_update({"type": "audio/pcm", "rate": rate})
                self.assertIsNone(groups.build_envelope(event))

        self.assertIsNone(
            groups.build_envelope(
                session_update({"type": "audio/g711_ulaw", "rate": 8000})
            )
        )
        self.assertIsNone(
            groups.build_envelope({"type": "session.update", "session": None})
        )
        self.assertIsNone(
            groups.build_envelope(
                {"type": "session.update", "session": {"audio": None}}
            )
        )
        self.assertIsNone(groups.build_envelope({"type": "session.update"}))

    def test_find_recovers_the_offset_between_connections(self):
        groups = registry()
        source = speech(1200, seed=4)

        feeder = groups.build_envelope(session_update())
        feeder.append(encode(source[: 600 * SAMPLES_PER_FRAME]))
        group = groups.open("signature", feeder)

        # started together, so the follower has simply consumed 40 frames less
        follower = groups.build_envelope(session_update())
        follower.append(encode(source[: 560 * SAMPLES_PER_FRAME], gain=0.4))
        found = groups.find("signature", follower)
        self.assertIsNotNone(found)
        self.assertIs(found[0], group)
        self.assertEqual(found[1], 0)

        # joined 400 frames later, so it is at the same live position with its own counter
        latecomer = groups.build_envelope(session_update())
        latecomer.append(
            encode(source[400 * SAMPLES_PER_FRAME : 1000 * SAMPLES_PER_FRAME], gain=1.7)
        )
        feeder.append(
            encode(source[600 * SAMPLES_PER_FRAME : 1000 * SAMPLES_PER_FRAME])
        )
        found = groups.find("signature", latecomer)
        self.assertIsNotNone(found)
        self.assertEqual(found[1], -400)

    def test_the_order_between_groups_only_settles_level_connections(self):
        source = speech(1200, seed=21)

        def open_group(groups, frames, group_id, gain=1.0):
            envelope = groups.build_envelope(session_update())
            envelope.append(encode(source[: frames * SAMPLES_PER_FRAME], gain=gain))
            group = groups.open("signature", envelope)
            groups.groups.pop(group.group_id)
            group.group_id = group_id
            groups.groups[group_id] = group
            return group, envelope

        # the offset already says which way round these two go, so the order must not intervene
        offset = registry()
        ahead, _ = open_group(offset, 600, "z" * 32)
        behind, behind_envelope = open_group(offset, 560, "a" * 32, gain=0.4)

        found = offset.find("signature", behind_envelope, exclude=behind)
        self.assertIsNotNone(found)
        self.assertIs(found[0], ahead)

        # at the same position nothing else separates them, so only the order can
        level = registry()
        lower, lower_envelope = open_group(level, 600, "a" * 32)
        higher, higher_envelope = open_group(level, 600, "z" * 32, gain=0.4)

        self.assertIsNone(level.find("signature", lower_envelope, exclude=lower))
        found = level.find("signature", higher_envelope, exclude=higher)
        self.assertIsNotNone(found)
        self.assertIs(found[0], lower)

    def test_find_refuses_unrelated_audio_and_foreign_signatures(self):
        groups = registry()

        feeder = groups.build_envelope(session_update())
        feeder.append(encode(speech(600, seed=5)))
        group = groups.open("signature", feeder)

        unrelated = groups.build_envelope(session_update())
        unrelated.append(encode(speech(600, seed=6)))
        self.assertIsNone(groups.find("signature", unrelated))

        same = groups.build_envelope(session_update())
        same.append(encode(speech(600, seed=5)))
        self.assertIsNone(groups.find("other signature", same))
        self.assertIsNone(groups.find("signature", same, exclude=group))

    def test_find_skips_a_connection_ahead_of_the_feeder(self):
        groups = registry()
        source = speech(1200, seed=7)

        feeder = groups.build_envelope(session_update())
        feeder.append(encode(source[: 560 * SAMPLES_PER_FRAME]))
        groups.open("signature", feeder)

        ahead = groups.build_envelope(session_update())
        ahead.append(encode(source[: 600 * SAMPLES_PER_FRAME]))
        self.assertIsNone(groups.find("signature", ahead))

    def test_positions_too_far_apart_stop_matching(self):
        groups = registry()
        source = speech(1500, seed=20)

        feeder = groups.build_envelope(session_update())
        feeder.append(encode(source))
        groups.open("signature", feeder)

        # the same audio, but far enough back that the windows barely overlap
        distant = groups.build_envelope(session_update())
        distant.append(encode(source[: -200 * SAMPLES_PER_FRAME], gain=0.6))

        self.assertIsNone(groups.find("signature", distant))

    def test_periodic_audio_needs_more_than_a_high_peak(self):
        groups = registry()

        feeder = groups.build_envelope(session_update())
        feeder.append(encode(music(600, seed=11)))
        groups.open("signature", feeder)

        # a different track at the same tempo lines up on every beat
        other = groups.build_envelope(session_update())
        other.append(encode(music(600, seed=12), gain=0.5))

        self.assertGreater(
            api.realtime_group.strength_at(feeder.curve(), other.curve(), 0), 0.7
        )
        self.assertIsNone(groups.find("signature", other))

    def test_strength_at_reports_the_stored_alignment(self):
        groups = registry()
        source = speech(1200, seed=8)

        feeder = groups.build_envelope(session_update())
        feeder.append(encode(source[: 600 * SAMPLES_PER_FRAME]))
        groups.open("signature", feeder)

        follower = groups.build_envelope(session_update())
        follower.append(encode(source[: 560 * SAMPLES_PER_FRAME], gain=0.4))
        offset = groups.find("signature", follower)[1]

        lag = api.realtime_group.offset_of(feeder, follower, offset)
        self.assertEqual(lag, -40)
        self.assertGreater(
            api.realtime_group.strength_at(feeder.curve(), follower.curve(), lag), 0.7
        )

        drifted = groups.build_envelope(session_update())
        drifted.append(encode(speech(560, seed=9)))
        self.assertLess(
            api.realtime_group.strength_at(feeder.curve(), drifted.curve(), lag), 0.7
        )

    def test_signature_separates_models_and_session_settings(self):
        event = session_update()
        self.assertEqual(
            api.realtime_group.signature_of("model", event),
            api.realtime_group.signature_of("model", session_update()),
        )
        self.assertNotEqual(
            api.realtime_group.signature_of("model", event),
            api.realtime_group.signature_of("other", event),
        )

        changed = session_update()
        changed["session"]["instructions"] = "translate"
        self.assertNotEqual(
            api.realtime_group.signature_of("model", event),
            api.realtime_group.signature_of("model", changed),
        )

    def test_group_delivers_to_every_subscriber(self):
        groups = registry()
        envelope = groups.build_envelope(session_update())
        envelope.append(encode(speech(7, seed=10)))
        group = groups.open("signature", envelope)

        first = group.attach("first")
        second = group.attach("second")
        group.publish("response.done", "payload")

        self.assertEqual(first.get_nowait(), (7, "response.done", "payload"))
        self.assertEqual(second.get_nowait(), (7, "response.done", "payload"))

    def test_detach_and_close_release_every_subscriber(self):
        groups = registry()
        envelope = groups.build_envelope(session_update())
        group = groups.open("signature", envelope)

        detached = group.attach("detached")
        group.publish("response.done", "dropped on detach")
        group.detach("detached")
        self.assertIsNone(detached.get_nowait())
        self.assertNotIn("detached", group.subscribers)

        remaining = group.attach("remaining")
        groups.close(group)
        self.assertIsNone(remaining.get_nowait())
        self.assertFalse(group.is_open)
        self.assertEqual(groups.groups, {})

    def test_a_subscriber_that_stops_reading_is_dropped(self):
        groups = api.realtime_group.RealtimeGroupRegistry(
            window_seconds=WINDOW_SECONDS,
            backlog_seconds=1,
            match_threshold=0.7,
            sharpness_threshold=0.35,
        )
        envelope = groups.build_envelope(session_update())
        group = groups.open("signature", envelope)
        queue = group.attach("stalled")

        for _ in range(groups.queue_size + 1):
            group.publish("response.output_audio.delta", "payload")

        self.assertNotIn("stalled", group.subscribers)
        self.assertIsNone(queue.get_nowait())


if __name__ == "__main__":
    unittest.main()
