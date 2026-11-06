import abc
import threading

import numpy
import PIL.Image

CORRECTION_DESCRIPTIONS = {
    "exposure": "brightening of its exposure",
    "white-balance": "removal of a color cast",
}

# https://www.itu.int/rec/R-REC-BT.709
LUMINANCE_WEIGHTS = numpy.array([0.2126, 0.7152, 0.0722])
NOISE_ESTIMATION_SIZE = 1024
# Lifting the shadows of Wikimedia Commons interiors noisier than this washed them out
MAX_NOISE = 3.0
# Brightened pixels kept their full saturation turned orange walls garish, and lost to 0.55 of it turned them salmon
MIN_SATURATION = 0.8
STRENGTH_KNOTS = [0.0, 0.5, 1.0]
# Stronger than 0.5 turned a lamp-lit wall cyan and washed out noisy shadows of Wikimedia Commons interiors
HIGHLIGHT_PERCENTILES = [99.5, 98, 95]
TARGET_LUMINANCES = [0.18, 0.24, 0.24]
MAX_EXPOSURE_GAINS = [2.5, 4.0, 4.0]
MAX_SHADOW_LIFT_STOPS = [1.0, 1.5, 1.5]
# Gray-world at full strength turned beige woodchip wallpaper white
WHITE_BALANCE_STRENGTHS = [0.5, 0.75, 1.0]
MAX_WHITE_BALANCE_GAIN_RATIOS = [1.2, 1.35, 1.5]


def srgb_to_linear(rgb: numpy.ndarray) -> numpy.ndarray:
    # https://www.color.org/chardata/rgb/srgb.xalter
    return numpy.where(rgb <= 0.04045, rgb / 12.92, ((rgb + 0.055) / 1.055) ** 2.4)


def linear_to_srgb(linear: numpy.ndarray) -> numpy.ndarray:
    linear = numpy.clip(linear, 0, 1)
    return numpy.where(
        linear <= 0.0031308, 12.92 * linear, 1.055 * linear ** (1 / 2.4) - 0.055
    )


def box_mean(x: numpy.ndarray, radius: int) -> numpy.ndarray:
    padded = (
        numpy.pad(numpy.pad(x, radius, mode="reflect"), ((1, 0), (1, 0)))
        .cumsum(0)
        .cumsum(1)
    )
    w = 2 * radius + 1
    return (
        padded[w:, w:] - padded[:-w, w:] - padded[w:, :-w] + padded[:-w, :-w]
    ) / w**2


def lift_exposure(image: PIL.Image.Image, strength: float) -> PIL.Image.Image:
    small = image.copy()
    small.thumbnail((NOISE_ESTIMATION_SIZE, NOISE_ESTIMATION_SIZE))
    y = numpy.asarray(small.convert("L")).astype(numpy.float64)
    laplacian = (
        y[:-2, :-2]
        - 2 * y[:-2, 1:-1]
        + y[:-2, 2:]
        - 2 * y[1:-1, :-2]
        + 4 * y[1:-1, 1:-1]
        - 2 * y[1:-1, 2:]
        + y[2:, :-2]
        - 2 * y[2:, 1:-1]
        + y[2:, 2:]
    )
    # https://doi.org/10.1006/cviu.1996.0060
    if numpy.sqrt(numpy.pi / 2) * numpy.abs(laplacian).mean() / 6 >= MAX_NOISE:
        return image

    epsilon = 1e-6
    linear = srgb_to_linear(numpy.asarray(image).astype(numpy.float64) / 255)
    luminance = linear @ LUMINANCE_WEIGHTS + epsilon
    chroma = linear / luminance[..., None]
    p5, p50, p98, highlight, p999 = numpy.percentile(
        luminance,
        [
            5,
            50,
            98,
            numpy.interp(strength, STRENGTH_KNOTS, HIGHLIGHT_PERCENTILES),
            99.9,
        ],
    )
    target_luminance = numpy.interp(strength, STRENGTH_KNOTS, TARGET_LUMINANCES)
    target = numpy.clip(
        target_luminance + 0.10 * numpy.tanh((target_luminance - p50) / 0.08),
        target_luminance - 0.05,
        target_luminance + 0.07,
    )
    gain = numpy.clip(
        target / (p50 + epsilon),
        1.0,
        max(
            min(
                numpy.interp(strength, STRENGTH_KNOTS, MAX_EXPOSURE_GAINS),
                0.92 / (highlight + epsilon),
            ),
            1.0,
        ),
    )
    dynamic_range = numpy.log2((p98 + 1e-4) / (p5 + 1e-4))
    shadow_lift = numpy.clip(
        numpy.log2(max(target / (p50 + epsilon), 1.0))
        * numpy.clip((dynamic_range - 4.0) / 4.0, 0.25, 1.0),
        0.0,
        numpy.interp(strength, STRENGTH_KNOTS, MAX_SHADOW_LIFT_STOPS),
    )
    if gain <= 1.02 and shadow_lift <= 0.02:
        return image

    # https://doi.org/10.1109/TPAMI.2012.213
    radius = max(4, int(0.02 * max(luminance.shape)))
    logarithm = numpy.log2(luminance)
    mean = box_mean(logarithm, radius)
    variance = numpy.maximum(0.0, box_mean(logarithm * logarithm, radius) - mean**2)
    weight = variance / (variance + 0.16)
    base = box_mean(weight, radius) * logarithm + box_mean(
        (1.0 - weight) * mean, radius
    )
    base_luminance = numpy.clip(2.0**base, 0.0, 1.0)
    u = numpy.clip((base_luminance - 0.04) / 0.51, 0.0, 1.0)
    shadow_weight = (1.0 - (3.0 * u * u - 2.0 * u**3)) ** 2
    lifted = (
        2.0
        ** (
            base
            + shadow_lift * shadow_weight * (1.0 - base_luminance)
            + (logarithm - base)
        )
        * gain
    )
    knee = 0.65
    white = max(1.15, gain * p999, numpy.percentile(lifted, 99.9))
    rolled = numpy.where(
        lifted <= knee,
        lifted,
        knee
        + (1.0 - knee)
        * (lifted - knee)
        / (
            (1.0 - knee)
            + (lifted - knee) * ((1.0 - knee) / max(white - knee, 0.1))
            + epsilon
        ),
    )
    peak = chroma.max(axis=-1)
    saturation = numpy.minimum(
        numpy.clip(
            numpy.maximum(rolled / luminance, 1.0) ** -0.15, MIN_SATURATION, 1.0
        ),
        numpy.where(
            peak > 1,
            numpy.clip(
                (1 - rolled) / numpy.maximum(rolled * (peak - 1), epsilon), 0, 1
            ),
            1,
        ),
    )[..., None]
    out = rolled[..., None] * (1.0 + saturation * (chroma - 1.0))
    out = out / numpy.maximum(out.max(axis=-1, keepdims=True), 1.0)
    return PIL.Image.fromarray((linear_to_srgb(out) * 255).round().astype(numpy.uint8))


def balance_white(image: PIL.Image.Image, strength: float) -> PIL.Image.Image:
    x = numpy.asarray(image).astype(numpy.float64) / 255
    brightness = x.mean(axis=2)
    neutral = (
        (brightness > 0.35)
        & (brightness < 0.95)
        & ((x.max(axis=2) - x.min(axis=2)) < 0.35 * x.max(axis=2))
    )
    if neutral.mean() < 0.05:
        return image
    linear = x**2.2
    means = linear[neutral].mean(axis=0)
    gains = numpy.log(means.mean() / means) * numpy.interp(
        strength, STRENGTH_KNOTS, WHITE_BALANCE_STRENGTHS
    )
    max_spread = numpy.log(
        numpy.interp(strength, STRENGTH_KNOTS, MAX_WHITE_BALANCE_GAIN_RATIOS)
    )
    spread = gains.max() - gains.min()
    if spread > max_spread:
        gains *= max_spread / spread
    return PIL.Image.fromarray(
        (numpy.clip(linear * numpy.exp(gains), 0, 1) ** (1 / 2.2) * 255)
        .round()
        .astype(numpy.uint8)
    )


CORRECTIONS = {"exposure": lift_exposure, "white-balance": balance_white}


class Enhancer(abc.ABC):
    reproduction_instructions: str
    need_instructions: dict[str, str]
    prefer_instructions: str
    harm_instructions: str
    reproduction_threshold: float
    need_threshold: float
    prefer_threshold: float
    harm_threshold: float
    strength: float

    @abc.abstractmethod
    def ask(self, images: list[PIL.Image.Image], instructions: str) -> float | None:
        # The probability that instructions hold for images, or None where the model refuses to answer
        raise NotImplementedError

    def enhance(
        self,
        image: PIL.Image.Image,
        strength: float | None,
        cancelled: threading.Event,
    ) -> tuple[PIL.Image.Image, list[str]]:
        # The image with the corrections taken, and their names in the order taken
        # strength overrides the configured one unless None
        enhanced = image
        applied = []
        reproduction_checked = False
        for name, correct in CORRECTIONS.items():
            if cancelled.is_set():
                return image, []
            need = self.ask([enhanced], self.need_instructions[name])
            if need is None or need < self.need_threshold:
                continue
            # Asked before the first correction, as the harm question let sepia prints turned grey through
            if not reproduction_checked:
                reproduction = self.ask([image], self.reproduction_instructions)
                if reproduction is None or reproduction >= self.reproduction_threshold:
                    return image, []
                reproduction_checked = True
            if cancelled.is_set():
                return image, []
            candidate = correct(
                enhanced, self.strength if strength is None else strength
            )
            if candidate is enhanced:
                continue
            description = CORRECTION_DESCRIPTIONS[name]
            prefer = self.ask(
                [enhanced, candidate],
                self.prefer_instructions.format(description=description),
            )
            if prefer is None or prefer < self.prefer_threshold:
                continue
            harm = self.ask(
                [enhanced, candidate],
                self.harm_instructions.format(description=description),
            )
            if harm is None or harm >= self.harm_threshold:
                continue
            enhanced = candidate
            applied.append(name)
        return enhanced, applied
