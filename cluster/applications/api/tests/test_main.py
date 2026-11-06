import asyncio
import base64
import json
import os
import unittest
import unittest.mock

import numpy

# pydantic-settings resolves .env against the working directory, so nothing here relies on it.
for name, value in {
    "REDIS_HOST": "localhost",
    "REDIS_PORT": "6379",
    "GITHUB_TOKEN": "test",
    "SLACK_BOT_TOKEN": "test",
    "GOOGLE_CLIENT_ID": "test",
    "GOOGLE_CLIENT_SECRET": "test",
    "GOOGLE_PRE_ISSUED_REFRESH_TOKEN": "test",
}.items():
    os.environ.setdefault(name, value)

import api.realtime_group
import cortex.rate_limit
import main
import websockets.asyncio.client

SAMPLE_RATE = 24000
SAMPLES_PER_FRAME = SAMPLE_RATE * api.realtime_group.FRAME_MILLISECONDS // 1000
WINDOW_SECONDS = 1
WINDOW_FRAMES = WINDOW_SECONDS * 1000 // api.realtime_group.FRAME_MILLISECONDS
AUDIO_FRAMES = WINDOW_FRAMES * 2
RECHECK_SECONDS = 0.01
TIMEOUT_SECONDS = 5

SESSION_UPDATE = json.dumps(
    {
        "type": "session.update",
        "session": {
            "audio": {"input": {"format": {"type": "audio/pcm", "rate": SAMPLE_RATE}}}
        },
    }
)


def speech(frames: int, seed: int) -> numpy.ndarray:
    generator = numpy.random.default_rng(seed)
    levels = numpy.repeat(generator.uniform(200, 8000, frames), SAMPLES_PER_FRAME)
    noise = generator.normal(size=frames * SAMPLES_PER_FRAME)
    return numpy.clip(levels * noise, -32000, 32000).astype(numpy.int16)


def audio_append(samples: numpy.ndarray, gain: float = 1.0) -> str:
    payload = base64.b64encode((samples * gain).astype(numpy.int16).tobytes()).decode()
    return json.dumps({"type": "input_audio_buffer.append", "audio": payload})


def response_event(event_type: str) -> str:
    if event_type == "response.done":
        return json.dumps(
            {
                "type": "response.done",
                "response": {"usage": {"input_tokens": 1, "output_tokens": 1}},
            }
        )
    return json.dumps({"type": event_type})


class ClientGone(Exception):
    pass


class FakeWebSocket:
    def __init__(self):
        self.scope = {"subprotocols": []}
        self.accepted = False
        self.sent = []
        self.inbox = asyncio.Queue()
        self.gone = False

    async def accept(self, subprotocol=None):
        self.accepted = True

    async def send_text(self, message):
        self.sent.append(message)

    async def iter_text(self):
        # a closed socket keeps failing, so a second pass over it must not block
        if self.gone:
            raise ClientGone
        while True:
            message = await self.inbox.get()
            if message is None:
                self.gone = True
                raise ClientGone
            yield message

    def send_from_client(self, message):
        self.inbox.put_nowait(message)

    def disconnect(self):
        self.inbox.put_nowait(None)

    def received_types(self):
        return [json.loads(message)["type"] for message in self.sent]


class FakeUpstream:
    def __init__(self):
        self.received = []
        self.outbox = asyncio.Queue()
        self.closed = False

    async def send(self, message):
        self.received.append(message)

    def __aiter__(self):
        return self

    async def __anext__(self):
        return await self.outbox.get()

    def deliver(self, message):
        self.outbox.put_nowait(message)


class FakeRateLimiter(cortex.rate_limit.RateLimiter):
    async def take(self, key, amount):
        pass

    async def remaining(self, key, limit):
        return cortex.rate_limit.RateLimitInfo(
            limit=limit, remaining=limit, reset_timestamp=0
        )


class TestCase(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.upstreams = []
        self.connections = []

        async def connect(url, **kwargs):
            upstream = FakeUpstream()
            self.upstreams.append(upstream)
            try:
                yield upstream
            finally:
                upstream.closed = True

        self.overrides = {
            "realtime_group_window_seconds": WINDOW_SECONDS,
            "realtime_group_recheck_seconds": RECHECK_SECONDS,
        }
        self.restore = {name: getattr(main.s, name) for name in self.overrides}
        for name, value in self.overrides.items():
            setattr(main.s, name, value)

        main.global_realtime_groups = None
        main.global_rate_limiter = FakeRateLimiter()
        self.patch = unittest.mock.patch.object(
            websockets.asyncio.client, "connect", connect
        )
        self.patch.start()

    async def asyncTearDown(self):
        for task in self.connections:
            task.cancel()
        for task in self.connections:
            try:
                await task
            except asyncio.CancelledError:
                pass
        self.patch.stop()
        for name, value in self.restore.items():
            setattr(main.s, name, value)
        main.global_realtime_groups = None
        main.global_rate_limiter = None

    async def until(self, condition):
        async with asyncio.timeout(TIMEOUT_SECONDS):
            while not condition():
                await asyncio.sleep(RECHECK_SECONDS)

    async def open_connection(self, user):
        websocket = FakeWebSocket()
        # the endpoint is called directly, so the header FastAPI would resolve is passed here
        self.connections.append(
            asyncio.create_task(main.realtime(websocket, "test-model", user))
        )
        await self.until(lambda: len(self.upstreams) == len(self.connections))
        return websocket

    async def send_audio(self, websocket, samples, gain=1.0):
        websocket.send_from_client(SESSION_UPDATE)
        websocket.send_from_client(audio_append(samples, gain))

    async def pair_on_the_same_audio(self, seed):
        source = speech(AUDIO_FRAMES, seed=seed)

        first = await self.open_connection("first")
        await self.send_audio(first, source)

        second = await self.open_connection("second")
        await self.send_audio(second, source, gain=0.4)
        await self.until(lambda: len(main.get_realtime_groups().groups) == 2)
        return first, second

    async def settle_the_pair(self, websockets_):
        # completing a response on both lets whichever one is allowed to join do so
        for upstream in self.upstreams[:2]:
            upstream.deliver(response_event("response.done"))
        await self.until(lambda: len(main.get_realtime_groups().groups) == 1)

        joined = next(i for i in (0, 1) if self.upstreams[i].closed)
        return joined, 1 - joined, websockets_[joined], websockets_[1 - joined]

    async def test_one_upstream_serves_both_connections(self):
        pair = await self.pair_on_the_same_audio(seed=1)
        joined, kept, follower, feeder = await self.settle_the_pair(pair)

        self.upstreams[kept].deliver(response_event("response.output_audio.delta"))
        await self.until(
            lambda: "response.output_audio.delta" in follower.received_types()
        )

        self.assertEqual(len(self.upstreams), 2)
        self.assertTrue(self.upstreams[joined].closed)
        self.assertFalse(self.upstreams[kept].closed)
        self.assertIn("response.output_audio.delta", feeder.received_types())

    async def test_a_late_joiner_receives_events_at_its_own_position(self):
        source = speech(WINDOW_FRAMES * 3, seed=9)

        feeder = await self.open_connection("feeder")
        await self.send_audio(feeder, source)

        # joined a window late and still a little behind, so it can only follow
        follower = await self.open_connection("follower")
        await self.send_audio(
            follower,
            source[WINDOW_FRAMES * SAMPLES_PER_FRAME : -5 * SAMPLES_PER_FRAME],
            gain=0.4,
        )
        await self.until(lambda: len(main.get_realtime_groups().groups) == 2)

        self.upstreams[1].deliver(response_event("response.done"))
        await self.until(lambda: self.upstreams[1].closed)
        before = len(follower.sent)

        self.upstreams[0].deliver(response_event("response.output_audio.delta"))
        await asyncio.sleep(RECHECK_SECONDS * 10)
        self.assertEqual(
            len(follower.sent), before, "released before the audio arrived"
        )

        follower.send_from_client(
            audio_append(source[-5 * SAMPLES_PER_FRAME :], gain=0.4)
        )
        await self.until(
            lambda: "response.output_audio.delta" in follower.received_types()
        )

    async def test_only_one_of_two_matching_connections_gives_up_its_upstream(self):
        pair = await self.pair_on_the_same_audio(seed=8)
        await self.settle_the_pair(pair)

        self.assertEqual(sum(1 for upstream in self.upstreams if upstream.closed), 1)
        self.assertEqual(len(self.upstreams), 2)

    async def test_the_switch_waits_for_the_response_to_finish(self):
        pair = await self.pair_on_the_same_audio(seed=2)

        # a response is already streaming on both when the match becomes available
        for upstream in self.upstreams[:2]:
            upstream.deliver(response_event("response.created"))
            for _ in range(3):
                upstream.deliver(response_event("response.output_audio.delta"))
            upstream.deliver(response_event("response.done"))

        await self.until(lambda: len(main.get_realtime_groups().groups) == 1)
        joined = next(i for i in (0, 1) if self.upstreams[i].closed)

        received = pair[joined].received_types()
        self.assertEqual(received.count("response.output_audio.delta"), 3)
        self.assertEqual(received[-1], "response.done")

    async def test_the_follower_takes_over_when_the_feeder_leaves(self):
        pair = await self.pair_on_the_same_audio(seed=3)
        _, _, follower, feeder = await self.settle_the_pair(pair)

        feeder.disconnect()
        await self.until(lambda: len(self.upstreams) > 2)

        taken_over = self.upstreams[-1]
        taken_over.deliver(response_event("response.output_audio.delta"))
        await self.until(
            lambda: "response.output_audio.delta" in follower.received_types()
        )
        self.assertFalse(taken_over.closed)

    async def test_a_settings_change_drops_a_pending_match(self):
        pair = await self.pair_on_the_same_audio(seed=10)
        # let one of them line up a match that has not been acted on yet
        await asyncio.sleep(RECHECK_SECONDS * 10)

        changed = json.loads(SESSION_UPDATE)
        changed["session"]["instructions"] = "translate"
        for websocket in pair:
            websocket.send_from_client(json.dumps(changed))
        await asyncio.sleep(RECHECK_SECONDS * 10)

        for upstream in self.upstreams[:2]:
            upstream.deliver(response_event("response.done"))
        await asyncio.sleep(RECHECK_SECONDS * 20)

        self.assertEqual(len(self.upstreams), 2)

    async def test_a_connection_alone_keeps_its_own_upstream(self):
        alone = await self.open_connection("alone")
        await self.send_audio(alone, speech(AUDIO_FRAMES, seed=4))

        self.upstreams[0].deliver(response_event("response.done"))
        await self.until(lambda: "response.done" in alone.received_types())

        self.assertEqual(len(self.upstreams), 1)
        self.assertFalse(self.upstreams[0].closed)

    async def test_unrelated_audio_never_joins(self):
        feeder = await self.open_connection("feeder")
        await self.send_audio(feeder, speech(AUDIO_FRAMES, seed=5))

        other = await self.open_connection("other")
        await self.send_audio(other, speech(AUDIO_FRAMES, seed=6))
        await self.until(lambda: len(main.get_realtime_groups().groups) == 2)

        self.upstreams[1].deliver(response_event("response.done"))
        await self.until(lambda: "response.done" in other.received_types())

        self.assertFalse(self.upstreams[1].closed)
        self.assertEqual(feeder.received_types(), [])

    async def test_non_pcm_audio_is_never_grouped(self):
        alone = await self.open_connection("alone")
        alone.send_from_client(
            json.dumps(
                {
                    "type": "session.update",
                    "session": {
                        "audio": {
                            "input": {
                                "format": {"type": "audio/g711_ulaw", "rate": 8000}
                            }
                        }
                    },
                }
            )
        )
        alone.send_from_client(audio_append(speech(AUDIO_FRAMES, seed=7)))

        self.upstreams[0].deliver(response_event("response.done"))
        await self.until(lambda: "response.done" in alone.received_types())

        self.assertEqual(main.get_realtime_groups().groups, {})


if __name__ == "__main__":
    unittest.main()
