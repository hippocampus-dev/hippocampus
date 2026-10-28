import asyncio
import base64
import collections.abc
import concurrent.futures
import contextlib
import dataclasses
import datetime
import http
import io
import json
import logging
import os
import threading
import time
import typing

import fastapi
import numpy
import opentelemetry.context
import opentelemetry.exporter.otlp.proto.grpc.trace_exporter
import opentelemetry.exporter.prometheus
import opentelemetry.instrumentation.fastapi
import opentelemetry.metrics
import opentelemetry.sdk.metrics
import opentelemetry.sdk.resources
import opentelemetry.sdk.trace.export
import opentelemetry.trace
import PIL
import PIL.Image
import PIL.ImageFont
import PIL.ImageOps
import prometheus_client
import pydantic
import pythonjsonlogger.jsonlogger
import torch
import torch.nn.functional
import yaml

import meegeren.cancellation
import meegeren.harmonizer
import meegeren.harmonizer.ic_light
import meegeren.harmonizer.pct_net
import meegeren.imaging
import meegeren.inpainter
import meegeren.inpainter.gpt_image
import meegeren.inpainter.lama
import meegeren.inpainter.osor
import meegeren.inpainter.qwen_image_edit
import meegeren.matting
import meegeren.matting.vitmatte
import meegeren.model
import meegeren.object_locator
import meegeren.object_locator.grounding_dino_sam2
import meegeren.object_locator.sam3
import meegeren.refiner
import meegeren.refiner.sdxl_inpainting
import meegeren.settings
import meegeren.telemetry
import meegeren.text_locator
import meegeren.text_locator.craft
import meegeren.upscaler
import meegeren.upscaler.real_esrgan
import meegeren.upscaler.swinir
import meegeren.watermark

s = meegeren.settings.Settings()

# Inference saturates every core through torch's intra-op threads
executor = concurrent.futures.ThreadPoolExecutor(max_workers=1)
# Held through a whole request, so no other request's job runs between its executor jobs
lock = asyncio.Lock()


@dataclasses.dataclass(frozen=True)
class PipelineModels:
    object_locator: meegeren.object_locator.Locator
    text_locator: meegeren.text_locator.TextLocator
    inpainter: meegeren.inpainter.Inpainter
    refiner: meegeren.refiner.Refiner | None
    matting: meegeren.matting.Matting | None
    harmonizers: list[meegeren.harmonizer.Harmonizer]
    upscaler: meegeren.upscaler.Upscaler | None
    pipeline: meegeren.model.Pipeline


@dataclasses.dataclass(frozen=True)
class Models:
    pipeline: PipelineModels | None
    upscaler: meegeren.upscaler.Upscaler | None
    font: PIL.ImageFont.FreeTypeFont


global_models: Models | None = None


async def startup():
    class JsonFormatter(pythonjsonlogger.jsonlogger.JsonFormatter):
        def __init__(self, *args, **kwargs):
            # https://opentelemetry.io/docs/specs/otel/logs/data-model/
            super().__init__(*args, **kwargs, rename_fields={"message": "body"})

        def add_fields(
            self,
            log_record: dict[str, typing.Any],
            record: logging.LogRecord,
            message_dict: dict[str, typing.Any],
        ):
            now = datetime.datetime.now()
            log_record["name"] = record.name
            log_record["time"] = now.isoformat()
            log_record["severitytext"] = record.levelname

            super().add_fields(log_record, record, message_dict)

    handler = logging.StreamHandler()
    handler.setFormatter(JsonFormatter())
    if s.is_debug():
        handler.setFormatter(
            logging.Formatter(
                "time=%(asctime)s name=%(name)s severitytext=%(levelname)s body=%(message)s traceid=%(traceid)s spanid=%(spanid)s",
                defaults={"traceid": "", "spanid": ""},
            )
        )
    logging.basicConfig(level=s.convert_log_level(), handlers=[handler])
    provider = opentelemetry.sdk.trace.TracerProvider(
        resource=opentelemetry.sdk.resources.OTELResourceDetector().detect(),
    )
    processor = opentelemetry.sdk.trace.export.BatchSpanProcessor(
        opentelemetry.exporter.otlp.proto.grpc.trace_exporter.OTLPSpanExporter(),
    )
    provider.add_span_processor(processor)
    opentelemetry.trace.set_tracer_provider(provider)

    opentelemetry.metrics.set_meter_provider(
        opentelemetry.sdk.metrics.MeterProvider(
            metric_readers=[opentelemetry.exporter.prometheus.PrometheusMetricReader()],
        )
    )

    if s.num_threads is not None:
        torch.set_num_threads(s.num_threads)
    # SAM's image encoder on CPU ran ten times slower when its activations underflowed to denormal floats
    torch.set_flush_denormal(True)

    def load():
        with open(s.config_file) as f:
            configuration = meegeren.model.Configuration.model_validate(
                yaml.safe_load(f)
            )

        def select[T: meegeren.model.Pipeline | meegeren.model.UpscalerConfiguration](
            kind: str, entries: list[T], name: str | None
        ) -> T | None:
            if not name:
                return None
            for entry in entries:
                if entry.name == name:
                    return entry
            raise ValueError(f"{kind} {name} is not in {s.config_file}")

        pipeline = select("pipeline", configuration.pipelines, s.pipeline)
        upscaler_configuration = select("upscaler", configuration.upscalers, s.upscaler)
        if pipeline is None and upscaler_configuration is None:
            raise ValueError("PIPELINE or UPSCALER must be set")
        directory = os.path.expanduser(s.model_directory)

        # An upscaler both UPSCALER and the pipeline name is loaded once
        upscalers: dict[str, meegeren.upscaler.Upscaler] = {}

        def load_upscaler(
            c: meegeren.model.UpscalerConfiguration | None,
        ) -> meegeren.upscaler.Upscaler | None:
            if c is None:
                return None
            if c.name not in upscalers:
                match c:
                    case meegeren.model.SwinirConfiguration():
                        upscalers[c.name] = (
                            meegeren.upscaler.swinir.SwinirUpscaler.from_file(
                                path=os.path.join(directory, c.model_path),
                                keep_on_device=c.keep_on_device,
                                device=s.device,
                            )
                        )
                    case meegeren.model.RealEsrganConfiguration():
                        upscalers[c.name] = (
                            meegeren.upscaler.real_esrgan.RealEsrganUpscaler.from_file(
                                path=os.path.join(directory, c.model_path),
                                weak_denoise_path=os.path.join(
                                    directory, c.weak_denoise_model_path
                                ),
                                denoise_strength=c.denoise_strength,
                                dtype=getattr(torch, c.dtype),
                                keep_on_device=c.keep_on_device,
                                device=s.device,
                            )
                        )
            return upscalers[c.name]

        pipeline_models = None
        if pipeline is not None:
            match pipeline.object_locator:
                case meegeren.model.GroundingDinoSam2Configuration() as c:
                    object_locator = meegeren.object_locator.grounding_dino_sam2.GroundingDinoSam2Locator.from_pretrained(
                        grounding_model=os.path.join(directory, c.grounding_model),
                        segmentation_model=os.path.join(
                            directory, c.segmentation_model
                        ),
                        deformable_attention_path=os.path.join(
                            directory, c.deformable_attention_path
                        ),
                        dtype=getattr(torch, c.dtype),
                        keep_on_device=c.keep_on_device,
                        device=s.device,
                    )
                case meegeren.model.Sam3Configuration() as c:
                    object_locator = (
                        meegeren.object_locator.sam3.Sam3Locator.from_pretrained(
                            model=os.path.join(directory, c.model),
                            mask_threshold=c.mask_threshold,
                            dtype=getattr(torch, c.dtype),
                            keep_on_device=c.keep_on_device,
                            device=s.device,
                        )
                    )
            match pipeline.inpainter:
                case meegeren.model.BigLamaConfiguration() as c:
                    inpainter = meegeren.inpainter.lama.LamaInpainter.from_file(
                        path=os.path.join(directory, c.model_path),
                        keep_on_device=c.keep_on_device,
                        device=s.device,
                    )
                case meegeren.model.OsorConfiguration() as c:
                    inpainter = meegeren.inpainter.osor.OsorInpainter.from_pretrained(
                        base_model=os.path.join(directory, c.base_model),
                        weights_path=os.path.join(directory, c.weights_path),
                        dtype=getattr(torch, c.dtype),
                        keep_on_device=c.keep_on_device,
                        device=s.device,
                    )
                case meegeren.model.GptImageConfiguration() as c:
                    inpainter = (
                        meegeren.inpainter.gpt_image.GptImageInpainter.from_environment(
                            model=c.model,
                            quality=c.quality,
                            whole_image=pipeline.remove.whole_image,
                        )
                    )
                case meegeren.model.QwenImageEditConfiguration() as c:
                    inpainter = meegeren.inpainter.qwen_image_edit.QwenImageEditInpainter.from_pretrained(
                        model=os.path.join(directory, c.model),
                        transformer_path=os.path.join(directory, c.transformer_path),
                        lightning_path=os.path.join(directory, c.lightning_path),
                        remover_path=os.path.join(directory, c.remover_path),
                        steps=c.steps,
                        whole_image=pipeline.remove.whole_image,
                        dtype=getattr(torch, c.dtype),
                        keep_text_encoder_on_device=c.text_encoder.keep_on_device,
                        keep_transformer_on_device=c.transformer.keep_on_device,
                        keep_vae_on_device=c.vae.keep_on_device,
                        device=s.device,
                    )
            refiner = None
            if pipeline.refiner is not None:
                refiner = meegeren.refiner.sdxl_inpainting.SdxlInpaintingRefiner.from_pretrained(
                    base_model=os.path.join(directory, pipeline.refiner.base_model),
                    loras=[
                        (os.path.join(directory, lora.path), lora.weight)
                        for lora in pipeline.refiner.loras
                    ],
                    timestep_spacing=pipeline.refiner.timestep_spacing,
                    dtype=getattr(torch, pipeline.refiner.dtype),
                    keep_on_device=pipeline.refiner.keep_on_device,
                    device=s.device,
                )
            matting = None
            if pipeline.matting is not None:
                matting = meegeren.matting.vitmatte.VitMatteMatting.from_pretrained(
                    model=os.path.join(directory, pipeline.matting.model),
                    trimap_width=pipeline.matting.trimap_width,
                    dtype=getattr(torch, pipeline.matting.dtype),
                    keep_on_device=pipeline.matting.keep_on_device,
                    device=s.device,
                )
            harmonizers = []
            for c in pipeline.harmonizers:
                match c:
                    case meegeren.model.PctNetConfiguration():
                        harmonizers.append(
                            meegeren.harmonizer.pct_net.PctNetHarmonizer.from_file(
                                path=os.path.join(directory, c.model_path),
                                dtype=getattr(torch, c.dtype),
                                keep_on_device=c.keep_on_device,
                                device=s.device,
                            )
                        )
                    case meegeren.model.IcLightConfiguration():
                        harmonizers.append(
                            meegeren.harmonizer.ic_light.IcLightHarmonizer.from_pretrained(
                                base_model=os.path.join(directory, c.base_model),
                                weights_path=os.path.join(directory, c.weights_path),
                                steps=c.steps,
                                guidance_scale=c.guidance_scale,
                                added_prompt=c.added_prompt,
                                negative_prompt=c.negative_prompt,
                                highres_scale=c.highres_scale,
                                highres_strength=c.highres_strength,
                                dtype=getattr(torch, c.dtype),
                                keep_on_device=c.keep_on_device,
                                device=s.device,
                            )
                        )
            pipeline_models = PipelineModels(
                object_locator=object_locator,
                text_locator=meegeren.text_locator.craft.CraftTextLocator.from_file(
                    path=os.path.join(directory, pipeline.text_locator.model_path),
                    region_threshold=pipeline.text_locator.region_threshold,
                    affinity_threshold=pipeline.text_locator.affinity_threshold,
                    dtype=getattr(torch, pipeline.text_locator.dtype),
                    keep_on_device=pipeline.text_locator.keep_on_device,
                    device=s.device,
                ),
                inpainter=inpainter,
                refiner=refiner,
                matting=matting,
                harmonizers=harmonizers,
                upscaler=load_upscaler(
                    select("upscaler", configuration.upscalers, pipeline.upscaler)
                ),
                pipeline=pipeline,
            )
        return Models(
            pipeline=pipeline_models,
            upscaler=load_upscaler(upscaler_configuration),
            font=PIL.ImageFont.truetype(os.path.join(directory, s.watermark_font_path)),
        )

    global global_models
    global_models = await asyncio.get_running_loop().run_in_executor(executor, load)


@contextlib.asynccontextmanager
async def lifespan(_app: fastapi.FastAPI):
    await startup()
    yield
    executor.shutdown(wait=False, cancel_futures=True)


app = fastapi.FastAPI(lifespan=lifespan)
opentelemetry.instrumentation.fastapi.FastAPIInstrumentor.instrument_app(app)


@app.middleware("http")
async def override_server_error_middleware(request: fastapi.Request, call_next):
    try:
        return await call_next(request)
    except Exception as e:
        meegeren.telemetry.logger.error(e, exc_info=True)
        return fastapi.responses.PlainTextResponse(
            http.HTTPStatus.INTERNAL_SERVER_ERROR.phrase,
            status_code=http.HTTPStatus.INTERNAL_SERVER_ERROR,
        )


def get_models() -> Models:
    if global_models is None:
        raise fastapi.HTTPException(http.HTTPStatus.SERVICE_UNAVAILABLE)
    return global_models


# FastAPI validates a dependant's form fields only after its dependencies, so the lock is taken later by what this yields
async def hold_lock(
    request: fastapi.Request,
) -> collections.abc.AsyncIterator[
    collections.abc.Callable[[], collections.abc.Awaitable[None]]
]:
    held = False

    async def acquire() -> None:
        nonlocal held
        async with meegeren.cancellation.cancel_on_disconnect(request):
            await lock.acquire()
        held = True

    try:
        yield acquire
    finally:
        if held:
            lock.release()


def get_pipeline_models(
    models: Models = fastapi.Depends(get_models),
) -> PipelineModels:
    if models.pipeline is None:
        raise fastapi.HTTPException(
            http.HTTPStatus.NOT_IMPLEMENTED, "No pipeline is loaded"
        )
    return models.pipeline


async def read_image(upload: fastapi.UploadFile, mode: str) -> PIL.Image.Image:
    data = await upload.read(s.max_upload_size + 1)
    if len(data) > s.max_upload_size:
        raise fastapi.HTTPException(
            http.HTTPStatus.REQUEST_ENTITY_TOO_LARGE,
            f"{upload.filename} exceeds {s.max_upload_size} bytes",
        )
    try:
        image = PIL.Image.open(io.BytesIO(data))
        return PIL.ImageOps.exif_transpose(image).convert(mode)
    except (OSError, PIL.Image.DecompressionBombError) as e:
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST, f"{upload.filename} is not a valid image"
        ) from e


async def read_mask(
    upload: fastapi.UploadFile, source: PIL.Image.Image
) -> numpy.ndarray:
    uploaded = await read_image(upload, "L")
    if uploaded.size != source.size:
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST,
            f"mask size {uploaded.size} does not match image size {source.size}",
        )
    return numpy.asarray(uploaded) > 127


def override[T: pydantic.BaseModel](
    stage: str, configuration: T | None, prefix: str, **values: typing.Any
) -> T | None:
    given = {name: value for name, value in values.items() if value is not None}
    if len(given) == 0:
        return configuration
    if configuration is None:
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST,
            f"No {stage} in the pipeline for {', '.join(prefix + name for name in given)}",
        )
    try:
        # Unset fields stay unset, as RemoveConfiguration rejects max_removals set without redetect
        return type(configuration).model_validate(
            configuration.model_dump(exclude_unset=True) | given
        )
    except pydantic.ValidationError as e:
        raise fastapi.exceptions.RequestValidationError(
            [
                {**error, "loc": ("body", *(prefix + str(loc) for loc in error["loc"]))}
                for error in e.errors(
                    include_url=False, include_context=False, include_input=False
                )
            ]
        ) from e


def get_object_locator_configuration(
    threshold: float | None = fastapi.Form(None),
    prompt: str | None = fastapi.Form(None),
    keep: str | None = fastapi.Form(None),
    models: PipelineModels = fastapi.Depends(get_pipeline_models),
) -> meegeren.model.ObjectLocatorConfiguration:
    labels = meegeren.object_locator.parse_labels(
        prompt or ""
    ) + meegeren.object_locator.parse_labels(keep or "")
    if threshold is not None and len(labels) == 0:
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST, "threshold applies only with prompt or keep"
        )
    return override(
        "object_locator", models.pipeline.object_locator, "", threshold=threshold
    )


def get_refiner_configuration(
    refiner_prompt: str | None = fastapi.Form(None),
    refiner_negative_prompt: str | None = fastapi.Form(None),
    refiner_guidance_scale: float | None = fastapi.Form(None),
    refiner_strength: float | None = fastapi.Form(None),
    refiner_steps: int | None = fastapi.Form(None),
    models: PipelineModels = fastapi.Depends(get_pipeline_models),
) -> meegeren.model.SdxlInpaintingConfiguration | None:
    refiner = override(
        "refiner",
        models.pipeline.refiner,
        "refiner_",
        prompt=refiner_prompt,
        negative_prompt=refiner_negative_prompt,
        guidance_scale=refiner_guidance_scale,
        strength=refiner_strength,
        steps=refiner_steps,
    )
    # https://github.com/huggingface/diffusers/blob/v0.41.0/src/diffusers/pipelines/stable_diffusion_xl/pipeline_stable_diffusion_xl_inpaint.py
    if (
        refiner is not None
        and refiner_negative_prompt is not None
        and refiner.guidance_scale <= 1
    ):
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST,
            "refiner_negative_prompt applies only with refiner_guidance_scale above 1",
        )
    return refiner


def get_remove_configuration(
    remove_additions: bool | None = fastapi.Form(None),
    models: PipelineModels = fastapi.Depends(get_pipeline_models),
) -> meegeren.model.RemoveConfiguration:
    if remove_additions is not None and models.pipeline.refiner is None:
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST, "remove_additions applies only with refiner"
        )
    return override(
        "remove", models.pipeline.remove, "", remove_additions=remove_additions
    )


def get_feathering(
    feathering: int | None = fastapi.Form(None, ge=0),
    models: PipelineModels = fastapi.Depends(get_pipeline_models),
) -> int:
    if feathering is not None and not models.inpainter.pastes_back:
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST,
            "feathering applies only with an inpainter pasting back what it fills",
        )
    return s.mask_feathering if feathering is None else feathering


@dataclasses.dataclass(frozen=True)
class Target:
    image: PIL.Image.Image
    # (height, width) each, for text and the uploaded mask
    masks: list[numpy.ndarray]
    # (height, width) each, one per object the labels name
    objects: list[numpy.ndarray]
    labels: list[str]
    dilation: int
    # (height, width), for the objects keep names
    kept: numpy.ndarray


def mostly_outside_keep(
    objects: list[numpy.ndarray], kept: numpy.ndarray
) -> list[numpy.ndarray]:
    # An object mostly within what keep names is that object matched by prompt as well
    return [m for m in objects if m.any() and (m & kept).sum() * 2 < m.sum()]


async def get_target(
    request: fastapi.Request,
    image: fastapi.UploadFile,
    prompt: str | None = fastapi.Form(None),
    text: bool = fastapi.Form(False),
    mask: fastapi.UploadFile | None = None,
    dilation: int | None = fastapi.Form(None, ge=0),
    keep: str | None = fastapi.Form(None),
    restyle: bool = fastapi.Form(False),
    object_locator: meegeren.model.ObjectLocatorConfiguration = fastapi.Depends(
        get_object_locator_configuration
    ),
    models: PipelineModels = fastapi.Depends(get_pipeline_models),
    acquire_lock: collections.abc.Callable[
        [], collections.abc.Awaitable[None]
    ] = fastapi.Depends(hold_lock),
) -> Target:
    labels = meegeren.object_locator.parse_labels(prompt or "")
    if len(labels) == 0 and not text and mask is None:
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST, "One of prompt, text or mask is required"
        )
    if restyle and request.url.path not in ("/v1/replace", "/v1/masks"):
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST,
            "restyle applies only to /v1/replace and /v1/masks",
        )
    # qwen-image-edit is told the targets by name, as a highlight would hide the shape and shading it keeps
    if restyle and (text or mask is not None):
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST, "restyle takes prompt alone as its targets"
        )

    source = await read_image(image, "RGB")
    uploaded_dilation = s.uploaded_mask_dilation if dilation is None else dilation
    if dilation is None:
        dilation = (
            round(max(source.size) * s.restyle_mask_dilation_ratio)
            if restyle
            else s.mask_dilation
        )

    masks = []
    if mask is not None:
        masks.append((await read_mask(mask, source), uploaded_dilation))
    await acquire_lock()
    if text:

        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            return models.text_locator.locate(source)

        async with meegeren.cancellation.cancel_on_disconnect(request):
            masks.append(
                (
                    await asyncio.get_running_loop().run_in_executor(
                        executor, run_in_context, opentelemetry.context.get_current()
                    ),
                    dilation,
                )
            )
    objects = []
    kept = numpy.zeros((source.height, source.width), dtype=bool)
    keep_labels = meegeren.object_locator.parse_labels(keep or "")
    if len(labels) + len(keep_labels) > 0:

        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            # Located apart, Grounding DINO took dining tables and TV stands for the kitchen counter keep named
            return models.object_locator.locate(
                source, keep_labels + labels, object_locator.threshold
            )

        async with meegeren.cancellation.cancel_on_disconnect(request):
            located = await asyncio.get_running_loop().run_in_executor(
                executor, run_in_context, opentelemetry.context.get_current()
            )
        objects = [m for i, m in located if i >= len(keep_labels)]
        kept = numpy.logical_or.reduce(
            [kept] + [m for i, m in located if i < len(keep_labels)]
        )

    masks = [(m, d) for m, d in masks if (m & ~kept).any()]
    objects = mostly_outside_keep(objects, kept)
    if len(masks) == 0 and len(objects) == 0:
        raise fastapi.HTTPException(
            http.HTTPStatus.UNPROCESSABLE_ENTITY,
            "Nothing matched prompt or text outside keep",
        )

    def run_in_context(ctx: opentelemetry.context.Context):
        opentelemetry.context.attach(ctx)
        return (
            [meegeren.imaging.dilate(m, d) & ~kept for m, d in masks],
            [meegeren.imaging.dilate(m, dilation) & ~kept for m in objects],
        )

    async with meegeren.cancellation.cancel_on_disconnect(request):
        masks, objects = await asyncio.get_running_loop().run_in_executor(
            executor, run_in_context, opentelemetry.context.get_current()
        )
    return Target(
        image=source,
        masks=masks,
        objects=objects,
        labels=labels,
        dilation=dilation,
        kept=kept,
    )


def encode(image: PIL.Image.Image) -> bytes:
    buffer = io.BytesIO()
    image.save(buffer, format="PNG")
    return buffer.getvalue()


def png(image: PIL.Image.Image) -> fastapi.Response:
    return fastapi.Response(encode(image), media_type="image/png")


def get_watermark(
    watermark: str | None = fastapi.Form(None, max_length=200),
    watermark_position: meegeren.watermark.Position | None = fastapi.Form(None),
    models: Models = fastapi.Depends(get_models),
) -> typing.Callable[[PIL.Image.Image], PIL.Image.Image]:
    if watermark is None and watermark_position is not None:
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST,
            "watermark_position applies only with watermark",
        )

    def mark(image: PIL.Image.Image) -> PIL.Image.Image:
        if watermark is None:
            return image
        return meegeren.watermark.mark(
            image, models.font, watermark, watermark_position or s.watermark_position
        )

    return mark


def server_sent_event(data: dict[str, typing.Any]) -> str:
    return f"event: {data['type']}\ndata: {json.dumps(data)}\n\n"


@app.post("/v1/masks")
async def masks(target: Target = fastapi.Depends(get_target)) -> fastapi.Response:
    """Returns the mask `/v1/remove` would fill before OSOR narrows it, `redetect` locates objects again and `remove_additions` adds to it, as a PNG, or with `restyle=true` the mask `/v1/replace` restyles."""
    return png(
        PIL.Image.fromarray(
            numpy.logical_or.reduce(target.masks + target.objects).astype(numpy.uint8)
            * 255
        )
    )


@app.post("/v1/remove")
async def remove(
    request: fastapi.Request,
    stream: bool = fastapi.Form(False),
    # Resolved before target, so a value outside the refiner's constraints fails before locating anything
    refiner: meegeren.model.SdxlInpaintingConfiguration | None = fastapi.Depends(
        get_refiner_configuration
    ),
    remove_configuration: meegeren.model.RemoveConfiguration = fastapi.Depends(
        get_remove_configuration
    ),
    feathering: int = fastapi.Depends(get_feathering),
    target: Target = fastapi.Depends(get_target),
    object_locator: meegeren.model.ObjectLocatorConfiguration = fastapi.Depends(
        get_object_locator_configuration
    ),
    models: PipelineModels = fastapi.Depends(get_pipeline_models),
    mark: typing.Callable[[PIL.Image.Image], PIL.Image.Image] = fastapi.Depends(
        get_watermark
    ),
) -> fastapi.Response:
    """Returns the image with its targets filled in as a PNG, or streams it as server-sent events shaped like the OpenAI image edit stream with `stream=true`."""
    # A started inference is not interrupted by the disconnect, so it checks this as it runs
    cancelled = threading.Event()
    loop = asyncio.get_running_loop()
    partial_images: asyncio.Queue[PIL.Image.Image | None] = asyncio.Queue()

    def restore(image: PIL.Image.Image) -> PIL.Image.Image:
        # OSOR and the refiner composite by masks of their own, which may reach into what keep names, and inpainters given whole_image redraw it
        result = numpy.array(image)
        result[target.kept] = numpy.asarray(target.image)[target.kept]
        return PIL.Image.fromarray(result)

    def refine(
        source: PIL.Image.Image, image: PIL.Image.Image, removed: numpy.ndarray
    ) -> PIL.Image.Image:
        if stream:
            loop.call_soon_threadsafe(partial_images.put_nowait, image)
        refined = restore(
            models.refiner.refine(source, image, refiner, models.upscaler, cancelled)
        )
        if not remove_configuration.remove_additions or cancelled.is_set():
            return refined
        difference = torch.from_numpy(
            numpy.abs(
                numpy.asarray(refined).astype(numpy.int16)
                - numpy.asarray(image).astype(numpy.int16)
            ).max(axis=2)
        ).to(s.device, torch.float32)[None, None]
        longer = max(image.width, image.height)
        # avg_pool2d keeps the size only for an odd kernel
        window = (
            round(
                (
                    longer
                    * meegeren.refiner.sdxl_inpainting.SDXL_INPAINTING_ADDITION_WINDOW
                    - 1
                )
                / 2
            )
            * 2
            + 1
        )
        changed = (
            torch.nn.functional.avg_pool2d(
                difference,
                kernel_size=window,
                stride=1,
                padding=window // 2,
                count_include_pad=False,
            )
            > meegeren.refiner.sdxl_inpainting.SDXL_INPAINTING_ADDITION_THRESHOLD
        )
        # The letters SDXL-Inpainting draws over a logo OSOR leaves change it too little to count above
        changed |= torch.from_numpy(models.text_locator.locate(refined) & removed).to(
            s.device
        )[None, None]
        labels = torch.where(
            changed,
            torch.arange(
                1, changed.numel() + 1, device=s.device, dtype=torch.float64
            ).reshape(changed.shape),
            0,
        )
        while True:
            grown = torch.where(
                changed,
                torch.nn.functional.max_pool2d(
                    labels, kernel_size=3, stride=1, padding=1
                ),
                0,
            )
            if torch.equal(grown, labels):
                break
            labels = grown
        indices, areas = labels.unique(return_counts=True)
        additions = [
            meegeren.imaging.dilate(
                (labels == index)[0, 0].cpu().numpy(), target.dilation
            )
            & ~target.kept
            for index, area in zip(indices.tolist(), areas.tolist())
            if index > 0
            and area
            >= meegeren.refiner.sdxl_inpainting.SDXL_INPAINTING_ADDITION_MIN_AREA
            * longer**2
        ]
        return restore(
            models.inpainter.inpaint(
                refined, additions, feathering, models.upscaler, cancelled
            )
        )

    def run_in_context(ctx: opentelemetry.context.Context):
        opentelemetry.context.attach(ctx)
        image = target.image
        refined = None
        removed = numpy.logical_or.reduce(
            [numpy.zeros_like(target.kept)] + target.masks
        )
        if not remove_configuration.redetect:
            removed = numpy.logical_or.reduce([removed] + target.objects)
            image = restore(
                models.inpainter.inpaint(
                    image,
                    target.masks + target.objects,
                    feathering,
                    models.upscaler,
                    cancelled,
                )
            )
        else:
            if len(target.masks) > 0:
                image = restore(
                    models.inpainter.inpaint(
                        image, target.masks, feathering, models.upscaler, cancelled
                    )
                )
            objects = target.objects
            for i in range(remove_configuration.max_removals):
                if cancelled.is_set():
                    break
                if len(objects) > 0:
                    # An object overlapping a nearer one may show more of itself once that one is removed, so it waits to be located again
                    separate = []
                    nearer = numpy.zeros_like(target.kept)
                    for m in sorted(objects, key=meegeren.imaging.bottom, reverse=True):
                        if not (m & nearer).any():
                            separate.append(m)
                            removed |= m
                        nearer |= m
                    image = restore(
                        models.inpainter.inpaint(
                            image, separate, feathering, models.upscaler, cancelled
                        )
                    )
                elif (
                    models.refiner is None
                    or refined is not None
                    or len(target.labels) == 0
                ):
                    break
                else:
                    # The refiner redraws faint remnants the inpainter leaves as solid objects, which are then located again
                    image = refine(target.image, image, removed)
                    refined = image
                    if remove_configuration.max_refinements == 1:
                        break
                if cancelled.is_set() or i == remove_configuration.max_removals - 1:
                    break
                objects = [
                    meegeren.imaging.dilate(m, target.dilation) & ~target.kept
                    for m in mostly_outside_keep(
                        [
                            m
                            for _, m in models.object_locator.locate(
                                image, target.labels, object_locator.threshold
                            )
                        ],
                        target.kept,
                    )
                ]
        if models.refiner is None or image is refined or cancelled.is_set():
            return image
        return refine(target.image if refined is None else refined, image, removed)

    if not stream:
        try:
            async with meegeren.cancellation.cancel_on_disconnect(request):
                return png(
                    mark(
                        await loop.run_in_executor(
                            executor,
                            run_in_context,
                            opentelemetry.context.get_current(),
                        )
                    )
                )
        except asyncio.CancelledError:
            cancelled.set()
            raise

    # https://developers.openai.com/api/reference/python/resources/images/methods/edit
    async def events() -> collections.abc.AsyncIterator[str]:
        inference = loop.run_in_executor(
            executor, run_in_context, opentelemetry.context.get_current()
        )
        inference.add_done_callback(lambda _: partial_images.put_nowait(None))
        try:
            index = 0
            while (image := await partial_images.get()) is not None:
                yield server_sent_event(
                    {
                        "type": "image_edit.partial_image",
                        "b64_json": base64.b64encode(encode(mark(image))).decode(),
                        "created_at": int(time.time()),
                        "output_format": "png",
                        "partial_image_index": index,
                    }
                )
                index += 1
            yield server_sent_event(
                {
                    "type": "image_edit.completed",
                    "b64_json": base64.b64encode(
                        encode(mark(inference.result()))
                    ).decode(),
                    "created_at": int(time.time()),
                    "output_format": "png",
                }
            )
        except Exception as e:
            meegeren.telemetry.logger.error(e, exc_info=True)
            yield server_sent_event(
                {
                    "type": "error",
                    "error": {
                        "type": "server_error",
                        "message": http.HTTPStatus.INTERNAL_SERVER_ERROR.phrase,
                    },
                }
            )
        finally:
            if not inference.done():
                cancelled.set()
                inference.cancel()

    return fastapi.responses.StreamingResponse(events(), media_type="text/event-stream")


def get_painter(
    models: PipelineModels = fastapi.Depends(get_pipeline_models),
) -> meegeren.inpainter.Painter:
    if not isinstance(models.inpainter, meegeren.inpainter.Painter):
        raise fastapi.HTTPException(
            http.HTTPStatus.NOT_IMPLEMENTED, "The inpainter does not take prompts"
        )
    return models.inpainter


def get_relight(
    relight: str | None = fastapi.Form(None, min_length=1),
    restyle: bool = fastapi.Form(False),
    painter: meegeren.inpainter.Painter = fastapi.Depends(get_painter),
) -> str | None:
    if relight is not None and not restyle:
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST, "relight applies only with restyle=true"
        )
    if relight is not None and not painter.relights:
        raise fastapi.HTTPException(
            http.HTTPStatus.NOT_IMPLEMENTED, "The inpainter does not relight"
        )
    return relight


@app.post("/v1/replace")
async def replace(
    request: fastapi.Request,
    content: str = fastapi.Form(min_length=1),
    feathering: int = fastapi.Form(s.mask_feathering, ge=0),
    restyle: bool = fastapi.Form(False),
    # Resolved before target, so relight without restyle fails before locating anything
    relight: str | None = fastapi.Depends(get_relight),
    painter: meegeren.inpainter.Painter = fastapi.Depends(get_painter),
    target: Target = fastapi.Depends(get_target),
    models: PipelineModels = fastapi.Depends(get_pipeline_models),
    mark: typing.Callable[[PIL.Image.Image], PIL.Image.Image] = fastapi.Depends(
        get_watermark
    ),
) -> fastapi.Response:
    """Returns the image with the box around each target redrawn as `content` as a PNG, or with `restyle=true` each target itself redrawn in the material or color `content` names and with `relight` the rest relit as it describes."""
    cancelled = threading.Event()

    def run_in_context(ctx: opentelemetry.context.Context):
        opentelemetry.context.attach(ctx)
        image = target.image
        if restyle:
            restyled = numpy.logical_or.reduce(target.objects)
            # Every target keeps its own shape, so none is drawn over another
            image = painter.paint(
                image,
                restyled,
                f"{' and '.join(target.labels)} to {content}",
                "restyle",
                feathering,
                models.upscaler,
                cancelled,
            )
            if relight is None or cancelled.is_set():
                return image
            relit = painter.paint(
                image,
                numpy.ones_like(restyled),
                f"{relight} coming in through the {' and '.join(target.labels)}",
                "relight",
                0,
                None,
                cancelled,
            )
            if cancelled.is_set():
                return image
            radius = max(
                1, round(max(image.size) * meegeren.inpainter.RELIGHT_SHADING_RADIUS)
            )
            original, lit = (
                (torch.from_numpy(numpy.array(i)).permute(2, 0, 1)[None].float() / 255)
                ** 2.2
                for i in (image, relit)
            )
            blurred = []
            for x in (original, lit):
                for kernel, padding in (
                    ((2 * radius + 1, 1), (radius, 0)),
                    ((1, 2 * radius + 1), (0, radius)),
                ):
                    x = torch.nn.functional.avg_pool2d(
                        x,
                        kernel,
                        stride=1,
                        padding=padding,
                        count_include_pad=False,
                    )
                blurred.append(x)
            changed = (
                (
                    (original * (blurred[1] + 1e-3) / (blurred[0] + 1e-3)).clamp(0, 1)
                    ** (1 / 2.2)
                    * 255
                )
                .round()
                .to(torch.uint8)[0]
                .permute(1, 2, 0)
                .numpy()
            )
            result = numpy.array(image)
            room = meegeren.imaging.dilate(~restyled, target.dilation) & ~target.kept
            result[room] = changed[room]
            return PIL.Image.fromarray(result)
        # Farther targets go first, so nearer replacements are drawn over them
        for mask in sorted(target.masks + target.objects, key=meegeren.imaging.bottom):
            if cancelled.is_set():
                break
            # The replacement may differ in shape from the target
            image = painter.paint(
                image,
                meegeren.imaging.rectangle(
                    mask.shape, meegeren.imaging.bounding_box(mask)
                )
                & ~target.kept,
                content,
                "replace",
                feathering,
                models.upscaler,
                cancelled,
            )
        return image

    try:
        async with meegeren.cancellation.cancel_on_disconnect(request):
            return png(
                mark(
                    await asyncio.get_running_loop().run_in_executor(
                        executor, run_in_context, opentelemetry.context.get_current()
                    )
                )
            )
    except asyncio.CancelledError:
        cancelled.set()
        raise


@app.post("/v1/insert")
async def insert(
    request: fastapi.Request,
    content: str = fastapi.Form(min_length=1),
    relation: typing.Literal["on", "in", "left", "right", "front"] = fastapi.Form(),
    feathering: int = fastapi.Form(s.mask_feathering, ge=0),
    painter: meegeren.inpainter.Painter = fastapi.Depends(get_painter),
    target: Target = fastapi.Depends(get_target),
    models: PipelineModels = fastapi.Depends(get_pipeline_models),
    mark: typing.Callable[[PIL.Image.Image], PIL.Image.Image] = fastapi.Depends(
        get_watermark
    ),
) -> fastapi.Response:
    """Returns the image with `content` drawn at `relation` (`on`, `in`, `left`, `right` or `front`) to the nearest target as a PNG."""
    anchor = max(target.masks + target.objects, key=meegeren.imaging.bottom)
    left, top, right, bottom = meegeren.imaging.bounding_box(anchor)
    width, height = right - left, bottom - top
    match relation:
        case "on":
            region = meegeren.imaging.rectangle(
                anchor.shape, (left, top - height // 2, right, bottom - height // 2)
            )
        case "in":
            region = anchor
        case "left":
            region = meegeren.imaging.rectangle(
                anchor.shape, (left - width, top, left, bottom)
            )
        case "right":
            region = meegeren.imaging.rectangle(
                anchor.shape, (right, top, right + width, bottom)
            )
        case "front":
            region = meegeren.imaging.rectangle(
                anchor.shape, (left, top + height // 2, right, bottom + height // 2)
            )
    region = region & ~target.kept
    if not region.any():
        raise fastapi.HTTPException(
            http.HTTPStatus.UNPROCESSABLE_ENTITY,
            f"relation {relation} leaves no room within the image outside keep",
        )

    cancelled = threading.Event()

    def run_in_context(ctx: opentelemetry.context.Context):
        opentelemetry.context.attach(ctx)
        return painter.paint(
            target.image,
            region,
            content,
            "insert",
            feathering,
            models.upscaler,
            cancelled,
        )

    try:
        async with meegeren.cancellation.cancel_on_disconnect(request):
            return png(
                mark(
                    await asyncio.get_running_loop().run_in_executor(
                        executor, run_in_context, opentelemetry.context.get_current()
                    )
                )
            )
    except asyncio.CancelledError:
        cancelled.set()
        raise


@app.post("/v1/background")
async def background(
    request: fastapi.Request,
    image: fastapi.UploadFile,
    content: str = fastapi.Form(min_length=1),
    prompt: str | None = fastapi.Form(None),
    mask: fastapi.UploadFile | None = None,
    object_locator: meegeren.model.ObjectLocatorConfiguration = fastapi.Depends(
        get_object_locator_configuration
    ),
    painter: meegeren.inpainter.Painter = fastapi.Depends(get_painter),
    models: PipelineModels = fastapi.Depends(get_pipeline_models),
    mark: typing.Callable[[PIL.Image.Image], PIL.Image.Image] = fastapi.Depends(
        get_watermark
    ),
    acquire_lock: collections.abc.Callable[
        [], collections.abc.Awaitable[None]
    ] = fastapi.Depends(hold_lock),
) -> fastapi.Response:
    """Returns the image with everything but the objects `prompt` or `mask` names redrawn as `content` behind them as a PNG."""
    labels = meegeren.object_locator.parse_labels(prompt or "")
    if len(labels) == 0 and mask is None:
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST, "One of prompt or mask is required"
        )

    source = await read_image(image, "RGB")
    subjects = []
    if mask is not None:
        subjects.append(await read_mask(mask, source))
    await acquire_lock()
    if len(labels) > 0:

        def run_in_context(ctx: opentelemetry.context.Context):
            opentelemetry.context.attach(ctx)
            return models.object_locator.locate(
                source, labels, object_locator.threshold
            )

        async with meegeren.cancellation.cancel_on_disconnect(request):
            located = await asyncio.get_running_loop().run_in_executor(
                executor, run_in_context, opentelemetry.context.get_current()
            )
        subjects.extend(m for _, m in located)
    subject = numpy.logical_or.reduce(
        [numpy.zeros((source.height, source.width), dtype=bool)] + subjects
    )
    if not subject.any():
        raise fastapi.HTTPException(
            http.HTTPStatus.UNPROCESSABLE_ENTITY, "Nothing matched prompt or mask"
        )
    if subject.all():
        raise fastapi.HTTPException(
            http.HTTPStatus.UNPROCESSABLE_ENTITY,
            "prompt or mask leaves no background",
        )

    cancelled = threading.Event()

    def run_in_context(ctx: opentelemetry.context.Context):
        opentelemetry.context.attach(ctx)
        if models.matting is None:
            alpha, foreground = subject.astype(numpy.float32), source
        else:
            alpha, foreground = models.matting.matte(source, subject)
        if not (alpha < 1).any():
            raise fastapi.HTTPException(
                http.HTTPStatus.UNPROCESSABLE_ENTITY,
                "prompt or mask leaves no background",
            )
        if not (alpha > 0).any():
            raise fastapi.HTTPException(
                http.HTTPStatus.UNPROCESSABLE_ENTITY, "Nothing matched prompt or mask"
            )
        if cancelled.is_set():
            return source
        # The background is drawn behind what is partly opaque as well, which is then laid over it
        painted = painter.paint(
            source, alpha < 1, content, "background", 0, models.upscaler, cancelled
        )
        for harmonizer in models.harmonizers:
            if cancelled.is_set():
                break
            foreground = harmonizer.harmonize(
                foreground, alpha, painted, content, cancelled
            )
        return meegeren.inpainter.composite(
            painted,
            (0, 0, source.width, source.height),
            numpy.asarray(foreground),
            alpha,
        )

    try:
        async with meegeren.cancellation.cancel_on_disconnect(request):
            return png(
                mark(
                    await asyncio.get_running_loop().run_in_executor(
                        executor, run_in_context, opentelemetry.context.get_current()
                    )
                )
            )
    except asyncio.CancelledError:
        cancelled.set()
        raise


def get_upscaler(
    models: Models = fastapi.Depends(get_models),
) -> meegeren.upscaler.Upscaler:
    if models.upscaler is None:
        raise fastapi.HTTPException(
            http.HTTPStatus.NOT_IMPLEMENTED, "No upscaler is loaded"
        )
    return models.upscaler


@app.post("/v1/upscale")
async def upscale(
    request: fastapi.Request,
    image: fastapi.UploadFile,
    scale: float | None = fastapi.Form(None, gt=1),
    upscaler: meegeren.upscaler.Upscaler = fastapi.Depends(get_upscaler),
    mark: typing.Callable[[PIL.Image.Image], PIL.Image.Image] = fastapi.Depends(
        get_watermark
    ),
    acquire_lock: collections.abc.Callable[
        [], collections.abc.Awaitable[None]
    ] = fastapi.Depends(hold_lock),
) -> fastapi.Response:
    """Returns the image enlarged `scale` times, up to and by default the upscaler's own scale, as a PNG."""
    if scale is None:
        scale = upscaler.scale
    if scale > upscaler.scale:
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST,
            f"scale {scale} exceeds the upscaler's {upscaler.scale}",
        )
    source = await read_image(image, "RGB")
    if source.width * source.height * upscaler.scale**2 > PIL.Image.MAX_IMAGE_PIXELS:
        raise fastapi.HTTPException(
            http.HTTPStatus.BAD_REQUEST,
            f"{source.width}x{source.height} enlarged {upscaler.scale} times exceeds {PIL.Image.MAX_IMAGE_PIXELS} pixels",
        )

    await acquire_lock()
    cancelled = threading.Event()

    def run_in_context(ctx: opentelemetry.context.Context):
        opentelemetry.context.attach(ctx)
        upscaled = upscaler.upscale(source, cancelled)
        if scale == upscaler.scale:
            return upscaled
        return upscaled.resize(
            (round(source.width * scale), round(source.height * scale)),
            PIL.Image.Resampling.LANCZOS,
        )

    try:
        async with meegeren.cancellation.cancel_on_disconnect(request):
            return png(
                mark(
                    await asyncio.get_running_loop().run_in_executor(
                        executor, run_in_context, opentelemetry.context.get_current()
                    )
                )
            )
    except asyncio.CancelledError:
        cancelled.set()
        raise


@app.get("/healthz")
def health() -> str:
    return "OK"


@app.get("/metrics")
def metrics() -> fastapi.Response:
    return fastapi.Response(
        prometheus_client.generate_latest(),
        media_type=prometheus_client.CONTENT_TYPE_LATEST,
    )


if __name__ == "__main__":
    if s.is_debug():
        import dotenv

        dotenv.load_dotenv(override=True)

    import uvicorn

    uvicorn.run(
        app,
        host=s.host,
        port=s.port,
        log_level=s.convert_log_level(),
        timeout_keep_alive=s.idle_timeout,
        timeout_graceful_shutdown=s.termination_grace_period_seconds,
        access_log=False,
    )
