import os
import pathlib

import kernels
import numpy
import PIL.Image
import torch
import torchvision.ops
import transformers

import meegeren.object_locator
import meegeren.offload
import meegeren.telemetry

SAM_BOX_CHUNK_SIZE = 16
# transformers' post-processing keeps every box
# https://github.com/IDEA-Research/Grounded-Segment-Anything/blob/main/grounded_sam_simple_demo.py
GROUNDING_NMS_IOU_THRESHOLD = 0.8


class GroundingDinoSam2Locator(meegeren.object_locator.Locator):
    def __init__(
        self,
        grounding_processor: transformers.ProcessorMixin,
        grounding_model: torch.nn.Module,
        segmentation_processor: transformers.ProcessorMixin,
        segmentation_model: torch.nn.Module,
        keep_on_device: bool,
        device: str,
    ):
        self.grounding_processor = grounding_processor
        self.grounding_model = grounding_model.to(
            device if keep_on_device else "cpu"
        ).eval()
        self.segmentation_processor = segmentation_processor
        self.segmentation_model = segmentation_model.to(
            device if keep_on_device else "cpu"
        ).eval()
        self.keep_on_device = keep_on_device
        self.device = device

    @classmethod
    def from_pretrained(
        cls,
        grounding_model: str,
        segmentation_model: str,
        deformable_attention_path: str,
        dtype: torch.dtype,
        keep_on_device: bool,
        device: str,
    ) -> GroundingDinoSam2Locator:
        grounding_configuration = transformers.AutoConfig.from_pretrained(
            grounding_model
        )
        # transformers 5.19 leaves Grounding DINO base's box heads newly initialized
        # transformers 5.19 ties MM-Grounding-DINO's box heads of every decoder layer to the first layer's
        grounding_configuration.tie_word_embeddings = False
        model = transformers.AutoModelForZeroShotObjectDetection.from_pretrained(
            grounding_model, config=grounding_configuration, dtype=dtype
        )
        state = transformers.modeling_utils.load_state_dict(
            os.path.join(grounding_model, "model.safetensors")
        )
        if grounding_configuration.decoder_bbox_embed_share:
            state |= {
                k.removeprefix("model.decoder.").replace(
                    "bbox_embed.0.", f"bbox_embed.{i}.", 1
                ): v
                for k, v in state.items()
                if k.removeprefix("model.decoder.").startswith("bbox_embed.0.")
                for i in range(grounding_configuration.decoder_layers)
            }
        model.load_state_dict(state, strict=False)
        if device.startswith("cuda"):
            with kernels.use_kernel_mapping(
                {
                    "MultiScaleDeformableAttention": {
                        "cuda": kernels.LocalLayerRepository(
                            repo_path=pathlib.Path(deformable_attention_path),
                            layer_name="MultiScaleDeformableAttention",
                        )
                    }
                }
            ):
                # Kernelizing the whole model warns for each GeLU it leaves without a kernel
                for module in model.modules():
                    if (
                        getattr(type(module), "kernel_layer_name", None)
                        == "MultiScaleDeformableAttention"
                    ):
                        kernels.kernelize(
                            module, mode=kernels.Mode.INFERENCE, device="cuda"
                        )
        return cls(
            grounding_processor=transformers.AutoProcessor.from_pretrained(
                grounding_model
            ),
            grounding_model=model,
            segmentation_processor=transformers.Sam2Processor.from_pretrained(
                segmentation_model
            ),
            segmentation_model=transformers.Sam2Model.from_pretrained(
                segmentation_model, dtype=dtype
            ),
            keep_on_device=keep_on_device,
            device=device,
        )

    @torch.inference_mode()
    def detect(
        self, image: PIL.Image.Image, labels: list[str], threshold: float
    ) -> tuple[torch.Tensor, torch.Tensor]:
        with meegeren.telemetry.tracer.start_as_current_span(
            "grounding_dino_sam2.detect"
        ):
            inputs = self.grounding_processor(
                images=image, text=[labels], return_tensors="pt"
            ).to(self.device, self.grounding_model.dtype)
            with meegeren.offload.placed(
                self.device, self.keep_on_device, self.grounding_model
            ):
                outputs = self.grounding_model(**inputs)
            result = self.grounding_processor.post_process_grounded_object_detection(
                outputs,
                inputs.input_ids,
                threshold=threshold,
                target_sizes=[(image.height, image.width)],
            )[0]
            # transformers' post-processing names each box by every token above text_threshold, which joins labels
            scores, tokens = outputs.logits[0].sigmoid().max(dim=-1)
            periods = inputs.input_ids[
                0
            ] == self.grounding_processor.tokenizer.convert_tokens_to_ids(".")
            indices = (periods.cumsum(0) - periods.long())[tokens[scores > threshold]]
            keep = torchvision.ops.nms(
                result["boxes"].float(),
                result["scores"].float(),
                GROUNDING_NMS_IOU_THRESHOLD,
            )
            return result["boxes"][keep].cpu(), indices[keep].cpu()

    @torch.inference_mode()
    def segment(
        self, image: PIL.Image.Image, boxes: torch.Tensor
    ) -> list[numpy.ndarray]:
        with meegeren.telemetry.tracer.start_as_current_span(
            "grounding_dino_sam2.segment"
        ):
            inputs = self.segmentation_processor(
                images=image, input_boxes=[boxes.tolist()], return_tensors="pt"
            ).to(self.device)
            masks = []
            with meegeren.offload.placed(
                self.device, self.keep_on_device, self.segmentation_model
            ):
                embeddings = self.segmentation_model.get_image_embeddings(
                    inputs["pixel_values"]
                )
                # Each box's mask is upsampled to the full image size
                for i in range(0, len(boxes), SAM_BOX_CHUNK_SIZE):
                    outputs = self.segmentation_model(
                        image_embeddings=embeddings,
                        input_boxes=inputs["input_boxes"][
                            :, i : i + SAM_BOX_CHUNK_SIZE
                        ],
                        multimask_output=False,
                    )
                    # (boxes, 1, height, width)
                    masks.extend(
                        self.segmentation_processor.post_process_masks(
                            outputs.pred_masks.float().cpu(),
                            inputs["original_sizes"].cpu(),
                        )[0][:, 0].numpy()
                    )
            return masks

    def locate(
        self, image: PIL.Image.Image, labels: list[str], threshold: float
    ) -> list[tuple[int, numpy.ndarray]]:
        boxes, indices = self.detect(image, labels, threshold)
        if len(boxes) == 0:
            return []
        return list(zip(indices.tolist(), self.segment(image, boxes)))
