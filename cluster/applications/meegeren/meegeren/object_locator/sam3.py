import numpy
import PIL.Image
import torch
import transformers

import meegeren.object_locator
import meegeren.offload
import meegeren.telemetry


class Sam3Locator(meegeren.object_locator.Locator):
    def __init__(
        self,
        processor: transformers.ProcessorMixin,
        model: torch.nn.Module,
        mask_threshold: float,
        keep_on_device: bool,
        device: str,
    ):
        self.processor = processor
        self.model = model.to(device if keep_on_device else "cpu").eval()
        self.mask_threshold = mask_threshold
        self.keep_on_device = keep_on_device
        self.device = device

    @classmethod
    def from_pretrained(
        cls,
        model: str,
        mask_threshold: float,
        dtype: torch.dtype,
        keep_on_device: bool,
        device: str,
    ) -> Sam3Locator:
        # facebook/sam3's config.json declares Sam3VideoModel and Sam3VideoProcessor
        return cls(
            processor=transformers.Sam3Processor.from_pretrained(model),
            model=transformers.Sam3Model.from_pretrained(model, dtype=dtype),
            mask_threshold=mask_threshold,
            keep_on_device=keep_on_device,
            device=device,
        )

    @torch.inference_mode()
    def locate(
        self, image: PIL.Image.Image, labels: list[str], threshold: float
    ) -> list[tuple[int, numpy.ndarray]]:
        with (
            meegeren.telemetry.tracer.start_as_current_span("sam3.locate"),
            meegeren.offload.placed(self.device, self.keep_on_device, self.model),
        ):
            image_inputs = self.processor(images=image, return_tensors="pt").to(
                self.device
            )
            # The image encoder dominates the cost
            vision_embeds = self.model.get_vision_features(
                pixel_values=image_inputs["pixel_values"]
            )
            masks = []
            for i, label in enumerate(labels):
                text_inputs = self.processor(text=label, return_tensors="pt").to(
                    self.device
                )
                outputs = self.model(vision_embeds=vision_embeds, **text_inputs)
                result = self.processor.post_process_instance_segmentation(
                    outputs,
                    threshold=threshold,
                    mask_threshold=self.mask_threshold,
                    target_sizes=[(image.height, image.width)],
                )[0]
                masks.extend((i, m) for m in result["masks"].bool().cpu().numpy())
            return masks
