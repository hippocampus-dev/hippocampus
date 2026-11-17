# meegeren

<!-- TOC -->
* [meegeren](#meegeren)
  * [Features](#features)
  * [Usage](#usage)
    * [Pipelines](#pipelines)
    * [Precision comparison](#precision-comparison)
    * [Inpainter comparison](#inpainter-comparison)
    * [Object locator comparison](#object-locator-comparison)
    * [Text locator comparison](#text-locator-comparison)
    * [Refiner comparison](#refiner-comparison)
    * [Matting comparison](#matting-comparison)
    * [Harmonizer comparison](#harmonizer-comparison)
    * [Upscaler comparison](#upscaler-comparison)
    * [Enhancer comparison](#enhancer-comparison)
  * [Development](#development)
    * [E2E Testing](#e2e-testing)
    * [LoRA Training](#lora-training)
  * [Deployment](#deployment)
<!-- TOC -->

meegeren is a FastAPI server that removes, replaces and inserts objects named in natural language in images, replaces the background behind them, removes text, corrects exposure and white balance, and upscales images.

## Features

- [x] `POST /v1/remove` returns the image with its targets filled in as a PNG, or streams it as server-sent events shaped like the OpenAI image edit stream with `stream=true`
- [x] `POST /v1/replace` returns the image with the box around each target redrawn as `content` as a PNG, or with `restyle=true` each target itself redrawn in the material or color `content` names, and with `relight` the rest relit as lit by the light it describes coming in through the targets
- [x] `POST /v1/insert` returns the image with `content` drawn at `relation` (`on`, `in`, `left`, `right` or `front`) to the nearest target as a PNG
- [x] `POST /v1/background` returns the image with everything but the objects `prompt` or `mask` names redrawn as `content` behind them as a PNG
- [x] `POST /v1/masks` returns the mask `/v1/remove` would fill before OSOR narrows it, or with `restyle=true` the mask `/v1/replace` restyles, for tuning the targets, `threshold`, `dilation` and `keep`, or for cutting out and blurring the targets
- [x] `POST /v1/upscale` returns the image enlarged `scale` times, up to and by default the upscaler's own scale, as a PNG
- [x] `POST /v1/enhance` returns the image with its exposure brightened and its color cast removed where the enhancer finds that needed, preferred and harmless, as a PNG
- [x] `watermark` draws text, such as a note that AI edited the image, on what `/v1/remove`, `/v1/replace`, `/v1/insert`, `/v1/background`, `/v1/upscale` and `/v1/enhance` return

## Usage

Every endpoint but `/v1/background`, `/v1/upscale` and `/v1/enhance` takes `image` and one or more of the targets below.

| Field | Targets |
|-------|---------|
| `prompt` | the objects named by English phrases separated by periods |
| `text=true` | text detected with CRAFT |
| `mask` | the white area of an image the size of `image` |

```sh
$ curl -sSf -F image=@photo.jpg -F 'prompt=power line. person.' -o removed.png http://127.0.0.1:8080/v1/remove
$ curl -sSf -F image=@photo.jpg -F mask=@mask.png -o removed.png http://127.0.0.1:8080/v1/remove
$ curl -sSf -F image=@photo.jpg -F 'prompt=table. chair.' -F 'keep=kitchen counter.' -F 'watermark=この画像はAIによって編集されています' -o removed.png http://127.0.0.1:8080/v1/remove
```

`/v1/replace`, `/v1/insert` and `/v1/background` need an inpainter of type `gpt-image` or `qwen-image-edit`, and return 501 with any other.

| Field | Effect |
|-------|--------|
| `dilation` | widens the targets by this many pixels to cover outlines and shadows, by default 15, or 0.25% of the longer side of `image` with `restyle=true`, and 0 for `mask` |
| `keep` | leaves the objects it names unchanged, except those `grounding-dino-sam2` scores higher for a phrase of `prompt` |
| `feathering` | blends what the inpainter draws into the image over up to this many pixels inside the edge of the area it draws in, by default 0, on `/v1/remove`, `/v1/replace` and `/v1/insert`, returning 400 on `/v1/remove` with `osor` or `whole_image: true` |
| `restyle=true` | takes `prompt` alone as its targets, returning 400 with `text` or `mask` and on `/v1/remove` and `/v1/insert` |
| `relight` | relights the rest of the restyled image as lit by the light it describes, keeping its detail, returning 400 without `restyle=true` and 501 with `gpt-image` |

```sh
$ curl -sSf -F image=@photo.jpg -F 'prompt=desk.' -F relation=on -F 'content=a potted plant' -o inserted.png http://127.0.0.1:8080/v1/insert
$ curl -sSf -F image=@photo.jpg -F 'prompt=sofa.' -F 'content=navy blue velvet' -F restyle=true -o restyled.png http://127.0.0.1:8080/v1/replace
$ curl -sSf -F image=@photo.jpg -F 'prompt=window.' -F 'content=a clear sky at sunset with warm orange light' -F restyle=true -F 'relight=warm orange evening sunlight' -o relit.png http://127.0.0.1:8080/v1/replace
```

`/v1/background` keeps the objects `prompt` names and the white area of `mask`, redraws the rest of `image` as `content`, takes no `text`, `dilation` or `keep`, and returns 422 when nothing matches or nothing is left to redraw.
What the objects hold or rest on is redrawn unless `prompt` names it, such as `person. basket of snacks.` in place of `person.`.

```sh
$ curl -sSf -F image=@photo.jpg -F 'prompt=person. basket of snacks.' -F 'content=a quiet street in a European old town' -o background.png http://127.0.0.1:8080/v1/background
```

Cutting out or blurring the targets needs no endpoint of its own, as `/v1/masks` with `dilation=0` returns every target merged into one mask aligned with `image` once its EXIF orientation is applied.

```sh
$ curl -sSf -F image=@photo.jpg -F 'prompt=sofa.' -F dilation=0 -o mask.png http://127.0.0.1:8080/v1/masks
$ magick photo.jpg -auto-orient mask.png -alpha off -compose CopyOpacity -composite -trim +repage cutout.png
$ magick photo.jpg -auto-orient \( +clone -blur 0x20 \) mask.png -composite blurred.png
```

`/v1/upscale` returns 400 for `image` that the upscaler's own scale would enlarge over Pillow's `MAX_IMAGE_PIXELS`.

```sh
$ curl -sSf -F image=@photo.jpg -F scale=2 -o upscaled.png http://127.0.0.1:8080/v1/upscale
```

`/v1/enhance` takes `image` and, in place of the enhancer's own, `strength`, returns it unchanged when the enhancer takes it for a black-and-white, sepia or old film reproduction, and names the corrections it takes, `exposure` and then `white-balance`, in the `Applied-Corrections` header, or `none`.

```sh
$ curl -sSf -D - -F image=@photo.jpg -o enhanced.png http://127.0.0.1:8080/v1/enhance
```

### Pipelines

`CONFIG_FILE` lists `pipelines`, each combining a model per stage, `upscalers` and `enhancers`, and `PIPELINE`, `UPSCALER` and `ENHANCER` name the one of each to load.
`make dev` sets the first two to `quality` and `real-esrgan` unless given.
Leaving any of them unset or empty makes the endpoints needing it return 501, and leaving all three stops the server at startup.

| Stage | `type` | Behavior |
|-------|--------|----------|
| `object_locator` | `grounding-dino-sam2` | boxes each phrase with Grounding DINO, then masks each box with SAM 2 |
| `object_locator` | `sam3` | masks every instance of each phrase with SAM 3 |
| `text_locator` | `craft` | masks text with CRAFT |
| `inpainter` | `big-lama` | fills the targets from their surroundings |
| `inpainter` | `osor` | removes each object together with its shadows and reflections with OSOR, but leaves masked pixels it does not take for an object |
| `inpainter` | `gpt-image` | fills the targets through the OpenAI image edit API, given `OPENAI_API_KEY` |
| `inpainter` | `qwen-image-edit` | removes the targets with Qwen-Image-Edit and an object remover LoRA |
| `refiner` | `sdxl-inpainting` | redraws what the inpainter changed with SDXL-Inpainting prompted by its `prompt` and `negative_prompt` |
| `matting` | `vitmatte` | decides for `/v1/background` how opaque the kept objects are within `trimap_width` pixels on each side of their located edge with ViTMatte, and frees the colors there of the old background |
| `harmonizers` | `pct-net` | brings the colors of what `/v1/background` keeps to those of the new background with PCT-Net's pixel-wise color transforms |
| `harmonizers` | `ic-light` | relights what `/v1/background` keeps for the new background with IC-Light, prompted by `content`, its `added_prompt` and its `negative_prompt`, redrawing its first result `highres_scale` times larger by `highres_strength` unless `highres_scale` is null |
| `upscalers` | `real-esrgan` | enlarges the image with Real-ESRGAN in one pass, weighing its weights against those trained without denoising by `denoise_strength` |
| `upscalers` | `swinir` | enlarges the image with SwinIR on tiles |
| `enhancers` | `decisions` | asks the OpenAI Decisions API with `model`, given `OPENAI_API_KEY`, the questions its `context` and `_instructions` word, keeping each correction whose answers pass its `_threshold` fields, and correcting more the higher `strength` is, from 0 to 1 |
| `enhancers` | `clef` | asks the same questions as `decisions` of Clef, Cloudflare's open-weight decision model, at `model` |

| Setting | On | Effect |
|---------|----|--------|
| `upscaler` | a pipeline | names the entry of `upscalers` that enlarges what the inpainter or the refiner draws 1.5 times smaller than the area it fills or more |
| `harmonizers` | a pipeline | run in their order, each on what the one before returns, and none by default |
| `redetect: true` | `remove` | makes `/v1/remove` remove the objects overlapping none nearer in rounds, nearest first, locating the rest again after each round, up to `max_removals` rounds |
| `remove_additions: true` | `remove`, with `sdxl-inpainting` | makes `/v1/remove` also remove, region by region, what each refinement changes most and the text found within what was removed |
| `whole_image: true` | `remove`, with `gpt-image` or `qwen-image-edit` | makes `/v1/remove` send the whole image in place of the area around the targets, leaving no seams but letting the model redraw the rest of the image |
| `loras` | `sdxl-inpainting` | merges each LoRA file at `path` into `base_model` at startup, in order and scaled by its `weight` |
| `timestep_spacing` | `sdxl-inpainting` | replaces that of `base_model`'s scheduler, such as `trailing` for the distilled LoRAs of Hyper-SDXL and SDXL-Lightning, whose `steps` times `strength` is the number of steps they then take |
| `keep_on_device: false` | an upscaler, `clef`, a stage other than `gpt-image` and `qwen-image-edit`, or the `text_encoder`, `transformer` or `vae` of `qwen-image-edit` | keeps its models in CPU memory and moves them onto `DEVICE` only while they run, for stages that leave too little GPU memory for the others |
| `dtype` | `real-esrgan`, `clef` or a stage other than `big-lama` and `gpt-image` | loads its models in `float32` by default, `clef` in `bfloat16`, or in `float16` or `bfloat16`, other than `float16` on `qwen-image-edit` |

`config.yaml` takes `float16` on every stage of every pipeline but `cpu`, and `bfloat16` on `qwen-image-edit` and `ic-light`.
That fits each pipeline on a 20GB GPU with every model on `DEVICE`, but for `qwen-image-edit`, which needs `keep_on_device: false` on its text encoder or its transformer, and on the `ic-light` beside them.

Requests override the fields below, validated as in `CONFIG_FILE`, and get 400 for a stage the pipeline lacks.

| Field | Overrides |
|-------|-----------|
| `threshold` | `object_locator`'s `threshold`, the score below which a detection is dropped |
| `refiner_prompt`, `refiner_negative_prompt`, `refiner_guidance_scale`, `refiner_strength`, `refiner_steps` | the `refiner` field of the same name on `/v1/remove` |
| `remove_additions` | `remove`'s `remove_additions` on `/v1/remove`, getting 400 without a `refiner` |

### Precision comparison

Running each model alone in each `dtype` on an NVIDIA RTX 4000 SFF Ada (20GB), over the inputs of the comparisons below, gave the peak GPU memory, the time and the agreement with `float32` below.
Masks are compared by IoU, and whole images, opacities and people by PSNR, as the median over the photos and, in parentheses, the worst.

| Model | `float32` | `float16` | `bfloat16` |
|-------|-----------|-----------|------------|
| SAM 3 | 3.96GiB, 2.42s | 2.12GiB, 0.70s, IoU 0.9997 (0.9992) | 2.12GiB, 0.68s, IoU 0.9979 (0.9531) |
| MM-Grounding-DINO large and SAM 2.1 large | 8.01GiB, 1.92s | 4.02GiB, 0.83s, IoU 0.9989 (0.9565) | 4.04GiB, 0.81s, IoU 0.9779 (0) |
| MM-Grounding-DINO base and SAM 2.1 large | 2.99GiB, 0.90s | 1.51GiB, 0.33s, IoU 0.9987 (0.9485) | 1.54GiB, 0.32s, IoU 0.9859 (0.8966) |
| Grounding DINO base and SAM 2.1 large | 2.99GiB, 0.89s | 1.51GiB, 0.34s, IoU 0.9990 (0.8939) | 1.54GiB, 0.32s, IoU 0.9886 (0.2486) |
| CRAFT | 0.84GiB, 0.11s | 0.65GiB, 0.07s, IoU 1.0000 (0.9988) | 0.65GiB, 0.07s, IoU 1.0000 (0.6296) |
| OSOR | 11.26GiB, 1.01s | 5.87GiB, 0.47s, 57.1dB | 5.87GiB, 0.45s, 47.7dB |
| SDXL-Inpainting | 16.84GiB, 28.9s | 8.96GiB, 8.2s, 61.1dB | 8.96GiB, 7.8s, 51.6dB |
| Real-ESRGAN general x4v3 | 0.12GiB, 0.04s | 0.07GiB, 0.03s, 59.9dB (59.3dB) | 0.07GiB, 0.02s, 53.0dB (52.0dB) |
| ViTMatte base Distinctions-646 | 2.85GiB, 0.88–3.18s | 1.43GiB, 0.51–1.75s, 79.4dB (71.2dB) | 1.43GiB, 0.51–1.75s, 62.4dB (55.6dB) |
| PCT-Net ViT | 0.57GiB, 0.07–0.16s | 0.57GiB, 0.05–0.14s, 62.1dB (61.5dB) | 0.57GiB, 0.05–0.14s, 53.6dB (52.1dB) |
| IC-Light fbc with Realistic Vision v5.1 | 9.32GiB, 41.5–58.0s | 8.43GiB, 11.6–15.5s, 59.0dB (56.2dB) | 5.20GiB, 11.3–14.8s, 48.8dB (45.2dB), and 3.35GiB with `keep_on_device: false` |
| Qwen-Image-Edit-2511 | not run, beyond 20GB | rejected by `dtype` | weights of 16.6GB and 13.2GB for the text encoder and the transformer |
| big-lama | runs | fails, as cuFFT takes only power-of-two sizes in half precision | fails, as its TorchScript casts inputs to `float32` |
| SwinIR-L | runs | rejected by spandrel | fails on tiles other than 64px |

Removing the furniture `e2e.sh` names from the same photos with OSOR and the refiner seeded alike, half precision changed SAM 3's and OSOR's results in a few photos without either precision doing consistently better, left the refiner's indistinguishable from `float32`, and through MM-Grounding-DINO base left a side table and a chair in `bfloat16` that `float16` and `float32` remove.

### Inpainter comparison

Removing chairs and sofas from interior photos on an NVIDIA RTX 4000 SFF Ada (20GB) gave the results below, where commercial use follows the weights' license and that of the model they are fine-tuned from.

| Model | `type` | Weights license | Commercial use | Result | Time per photo |
|-------|--------|-----------------|----------------|--------|----------------|
| OSOR (SDXL-Inpainting) | `osor` | CreativeML Open RAIL++-M | Yes, within the license's use restrictions | removes sofas and people cleanly with their shadows, but also removes table legs behind chairs | 8–45s |
| RORem | none, removed | Apache-2.0, fine-tuned from CreativeML Open RAIL++-M | Yes, within CreativeML Open RAIL++-M's use restrictions | leaves semi-transparent remnants of chairs and blocky smudges on floors | 84–220s |
| Qwen-Image-Edit-2511 with an object remover and an 8-step Lightning LoRA | `qwen-image-edit` | Apache-2.0 | Yes | redraws table legs hidden behind chairs, but redraws sofas as other sofas | 88–101s |
| Qwen-Image-Edit-2511 with the same LoRAs, given the whole image | `qwen-image-edit` with `whole_image: true` in `remove` | Apache-2.0 | Yes | in the photos `e2e.sh` sends, leaves none of the semi-transparent remnants of sofas and rugs the area around the targets leaves in 2, but still redraws furniture as other furniture in 3 and keeps a sofa in 1 | 72–81s |
| ObjectClear | none, removed | NTU S-Lab License 1.0 | No | redraws table legs and table settings hidden behind chairs, but keeps people lying on sofas | 115–160s |
| FLUX.2 [klein] 4B with an object remover LoRA from its ComfyUI file, as its `flux-object-remove-lora.safetensors` on Hugging Face is truncated | none, not added | Apache-2.0 | Yes | removes almost nothing given the box around each target, and redraws chairs and sofas as other chairs and sofas given targets painted red | 16–25s |
| big-lama | `big-lama` | Apache-2.0 | Yes | not compared | |
| GPT Image | `gpt-image` | OpenAI API terms | Not checked | in the photos `e2e.sh` sends, leaves the shapes of the targets as patches in 5 and keeps a sofa in 1 | 30–36s |
| GPT Image, given the whole image | `gpt-image` with `whole_image: true` in `remove` | OpenAI API terms | Not checked | in the photos `e2e.sh` sends, removes every target without seams in 5, keeping the kitchen island and counter the `quality` pipeline with MM-Grounding-DINO base removes in 2, but keeps a sofa and redraws a kitchen island in 1 | 31–35s |

### Object locator comparison

Removing the furniture `e2e.sh` names from 26 photos of properties listed on LIFULL HOME'S through the `quality` pipeline on the same GPU, twice with different seeds, gave the results below, where `quality` used MM-Grounding-DINO base.

| Model | `type` | Weights license | Commercial use | Result | Time per photo |
|-------|--------|-----------------|----------------|--------|----------------|
| Grounding DINO base | `grounding-dino-sam2` | Apache-2.0 | Yes | removes kitchen islands in 5 photos, and leaves chairs or sofas in 2 | 0.4–0.7s |
| Grounding DINO base on overlapping 800px tiles as well | none, not added | Apache-2.0 | Yes | finds more boxes, not compared end to end | 1.5–2.1s |
| MM-Grounding-DINO base | `grounding-dino-sam2` | Apache-2.0 | Yes | removes the targets better than Grounding DINO base in 11 photos and worse in 5, keeping the kitchen islands Grounding DINO base removes, but removes pendant lights in 2 photos, closet doors in 1 and a kitchen in 1, and leaves OSOR drawing clutter in 1 | 0.3–0.7s |
| MM-Grounding-DINO large | `grounding-dino-sam2` | Apache-2.0 | Yes | runs out of memory beside OSOR in `float32` in 24 of the 26 photos even with `keep_on_device: false` on OSOR and the refiner, fits beside them in the `float16` `quality` uses, and in `bfloat16` removes a window from an empty room and leaves OSOR drawing a dog in another photo | 1.2–1.4s |
| LLMDet large | none, not added | Apache-2.0 | Yes | scores furniture highest, but boxes ceiling lights as `floor lamp` and kitchen cabinets as `shelf` | 1.3–1.5s |
| OWLv2 large | none, not added | Apache-2.0 | Yes | scores furniture lower than the others and boxes it several times over | 1.1s |
| OmDet-Turbo Swin-T, the only size on Hugging Face in transformers' format | none, not added | Apache-2.0 | Yes | in the photos `e2e.sh` sends, leaves out the kitchen cabinets MM-Grounding-DINO base boxes as `shelf`, but boxes ceiling lights as `floor lamp` in 2 and a dining table only in part | 0.04s |
| Florence-2 large, given one phrase at a time | none, not added | MIT | Yes | in the photos `e2e.sh` sends, boxes every phrase in every photo whether present or not, such as an air conditioner as `floor lamp` and `tv stand`, with no score to drop them by | 1.7–1.8s |
| YOLOE-26 L | none, not added | AGPL-3.0 | Yes, within AGPL-3.0's terms | boxes no table, tv stand or floor lamp in the photos `e2e.sh` sends, and a plant in only one, even keeping boxes scored down to 0.1 | 0.01–0.02s |
| Qwen3-VL 4B Instruct, asked for every phrase in one prompt | none, not added | Apache-2.0 | Yes | boxes furniture as tightly as MM-Grounding-DINO base and leaves out kitchen cabinets, but boxes ceiling lights or an air conditioner as `floor lamp` in 5 of the photos `e2e.sh` sends, with no score to drop them by | 19–33s |
| Rex-Omni, run with the transformers 4.51.3 it pins, as 5.19 fails on its tokens | none, not added | IDEA License 1.0, fine-tuned from Qwen2.5-VL 3B under the Qwen Research License | No | in the photos `e2e.sh` sends, boxes ceiling lights as `floor lamp`, kitchen cabinets and a range hood as `shelf` and a kitchen island as `tv stand` | 6.0–8.9s |
| OV-DINO Swin-T | none, not added | Apache-2.0 | Yes | not compared, as it needs Detectron2 built against PyTorch 1.13.1 and CUDA 11.6 | |
| DINO-X and Grounding DINO 1.5 Pro | none, not added | Not released, served on DeepDataSpace given an API token | Not checked | not compared | |
| SAM 3 in `bfloat16` | `sam3` | SAM License, gated on Hugging Face | Yes, within the license's use restrictions | removes the targets better than MM-Grounding-DINO base in 8 photos and worse in 11, keeping the kitchen counters and pendant lights MM-Grounding-DINO base removes in 3 and 2, but leaves floor lamps, plants and TV stands, and leaves OSOR drawing furniture such as a bathtub in 6, while given `furniture` alone it masks 28–57% of what the listed phrases mask in the photos `e2e.sh` sends, where MM-Grounding-DINO base masks 0–31% | 0.7s |

### Text locator comparison

Masking text in the photos `e2e.sh` sends, and in copies of them with three lines of Japanese and English drawn over each, on the same GPU gave the results below.

| Model | `type` | Weights license | Commercial use | Result | Time per photo |
|-------|--------|-----------------|----------------|--------|----------------|
| CRAFT | `craft` | MIT | Yes | masks every drawn line glyph by glyph and the lettering of the logo in one photo, and nothing else | 0.09–0.13s |
| PP-OCRv6 small det | none, not added | Apache-2.0 | Yes | masks every drawn line and the lettering of the logo as boxes, but also a small box in a kitchen | 0.02–0.04s |
| PP-OCRv6 medium det | none, not added | Apache-2.0 | Yes | masks every drawn line and the lettering of the logo as boxes, but also a dining set | 0.06–0.11s |
| PP-OCRv5 server det | none, not added | Apache-2.0 | Yes | masks every drawn line and the lettering of the logo as boxes, but also a kitchen island and a ventilation grille | 0.04–0.05s |
| Florence-2 large with `<OCR_WITH_REGION>` | none, not added | MIT | Yes | masks the lettering of the logo, but leaves out red Japanese lines drawn over 2 photos and masks small spots without text in 4 | 0.16–0.55s |
| docTR DBNet ResNet-50 | none, not added | Apache-2.0 | Yes | leaves out most of the red Japanese lines drawn over 2 photos, and masks the logo's mark and a ventilation grille | 0.04–0.06s |

### Refiner comparison

Redrawing what OSOR changed in the same photos on the same GPU gave the results below.

| Model | `type` | Weights license | Commercial use | Result | Time per refinement |
|-------|--------|-----------------|----------------|--------|---------------------|
| SDXL-Inpainting | `sdxl-inpainting` | CreativeML Open RAIL++-M | Yes, within the license's use restrictions | restores the floor grain OSOR smears, but draws objects on floors given `a photo of a room` in place of the `quality` pipeline's prompt | 8–10s |
| big-lama | none, not added | Apache-2.0 | Yes | draws vertical stripes | |
| SDXL-Inpainting with ControlNet tile | none, not added | Apache-2.0, used with SDXL-Inpainting under CreativeML Open RAIL++-M | Yes, within CreativeML Open RAIL++-M's use restrictions | draws outlets as SDXL-Inpainting does at a conditioning scale of 0.5, and at 1.0, with `strength` up to 0.8, draws no outlets but keeps the remnant of a vase, the lines of a rug and the smeared floor OSOR leaves | 10.0–10.3s |
| SDXL-Inpainting with IP-Adapter Plus ViT-H given the area it redraws | none, not added | Apache-2.0, used with SDXL-Inpainting under CreativeML Open RAIL++-M | Yes, within CreativeML Open RAIL++-M's use restrictions | draws outlets, switches, shelves or dark objects on floors in 4 of the 6 photos `e2e.sh` sends at scales of 0.5 and 0.8 | 7.4–7.6s |
| SDXL with ControlNet Union ProMax tile, run on SDXL's 4-channel UNet, as diffusers' `StableDiffusionXLControlNetUnionInpaintPipeline` rejects the 9-channel SDXL-Inpainting UNet | none, not added | Apache-2.0, used with SDXL under CreativeML Open RAIL++-M | Yes, within CreativeML Open RAIL++-M's use restrictions | keeps what OSOR leaves as ControlNet tile does at a conditioning scale of 1.0, and draws outlets at a conditioning scale of 0 | 10.0–10.6s |

### Matting comparison

Matting the people `person.` locates through `grounding-dino-sam2` with Grounding DINO base and SAM 2.1 large in the photos `e2e.sh` sends to `/v1/background`, and in a photo of three women with long hair before a crowd, on the same GPU and laying them over a plain color, gave the results below.

| Model | `type` | Weights license | Commercial use | Result | Time per photo |
|-------|--------|-----------------|----------------|--------|----------------|
| SAM 2.1 large's mask as it is | none, without `matting` | Apache-2.0 | Yes | cuts hair along a hard edge, leaves the plain color through an earphone and a stray patch beside a hand | |
| ViTMatte small Distinctions-646 | none, not added | Apache-2.0 | Yes | mattes hair as ViTMatte base does | 0.40–1.32s |
| ViTMatte base Distinctions-646 | `vitmatte` | Apache-2.0 | Yes | keeps strands of hair semi-transparent and closes the holes SAM 2.1 leaves, keeping only what `person.` locates | 0.51–1.75s |
| ViTMatte base Composition-1k | none, not added | Apache-2.0 | Yes | mattes hair as ViTMatte base Distinctions-646 does, whose weights alone come as safetensors | |
| BiRefNet-matting | none, not added | MIT | Yes | softens hair over a wider band and keeps a microphone, a basket and a snowboard `person.` leaves out, but in the crowd keeps parts of the people behind and a semi-transparent ghost of one of them, with no prompt to choose what it keeps | 0.21s at 1024px after the first photo |
| BRIA RMBG-2.0 | none, not added | CC BY-NC 4.0 | No, without an agreement with BRIA | not compared | |

ViTMatte left hair alike with `trimap_width` 10 and 20 on the photos 1920 pixels wide, while 40 let it take in a bench behind the women and turned part of a flower crown semi-transparent, and laying the people over a dark color without the foreground estimation left the sky and the sea of their photo as a bright fringe around hair and shoulders.
Sending the photos `e2e.sh` sends to `/v1/background` through `qwen-image-edit` took 62–66s per photo, where ViTMatte kept the strands of hair that `qwen-image-edit` without `matting` cut along a hard edge in 58–61s, and both drew the shadows of the vendor and the snowboarder on the new ground, but kept the distant skiers `person.` locates floating over the beach and left the vendor sitting in the air where the wall he sat on was redrawn.

### Harmonizer comparison

Adjusting the people matted as above to the backgrounds `qwen-image-edit` drew for them in the photos `e2e.sh` sends to `/v1/background`, among them the old woman before a beach at sunset, and in the couple's drawn again before a city street at night, on the same GPU gave the results below.

| Model | `type` | Weights license | Commercial use | Result | Time per photo |
|-------|--------|-----------------|----------------|--------|----------------|
| Mean and standard deviation in Lab matched halfway to the background's | none, not added | No weights | Yes | washes the old woman out before the bright office and darkens the couple's skin before the garden, as clothes and backgrounds differ in more than light | |
| PCT-Net ViT | `pct-net` | MPL-2.0 | Yes | warms the old woman before the sunset and dims her before the office without redrawing any detail, but leaves the light falling on her from where it fell | 0.05–0.14s |
| IC-Light fbc with Realistic Vision v5.1, taken as it draws | none, not added | CreativeML Open RAIL-M | Yes, within the license's use restrictions | lights the old woman from the window and the sunset, but redraws faces, breaking those of the couple | 11.5–16.3s |
| IC-Light fbc with Realistic Vision v5.1, taking the smooth change in each color it draws | none, not added | CreativeML Open RAIL-M | Yes, within the license's use restrictions | keeps detail while lighting from the same side, but casts the snowboarder blue and the old woman red before the office | 11.5–16.3s |
| IC-Light fbc with Realistic Vision v5.1, taking the smooth change in luminance it draws | `ic-light` | CreativeML Open RAIL-M | Yes, within the license's use restrictions | keeps detail and colors while shading the side away from the window and the sunset, but alone leaves the old woman gray before the sunset | 11.3–14.8s |
| IC-Light V2 | none, not added | Not released, served on Hugging Face Spaces | Not checked | not compared | |
| Harmonizer | none, not added | CC BY-NC-SA 4.0 | No | not compared | |
| DucoNet | none, not added | None given | No | not compared | |
| INR-Harmonization | none, not added | Apache-2.0 | Yes | not compared | |

Running both in either order on the same people gave the results below.

| `harmonizers` | Result | Time per request through `qwen-image-edit` |
|---------------|--------|--------------------------------------------|
| `pct-net`, then `ic-light`, as `config.yaml` takes | warms and shades the old woman before the sunset, but darkens the snowboarder before the sunny beach and the vendor | 71–75s against 62–66s without harmonizers, where `ic-light` off `DEVICE` beside the transformer of `qwen-image-edit` made PyTorch free its cache and retry allocations it first failed |
| `ic-light`, then `pct-net` | leaves the snowboarder and the vendor brighter, but flattens the shading `ic-light` draws on the old woman before the sunset and the office | |

### Upscaler comparison

Enlarging the photos `e2e.sh` sends four times, after shrinking them to a quarter, on the same GPU gave the results below, where the PSNR against the photos is 28.59dB for Lanczos, and enlarging the photos twice as they are showed how each draws the rugs in them.

| Model | `type` | Weights license | Commercial use | Result | Time per photo |
|-------|--------|-----------------|----------------|--------|----------------|
| SwinIR-L trained on real-world degradations | `swinir` | Apache-2.0 | Yes | draws chair legs and the buildings outside windows sharpest, but draws streaks over rugs, at a PSNR of 27.99dB | 2.2–5.4s |
| BSRGAN | none, not added | Apache-2.0 | Yes | draws edges softer than SwinIR-L, at a PSNR of 28.19dB | 0.6–1.1s |
| Real-ESRGAN x4plus | none, not added | BSD-3-Clause | Yes | draws edges softer than SwinIR-L, at a PSNR of 27.80dB | 0.5–1.5s |
| Real-ESRGAN general x4v3 | `real-esrgan` with `denoise_strength: 1` | BSD-3-Clause | Yes | draws edges softer than SwinIR-L and paints rugs flat, at a PSNR of 27.29dB | 0.03–0.09s |
| Real-ESRGAN general x4v3 weighed against its weights trained without denoising | `real-esrgan` | BSD-3-Clause | Yes | keeps more of the weave of rugs the lower `denoise_strength` is, at a PSNR of 27.70dB at 0.5, 27.95dB at 0.2 and 28.05dB at 0 | |
| SwinIR-M trained on bicubic downscaling, x4 and x2 | none, not added | Apache-2.0 | Yes | brings the 8px JPEG blocks out as a grid over walls and rugs, at a PSNR of 31.10dB, and 36.94dB shrinking to a half against 35.52dB for Lanczos | |

Enlarging the backgrounds `qwen-image-edit` draws at about a megapixel for `/v1/background` to the 1920 pixels of the photos `e2e.sh` sends, `real-esrgan` drew the edges of windows, roofs and fountains sharper than Lanczos, with the requests taking 62–66s against 59–64s.
Shrinking the photos `e2e.sh` sends and enlarging them back, Lanczos kept them closer to the photos by PSNR at every ratio and with more detail by the mean gradient below 1.5 times, from which `real-esrgan` kept more, though enlarging what `osor` draws at 512 pixels on the shorter side back to two of those photos, 1.67 and 1.88 times, with their furniture masked together, it smoothed the grain of the floor.

### Enhancer comparison

Sending photos on Wikimedia Commons through `decisions` gave the results below, where two other models compared each changed photo with its original side by side without being told which was which.

| `strength` | Of 112 photos meant to stay as they are: night scenes, dark wood, wallpaper, backlit rooms and low-key portraits, changed | Of 197 interiors of kitchens, bathrooms, living rooms, bedrooms and hotel rooms in Japan and Germany, unseen while tuning, changed | Preferred by both over the original | Original preferred by both |
|------------|------|------|------|------|
| 0 | 0 | 25 | 9 | 0 |
| 0.25 | 1, a backlit gallery whose shadows it opened | 23 | 13 | 2 |
| 0.5 | 1, the same gallery | 23 | 10 | 0 |
| 1 | 1, the same gallery | 24 | 13 | 4 |

Brightened at their full saturation, orange walls turned garish, at less of it salmon, and noisy dark photos and old film scans washed out, which `MIN_SATURATION`, `MAX_NOISE` and the question about reproductions keep from happening in the photos above.
Above 0 the comparing models preferred the original for up to four photos a run, nearly always of the kinds below whose corrections the Decisions API let through, and 0.5 was the only run from 0.25 to 0.75 where they preferred it for none, though those photos looked the same at 0.5625 and 0.625.

| Photos | Correction above 0 |
|--------|----------------------|
| A wall lit by a lamp | turned cyan by the white balance |
| Dark rooms showing grain | washed out, with windows blown out |

Clef-Flash answered whether a listing would rather show the correction with 0.09–0.14 where the Decisions API answered 0.51–0.81, so `clef` in `config.yaml` takes `prefer_threshold: 0.1`.
The runs below cover 64 interiors, 47 photos meant to stay as they are and the 10 photos `e2e.sh` sends with a blocky copy of one of them, compared by `gpt-5.4` and `gpt-4.1` as above, and no `harm_threshold` from 0.5 to 0.9 stopped a correction.
The two models agreed on few of the corrections, `gpt-5.4` alone preferring the original over the correction 4 to 3 at 0.1, against 4 to 1 for `decisions`, and `gpt-4.1` alone preferring the correction 8 to 2, against 3 to 4 for `decisions`.

| Enhancer | `prefer_threshold` | Of the 47 meant to stay, changed | Of the other 75, changed | Preferred by both over the original | Original preferred by both |
|----------|--------------------|------|------|------|------|
| `decisions` | 0.5 | 1, a church at night | 6 | 0 | 1, a dark kitchen whose grain it lifted |
| `clef` | 0.5 | 0 | 0 | 0 | 0 |
| `clef` | 0.12 | 2, a backlit sign and the same church | 6 | 1 | 0 |
| `clef` | 0.1 | 2, the same | 11 | 2 | 0 |
| `clef` | 0.08 | 2, the same | 15 | 2 | 1 |

## Development

`make download` fetches the models, the deformable attention kernel and the watermark font, and needs `HF_HUB_TOKEN` of an account granted access to SAM 3.

```sh
$ export HF_HUB_TOKEN=<value>
$ make download
$ make dev
```

### E2E Testing

`make e2e` sends photos of properties listed on LIFULL HOME'S to `/v1/masks` and `/v1/remove` through each pipeline, to `/v1/replace` and `/v1/insert`, and photos of people on Wikimedia Commons to `/v1/background`, through those that support them, to `/v1/upscale` through each upscaler, and to `/v1/enhance` through each enhancer, and writes what each returns to `results/{name}/`.
`PIPELINES` narrows them to the pipelines, upscalers and enhancers it names, separated by spaces, so that a GPU without room for `qwen-image-edit` can leave it out.
`ENDPOINTS` narrows the requests to the endpoints it names, separated by spaces, and skips serving a pipeline, upscaler or enhancer none of them reaches.

```sh
$ export OPENAI_API_KEY=<value>
$ make e2e
$ make e2e PIPELINES="cpu quality-sam3"
$ make e2e PIPELINES=qwen-image-edit ENDPOINTS="replace insert"
```

### LoRA Training

The targets below train a LoRA that keeps the refiner drawing plain walls and floors, working in `DATASET_DIRECTORY` (by default `~/.cache/meegeren-lora`).

| Target | Effect | Writes |
|--------|--------|--------|
| `make collect` | downloads photos of empty rooms from Wikimedia Commons and the Openverse API under licenses allowing commercial use and modification | `images/{sha256}.jpg`, `manifest.jsonl` |
| `make segment` | locates walls, floors and `EXCLUDED_LABELS` in each photo | `masks/{sha256}.png`, `regions.jsonl` |
| `make review` | lays each photo beside itself tinted by its mask, for rejecting photos | `review.html`, which downloads `rejected.txt` |
| `make judge` | asks the OpenAI Decisions API through `JUDGE_MODEL` whether each photo shows a room interior, whether its masks reach beyond walls and floors, and which of its tiles hold an object | `judgments.jsonl` |
| `make nhr` | picks room edits adding or removing one object from [NHR-Edit](https://huggingface.co/datasets/iitolstykh/NHR-Edit) and its [second part](https://huggingface.co/datasets/iitolstykh/NHR-Edit-part2), released for research use only, and asks the Decisions API whether each pair is usable | `nhr/pairs/{sample_id}/`, `nhr/pairs.jsonl`, `nhr/judgments.jsonl` |
| `make train` | trains a LoRA on the attention of SDXL-Inpainting's UNet over the photos and pairs the answers above accept, apart from those a `rejected.txt` in `DATASET_DIRECTORY` lists | `checkpoints/{step:06d}/pytorch_lora_weights.safetensors` |

`make collect`, `make segment`, `make judge` and `make nhr` skip what they already hold, so running them again continues where they stopped.
`make segment` and `make train` run on `DEVICE`, the CPU unless set to `cuda`.

```sh
$ make collect
$ make segment
$ make review
$ export OPENAI_API_KEY=<value>
$ make judge
$ make nhr
$ DEVICE=cuda make train
```

## Deployment

`cluster/manifests/meegeren` runs each pipeline and enhancer of `config.yaml` as its own Knative Service scaling to zero, plus one loading only `real-esrgan` as `UPSCALER`, and routes the requests a pod in the mesh sends to `http://meegeren.meegeren.svc.cluster.local` by the first row below that matches, reading the models from `/srv/nfs/.cache/meegeren` on the minikube host.

| Request | Pipeline |
|---------|----------|
| `/v1/upscale` | none, `real-esrgan` alone |
| `/v1/enhance?enhancer=clef` | none, `clef` alone |
| `/v1/enhance` | none, `decisions` alone |
| `?pipeline=gpt-image-edit` | `gpt-image` |
| `?pipeline={name}` | `{name}` |
| `/v1/insert`, `/v1/replace`, `/v1/background` | `qwen-image-edit` |
| anything else | `quality` |

`restyle=true` arrives in the form body, which the routing cannot read, so a request restyling with `gpt-image` names it in `pipeline`.

```sh
$ make download MODEL_DIRECTORY=/srv/nfs/.cache/meegeren
$ curl -sSf -F image=@photo.jpg -F 'prompt=sofa.' -F 'content=navy blue velvet' -F restyle=true -o restyled.png 'http://meegeren.meegeren.svc.cluster.local/v1/replace?pipeline=gpt-image'
```
