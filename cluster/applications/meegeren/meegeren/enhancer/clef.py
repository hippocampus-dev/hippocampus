import importlib.util
import os
import sys
import threading
import types
import typing

import PIL.Image
import torch

import meegeren.enhancer
import meegeren.offload
import meegeren.telemetry

CLEF_THUMBNAIL_SIZE = 512


class ClefEnhancer(meegeren.enhancer.Enhancer):
    def __init__(
        self,
        module: types.ModuleType,
        model: torch.nn.Module,
        processor: typing.Any,
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
        keep_on_device: bool,
        device: str,
    ):
        self.module = module
        self.model = model
        self.processor = processor
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
        self.keep_on_device = keep_on_device
        self.device = device

    @classmethod
    def from_pretrained(
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
        dtype: torch.dtype,
        keep_on_device: bool,
        device: str,
    ) -> ClefEnhancer:
        # https://huggingface.co/Cloudflare/clef-flash#usage
        spec = importlib.util.spec_from_file_location(
            "joint_schema_model", os.path.join(model, "joint_schema_model.py")
        )
        module = importlib.util.module_from_spec(spec)
        # https://docs.python.org/3/library/importlib.html#importing-a-source-file-directly
        sys.modules[spec.name] = module
        spec.loader.exec_module(module)
        clef, processor = module.load_release_model(
            model, device=device if keep_on_device else "cpu", dtype=dtype
        )
        return cls(
            module=module,
            model=clef,
            processor=processor,
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
            keep_on_device=keep_on_device,
            device=device,
        )

    def ask(self, images: list[PIL.Image.Image], instructions: str) -> float | None:
        thumbnails = []
        for image in images:
            thumbnail = image.copy()
            thumbnail.thumbnail((CLEF_THUMBNAIL_SIZE, CLEF_THUMBNAIL_SIZE))
            thumbnails.append(thumbnail)
        response = self.module.systemone(
            self.model,
            self.processor,
            {
                "model": "clef",
                "state": self.context,
                "images": thumbnails,
                "questions": {"answer": {"type": "noul", "instructions": instructions}},
            },
        )
        return response["answers"]["answer"]["noul"]

    def enhance(
        self,
        image: PIL.Image.Image,
        strength: float | None,
        cancelled: threading.Event,
    ) -> tuple[PIL.Image.Image, list[str]]:
        with (
            meegeren.telemetry.tracer.start_as_current_span("clef.enhance"),
            meegeren.offload.placed(self.device, self.keep_on_device, self.model),
        ):
            return super().enhance(image, strength, cancelled)
