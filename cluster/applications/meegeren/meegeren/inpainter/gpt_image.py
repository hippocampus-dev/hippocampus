import base64
import io
import math
import threading
import typing

import numpy
import openai
import PIL.Image

import meegeren.inpainter
import meegeren.telemetry
import meegeren.upscaler

GPT_IMAGE_PROMPT = "Remove everything in the masked area and fill it with what would be behind it, matching its surroundings. Keep everything outside the masked area unchanged."
GPT_IMAGE_PAINT_PROMPTS = {
    "replace": "Replace everything in the masked area with {prompt}, matching the perspective and lighting of its surroundings, together with the shadows it casts. Keep everything outside the masked area unchanged.",
    "restyle": "Change the {prompt} in the masked area, keeping its shape, structure, position, lighting and shadows. Keep everything outside the masked area unchanged.",
    "insert": "Add {prompt} in the masked area, keeping what is already there, matching the perspective and lighting of its surroundings, together with the shadows it casts. Keep everything outside the masked area unchanged.",
    "background": "Replace everything in the masked area with {prompt} as the background behind the subjects outside it, matching their perspective and lighting, together with the shadows they cast on it. Keep everything outside the masked area unchanged.",
}
# https://developers.openai.com/api/reference/python/resources/images/methods/edit
GPT_IMAGE_SIZE_MODULO = 16
GPT_IMAGE_MAX_ASPECT_RATIO = 3
GPT_IMAGE_PIXELS = 1536 * 1024


class GptImageInpainter(meegeren.inpainter.Inpainter, meegeren.inpainter.Painter):
    # Relit as a whole, gpt-image-2.5-sunburst drew a room of another layout
    # gpt-image-2.5-sunburst rejects input_fidelity
    relights = False

    def __init__(
        self, client: openai.OpenAI, model: str, quality: str, whole_image: bool
    ):
        self.client = client
        self.model = model
        self.quality = quality
        self.whole_image = whole_image
        self.pastes_back = not whole_image

    @classmethod
    def from_environment(
        cls, model: str, quality: str, whole_image: bool
    ) -> GptImageInpainter:
        return cls(
            client=openai.OpenAI(),
            model=model,
            quality=quality,
            whole_image=whole_image,
        )

    def inpaint(
        self,
        image: PIL.Image.Image,
        masks: list[numpy.ndarray],
        feathering: int,
        upscaler: meegeren.upscaler.Upscaler | None,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        with meegeren.telemetry.tracer.start_as_current_span("gpt_image.inpaint"):
            # The model fills whatever is masked, so every target is filled at once
            return self.edit(
                image,
                numpy.logical_or.reduce(masks),
                GPT_IMAGE_PROMPT,
                self.whole_image,
                feathering,
                upscaler,
                cancelled,
            )

    def paint(
        self,
        image: PIL.Image.Image,
        mask: numpy.ndarray,
        prompt: str,
        mode: typing.Literal["replace", "restyle", "relight", "insert", "background"],
        feathering: int,
        upscaler: meegeren.upscaler.Upscaler | None,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        with meegeren.telemetry.tracer.start_as_current_span("gpt_image.paint"):
            return self.edit(
                image,
                mask,
                GPT_IMAGE_PAINT_PROMPTS[mode].format(prompt=prompt),
                # The whole image redrawn would lose what keep names, which only /v1/remove pastes back
                False,
                feathering,
                upscaler,
                cancelled,
            )

    def edit(
        self,
        image: PIL.Image.Image,
        mask: numpy.ndarray,
        prompt: str,
        whole_image: bool,
        feathering: int,
        upscaler: meegeren.upscaler.Upscaler | None,
        cancelled: threading.Event,
    ) -> PIL.Image.Image:
        if whole_image:
            box, crop, crop_mask = (0, 0, image.width, image.height), image, mask
        else:
            box, crop, crop_mask = meegeren.inpainter.crop(image, mask)

        # Crops beyond the API's aspect ratio limit are sent stretched, and the result is resized back
        ratio = min(
            max(crop.width / crop.height, 1 / GPT_IMAGE_MAX_ASPECT_RATIO),
            GPT_IMAGE_MAX_ASPECT_RATIO,
        )
        short = (
            round(
                math.sqrt(GPT_IMAGE_PIXELS / max(ratio, 1 / ratio))
                / GPT_IMAGE_SIZE_MODULO
            )
            * GPT_IMAGE_SIZE_MODULO
        )
        long = min(
            round(short * max(ratio, 1 / ratio) / GPT_IMAGE_SIZE_MODULO)
            * GPT_IMAGE_SIZE_MODULO,
            short * GPT_IMAGE_MAX_ASPECT_RATIO,
        )
        width, height = (long, short) if ratio >= 1 else (short, long)
        source = io.BytesIO()
        crop.resize((width, height), PIL.Image.Resampling.LANCZOS).save(
            source, format="PNG"
        )
        resized_mask = (
            numpy.asarray(
                PIL.Image.fromarray(crop_mask.astype(numpy.uint8) * 255).resize(
                    (width, height), PIL.Image.Resampling.BILINEAR
                )
            )
            > 0
        )
        # The API edits where the mask is fully transparent
        transparent = PIL.Image.new("RGBA", (width, height))
        transparent.putalpha(
            PIL.Image.fromarray(numpy.where(resized_mask, 0, 255).astype(numpy.uint8))
        )
        alpha = io.BytesIO()
        transparent.save(alpha, format="PNG")
        response = self.client.images.edit(
            image=("image.png", source.getvalue(), "image/png"),
            mask=("mask.png", alpha.getvalue(), "image/png"),
            prompt=prompt,
            model=self.model,
            quality=self.quality,
            size=f"{width}x{height}",
        )
        filled = PIL.Image.open(
            io.BytesIO(base64.b64decode(response.data[0].b64_json))
        ).convert("RGB")
        filled = meegeren.inpainter.enlarge(filled, box, upscaler, cancelled)
        if whole_image:
            return filled.resize(image.size, PIL.Image.Resampling.LANCZOS)
        # The mask only guides the model, so pixels outside it are composited back from the original
        return meegeren.inpainter.paste_back(image, box, filled, crop_mask, feathering)
