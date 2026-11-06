import base64
import io
import threading

import openai
import PIL.Image

import meegeren.enhancer
import meegeren.telemetry

DECISIONS_THUMBNAIL_SIZE = 512
DECISIONS_JPEG_QUALITY = 90


class DecisionsEnhancer(meegeren.enhancer.Enhancer):
    def __init__(
        self,
        client: openai.OpenAI,
        model: str,
        context: str,
        reproduction_instructions: str,
        need_instructions: dict[str, str],
        prefer_instructions: str,
        harm_instructions: str,
        reproduction_threshold: float,
        need_threshold: float,
        prefer_threshold: float,
        harm_threshold: float,
        strength: float,
    ):
        self.client = client
        self.model = model
        self.context = context
        self.reproduction_instructions = reproduction_instructions
        self.need_instructions = need_instructions
        self.prefer_instructions = prefer_instructions
        self.harm_instructions = harm_instructions
        self.reproduction_threshold = reproduction_threshold
        self.need_threshold = need_threshold
        self.prefer_threshold = prefer_threshold
        self.harm_threshold = harm_threshold
        self.strength = strength

    @classmethod
    def from_environment(
        cls,
        model: str,
        context: str,
        reproduction_instructions: str,
        need_instructions: dict[str, str],
        prefer_instructions: str,
        harm_instructions: str,
        reproduction_threshold: float,
        need_threshold: float,
        prefer_threshold: float,
        harm_threshold: float,
        strength: float,
    ) -> DecisionsEnhancer:
        return cls(
            client=openai.OpenAI(),
            model=model,
            context=context,
            reproduction_instructions=reproduction_instructions,
            need_instructions=need_instructions,
            prefer_instructions=prefer_instructions,
            harm_instructions=harm_instructions,
            reproduction_threshold=reproduction_threshold,
            need_threshold=need_threshold,
            prefer_threshold=prefer_threshold,
            harm_threshold=harm_threshold,
            strength=strength,
        )

    def ask(self, images: list[PIL.Image.Image], instructions: str) -> float | None:
        content = [{"type": "input_text", "text": self.context}]
        for image in images:
            thumbnail = image.copy()
            thumbnail.thumbnail((DECISIONS_THUMBNAIL_SIZE, DECISIONS_THUMBNAIL_SIZE))
            buffer = io.BytesIO()
            thumbnail.save(buffer, format="JPEG", quality=DECISIONS_JPEG_QUALITY)
            content.append(
                {
                    "type": "input_image",
                    "image_url": f"data:image/jpeg;base64,{base64.b64encode(buffer.getvalue()).decode('ascii')}",
                }
            )
        decision = self.client.decisions.create(
            model=self.model,
            input=[{"role": "user", "content": content}],
            questions=[
                {"type": "predicate", "name": "answer", "instructions": instructions}
            ],
        )
        answer = decision.answers[0]
        return None if answer.type == "refusal" else answer.probability

    def enhance(
        self,
        image: PIL.Image.Image,
        strength: float | None,
        cancelled: threading.Event,
    ) -> tuple[PIL.Image.Image, list[str]]:
        with meegeren.telemetry.tracer.start_as_current_span("decisions.enhance"):
            return super().enhance(image, strength, cancelled)
