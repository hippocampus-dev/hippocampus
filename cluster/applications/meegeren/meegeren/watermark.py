import typing

import PIL.Image
import PIL.ImageDraw
import PIL.ImageFont

import meegeren.telemetry

WATERMARK_SIZE_RATIO = 1 / 32
WATERMARK_BACKGROUND = (0, 0, 0, 128)
WATERMARK_FOREGROUND = (255, 255, 255, 255)

Position = typing.Literal["top-left", "top-right", "bottom-left", "bottom-right"]


def mark(
    image: PIL.Image.Image,
    font: PIL.ImageFont.FreeTypeFont,
    text: str,
    position: Position,
) -> PIL.Image.Image:
    with meegeren.telemetry.tracer.start_as_current_span("watermark.mark"):
        size = max(1, round(min(image.size) * WATERMARK_SIZE_RATIO))
        margin = size // 2
        measure = PIL.ImageDraw.Draw(PIL.Image.new("RGBA", (1, 1)))
        left, _, right, _ = measure.multiline_textbbox(
            (0, 0), text, font=font.font_variant(size=size)
        )
        size = max(
            1, min(size, int(size * (image.width - 4 * margin) / max(1, right - left)))
        )
        font = font.font_variant(size=size)
        left, top, right, bottom = measure.multiline_textbbox((0, 0), text, font=font)
        padding = size // 4
        band = PIL.Image.new(
            "RGBA",
            (right - left + 2 * padding, bottom - top + 2 * padding),
            WATERMARK_BACKGROUND,
        )
        PIL.ImageDraw.Draw(band).text(
            (padding - left, padding - top), text, font=font, fill=WATERMARK_FOREGROUND
        )
        vertical, horizontal = position.split("-")
        x = margin if horizontal == "left" else image.width - margin - band.width
        y = margin if vertical == "top" else image.height - margin - band.height
        marked = image.copy()
        marked.paste(band, (x, y), band)
        return marked
