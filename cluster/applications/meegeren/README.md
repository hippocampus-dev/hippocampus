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
  * [Development](#development)
    * [E2E Testing](#e2e-testing)
    * [LoRA Training](#lora-training)
  * [Deployment](#deployment)
<!-- TOC -->

meegeren is a FastAPI server that removes, replaces and inserts objects named in natural language in images, replaces the background behind them, removes text, and upscales images.

## Features

- [x] `POST /v1/remove` returns the image with its targets filled in as a PNG, or streams it as server-sent events shaped like the OpenAI image edit stream with `stream=true`
- [x] `POST /v1/replace` returns the image with the box around each target redrawn as `content` as a PNG, or with `restyle=true` each target itself redrawn in the material or color `content` names, and with `relight` the rest relit as lit by the light it describes coming in through the targets
- [x] `POST /v1/insert` returns the image with `content` drawn at `relation` (`on`, `in`, `left`, `right` or `front`) to the nearest target as a PNG
- [x] `POST /v1/background` returns the image with everything but the objects `prompt` or `mask` names redrawn as `content` behind them as a PNG
- [x] `POST /v1/masks` returns the mask `/v1/remove` would fill before OSOR narrows it, `redetect` locates objects again and `remove_additions` adds what each refinement changes most and the text found after it, for tuning the targets, `threshold`, `dilation` and `keep`, or for cutting out and blurring the targets, and with `restyle=true` the mask `/v1/replace` restyles
- [x] `POST /v1/upscale` returns the image enlarged `scale` times, up to and by default the upscaler's own scale, as a PNG
- [x] `watermark` draws text, such as a note that AI edited the image, on what `/v1/remove`, `/v1/replace`, `/v1/insert`, `/v1/background` and `/v1/upscale` return

## Usage

Every endpoint but `/v1/background` and `/v1/upscale` takes `image` and one or more of the targets below, widens them by `dilation` pixels (by default 15, or 0.25% of the longer side of `image` with `restyle=true`, and 0 for `mask`) to cover outlines and shadows, and leaves the objects `keep` names unchanged, except those `grounding-dino-sam2` scores higher for a phrase of `prompt`.

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

`/v1/remove`, `/v1/replace` and `/v1/insert` blend what the inpainter draws into the image over up to `feathering` pixels (by default 0) inside the edge of the area it draws in, and `/v1/remove` returns 400 for `feathering` with `osor` or `whole_image: true`, which draw past that area.
`/v1/replace`, `/v1/insert` and `/v1/background` need an inpainter of type `gpt-image` or `qwen-image-edit`, and return 501 with any other.
`restyle=true` takes `prompt` alone as its targets, as `qwen-image-edit` is told them by name in place of marking them, returning 400 with `text` or `mask` and on `/v1/remove` and `/v1/insert`, and in the photos `e2e.sh` sends `qwen-image-edit` reshaped two of six sofas, such as adding an arm, and left the part of a wall the object locator stopped short of, such as a band under the ceiling, in its old color.
`relight` applies only with `restyle=true`, returning 400 otherwise, and only with `qwen-image-edit`, returning 501 with `gpt-image`, which relit the whole image as a room of another layout, and has the inpainter redraw the whole restyled image as lit by it, taking onto all but the targets and what `keep` names only the change in each color smoother than 1/160 of the longer side, which in the photo `e2e.sh` sends with `qwen-image-edit` warmed the room around a window turned to a sunset while keeping its furniture and textures, casting a patch of evening sun on the wall beside it in two of three runs, but left the sheer curtains the window took in blotched.

```sh
$ curl -sSf -F image=@photo.jpg -F 'prompt=desk.' -F relation=on -F 'content=a potted plant' -o inserted.png http://127.0.0.1:8080/v1/insert
$ curl -sSf -F image=@photo.jpg -F 'prompt=sofa.' -F 'content=navy blue velvet' -F restyle=true -o restyled.png http://127.0.0.1:8080/v1/replace
$ curl -sSf -F image=@photo.jpg -F 'prompt=window.' -F 'content=a clear sky at sunset with warm orange light' -F restyle=true -F 'relight=warm orange evening sunlight' -o relit.png http://127.0.0.1:8080/v1/replace
```

`/v1/background` keeps the objects `prompt` names, together with the white area of `mask`, taking no `text`, `dilation` or `keep`, redraws the rest of `image` as `content`, and returns 422 when nothing matches or nothing is left to redraw, so what they hold or rest on, such as a basket or a snowboard, and what that carries, such as the snacks heaped on a basket, is redrawn unless `prompt` takes it in, such as `person. basket of snacks.` in place of `person. basket.`.

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

### Pipelines

`CONFIG_FILE` lists `pipelines`, each combining a model per stage, and `upscalers`, and `PIPELINE` and `UPSCALER` name the one of each to load, `make dev` setting them to `quality-fast` and `real-esrgan` unless given, while leaving either unset or empty makes the endpoints needing it return 501, and leaving both stops the server at startup.

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
| `matting` | `vitmatte` | decides for `/v1/background` how opaque the kept objects are within `trimap_width` pixels on each side of their located edge with ViTMatte, and frees the colors there of the old background by blur-fusion foreground estimation |
| `harmonizers` | `pct-net` | brings the colors of what `/v1/background` keeps to those of the new background with PCT-Net's pixel-wise color transforms |
| `harmonizers` | `ic-light` | relights what `/v1/background` keeps for the new background, described by `content` followed by its `added_prompt` and set against its `negative_prompt`, with IC-Light, redrawing its first result `highres_scale` times larger by `highres_strength` unless `highres_scale` is null, and taking only the smooth change in luminance it draws onto the original pixels |
| `upscalers` | `real-esrgan` | enlarges the image with Real-ESRGAN in one pass, weighing its weights against those trained without denoising by `denoise_strength` |
| `upscalers` | `swinir` | enlarges the image with SwinIR on tiles, drawing edges of small images sharper but slower than `real-esrgan`, and streaks over rugs in photos that already show their weave |

`upscaler` on a pipeline names the entry of `upscalers` that enlarges what the inpainter or the refiner draws 1.5 times smaller than the area it fills or more, the area for `/v1/remove` with `whole_image: true` being the whole image and that for `/v1/background` most of it, loading it once where `UPSCALER` names it as well.
`harmonizers` on a pipeline run in their order, each on what the one before returns, and none by default.
`redetect: true` makes `/v1/remove` remove the objects one at a time, nearest first, locating them again after each removal, up to `max_removals` times.
`loras` on `sdxl-inpainting` merges each LoRA file at `path` into `base_model` at startup, in order and scaled by its `weight`, and `timestep_spacing` replaces that of `base_model`'s scheduler, such as `trailing` for the distilled LoRAs of Hyper-SDXL and SDXL-Lightning, whose `steps` times `strength` is the number of steps they then take.
`remove_additions: true` in `remove` on a pipeline with `sdxl-inpainting` makes `/v1/remove` remove with the inpainter, region by region, what each refinement changes most and the text the text locator then finds within what was removed, as the letters SDXL-Inpainting drew over a logo OSOR left change it too little to count, and in the photos `e2e.sh` sends took out a potted plant, panels and a drain cover SDXL-Inpainting drew, but not what OSOR left without the refinement changing it.
`keep_on_device: false` on an upscaler, a stage other than `gpt-image` and `qwen-image-edit`, or the `text_encoder`, `transformer` or `vae` of `qwen-image-edit` keeps its models in CPU memory and moves them onto `DEVICE` only while they run, for stages that leave too little GPU memory for the others, while the default `true` keeps them on `DEVICE` and skips the transfer, such as about 1.3s per refinement for `sdxl-inpainting`.
`qwen-image-edit` moves each of those models onto `DEVICE` layer by layer, the text encoder and the transformer while running the previous layer, which took 60–79s per request with `keep_on_device: false` on the text encoder alone, on the transformer alone or on all three, at a peak GPU memory of 14.9GiB, 16.8GiB and 1.2GiB, where moving each model whole, as the inpainter comparison below ran, took 80s.
`grounding-dino-sam2` on CUDA runs the multi-scale deformable attention of Grounding DINO with the kernel `make download` fetches, which took MM-Grounding-DINO large 0.34–0.40s per photo against 0.69–0.77s for the implementation in transformers the comparisons below ran, and in `float16` kept every box a `float32` reference finds in the photos `e2e.sh` sends, of which that implementation lost one of 97.
`whole_image: true` in `remove` on a pipeline with `gpt-image` or `qwen-image-edit` makes `/v1/remove` send the whole image in place of the area around the targets and return what the model draws resized to the image size, leaving no seams along the targets that pasting only them back leaves, but letting the model redraw the rest of the image too, while `/v1/replace` and `/v1/insert` keep sending the area around the targets so that what `keep` names stays unchanged.
Requests override the fields below, validated as in `CONFIG_FILE`, and get 400 for a stage the pipeline lacks.

| Field | Overrides |
|-------|-----------|
| `threshold` | `object_locator`'s `threshold`, the score below which a detection is dropped |
| `refiner_prompt`, `refiner_negative_prompt`, `refiner_guidance_scale`, `refiner_strength`, `refiner_steps` | the `refiner` field of the same name on `/v1/remove` |
| `remove_additions` | `remove`'s `remove_additions` on `/v1/remove`, getting 400 without a `refiner` |

`dtype` on `real-esrgan` or a stage other than `big-lama` and `gpt-image` loads its models in `float32` by default, or in `float16` or `bfloat16`, other than `float16` on `qwen-image-edit`, while `big-lama` and `swinir` fail in either half precision.
`config.yaml` takes `float16` on every stage of every pipeline but `cpu`, and `bfloat16` on `qwen-image-edit`, which rejects `float16`, and on `ic-light`, which peaks lower in it, as the precision comparison below gives it results no worse than those of `float32` in about half the memory, which fits each pipeline on a 20GB GPU with every model on `DEVICE` but `qwen-image-edit`, whose text encoder and transformer weigh 16.6GB and 13.2GB and need `keep_on_device: false` on one of them there, and on the `ic-light` beside them.

### Precision comparison

Running each model alone in each `dtype` on an NVIDIA RTX 4000 SFF Ada (20GB) gave the peak GPU memory, the time and the agreement with `float32` below, where the object locators locate the 12 phrases `e2e.sh` names in the photos of the object locator comparison, CRAFT masks text in them, Real-ESRGAN enlarges them shrunk to a quarter, OSOR and SDXL-Inpainting fill the sofa of one photo, ViTMatte mattes the people in the photos `e2e.sh` sends to `/v1/background` and estimates their colors, and PCT-Net and IC-Light adjust the people of the harmonizer comparison, comparing masks by IoU and whole images, opacities and people by PSNR as the median over the photos and, in parentheses, the worst.

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

## Development

`make download` fetches the models, the deformable attention kernel and the watermark font, and needs `HF_HUB_TOKEN` of an account granted access to SAM 3.

```sh
$ export HF_HUB_TOKEN=<value>
$ make download
$ make dev
```

### E2E Testing

`make e2e` sends photos of properties listed on LIFULL HOME'S to `/v1/masks` and `/v1/remove` through each pipeline, to `/v1/replace` and `/v1/insert`, and photos of people on Wikimedia Commons to `/v1/background`, through those that support them, and to `/v1/upscale` through each upscaler, and writes what each returns to `results/{name}/`.
`PIPELINES` narrows them to the pipelines and upscalers it names, separated by spaces, so that a GPU without room for `qwen-image-edit` can leave it out.
`ENDPOINTS` narrows the requests to the endpoints it names, separated by spaces, and skips serving a pipeline or upscaler none of them reaches.

```sh
$ export OPENAI_API_KEY=<value>
$ make e2e
$ make e2e PIPELINES="cpu quality-sam3"
$ make e2e PIPELINES=qwen-image-edit ENDPOINTS="replace insert"
```

### LoRA Training

`make collect` downloads photos of empty rooms for training a LoRA that keeps the refiner drawing plain walls and floors, taking the JPEG photos under `Category:Empty rooms` on Wikimedia Commons and two levels of its subcategories under CC BY, CC BY-SA, CC0, the public domain or No restrictions, and those the Openverse API finds for each of `OPENVERSE_QUERIES` under licenses allowing commercial use and modification, into `DATASET_DIRECTORY` (by default `~/.cache/meegeren-lora`).
Each photo is stored as `images/{sha256}.jpg`, which drops photos uploaded to both sources, and described in `manifest.jsonl` by its page, author and license in the property names of schema.org's `ImageObject`, so running it again downloads only what is not yet listed there.
`make segment` then locates walls, floors and `EXCLUDED_LABELS` in each photo shrunk to 2048 pixels through the Grounding DINO base and SAM 2.1 tiny `make download` fetches, on `DEVICE`, the CPU unless set to `cuda`, writing `masks/{sha256}.png` with 1 for walls, 2 for floors drawn over walls and 3 for what is excluded drawn over both, and the fraction of the photo each covers to `regions.jsonl`, where a photo already listed is skipped.
In a photo of a seminar room under `Category:Empty rooms`, it left a light switch beside the door and a notice inside the wall mask, though `switch` and `sign` are among `EXCLUDED_LABELS`.
`make review` writes `review.html`, which lays each photo beside itself tinted by its mask from the least wall and floor up and downloads `rejected.txt` listing the photos clicked from its button, while `make review` marks again the photos a `rejected.txt` placed in `DATASET_DIRECTORY` lists.

`make judge` then asks the OpenAI Decisions API through `JUDGE_MODEL`, for each photo with at least `MINIMUM_REGION_FRACTION` of wall and floor and `MINIMUM_PHOTO_SIDE` pixels on its shorter side, whether it is a color photo of a room interior and whether its wall and floor masks reach beyond walls and floors, and for each tile of such a photo's `TILE_GRID` by `TILE_GRID` grid holding at least `MINIMUM_TILE_FRACTION` of wall and floor, shown with half a tile around it, whether an object or marking lies on the wall or floor inside it, appending the probabilities and responses to `judgments.jsonl`, where an answer for the same photo, tile, questions, model and mask is not asked again.
`make nhr` reads the text columns of [NHR-Edit](https://huggingface.co/datasets/iitolstykh/NHR-Edit) and its [second part](https://huggingface.co/datasets/iitolstykh/NHR-Edit-part2), synthetic edits under Apache-2.0 released for research use only, picks the forward edits adding or removing one object in `NHR_STYLES` whose prompt matches `NHR_ROOM_PATTERN`, reads the images of the row groups holding the most of them, 100 rows and about 230MB each, until they hold `NHR_PAIRS`, keeping the original PNGs of the picked rows, masks where the image with the object and the one without differ by more than `NHR_DIFFERENCE_THRESHOLD`, and asks the Decisions API for each pair masking between `NHR_MINIMUM_MASK_FRACTION` and `NHR_MAXIMUM_MASK_FRACTION` whether it shows a room, whether the object stood on the floor or a wall and whether nothing of it is left after its removal, writing each pair to `nhr/pairs/{sample_id}/` as `object.png`, `background.png` and `mask.png`, listed in `nhr/pairs.jsonl`, and the answers to `nhr/judgments.jsonl`, where a row group whose picked rows are all listed is not read again, a changed `NHR_DIFFERENCE_THRESHOLD` masks the pairs listed again, and an answer for the same pair, mask, questions and model is not asked again.
Reading 5 row groups 4 at a time took about 40 seconds for 54 pairs, while the 254 and 744 row groups holding the most picked rows hold 2,000 and 5,000 of them.

```sh
$ make collect
$ make segment
$ make review
$ export OPENAI_API_KEY=<value>
$ make judge
$ make nhr
```

`make train` then trains a LoRA on the attention of SDXL-Inpainting's UNet over the photos `judgments.jsonl` finds an interior at `INTERIOR_THRESHOLD` or above with masks reaching beyond walls and floors below `MISPLACED_THRESHOLD`, apart from those a `rejected.txt` in `DATASET_DIRECTORY` lists, masking ellipses and rectangles inside their walls and floors within the tiles it finds an object in below `OBJECT_THRESHOLD` and noising them only as far as the refiner's default `strength` and `steps` do, from the photo itself or, in `DEGRADED_PROBABILITY` of the samples, from it blurred and recolored inside the mask, and in `NHR_FRACTION` of the samples instead over the pairs `make nhr` finds a room, a floor or wall and nothing left at `PAIR_THRESHOLD` or above, masking where they differ and starting in `DEGRADED_PROBABILITY` of them from the image with the object, and writes `checkpoints/{step:06d}/pytorch_lora_weights.safetensors` with the settings beside it every `CHECKPOINT_STEPS` steps and at the last.
It runs on `DEVICE` as well, where two steps at `RESOLUTION=256` took a few seconds in `bfloat16`.

```sh
$ DEVICE=cuda make train
```

## Deployment

`cluster/manifests/meegeren` runs each pipeline of `config.yaml` as its own Knative Service scaling to zero, plus one loading only `real-esrgan` as `UPSCALER`, and routes the requests a pod in the mesh sends to `http://meegeren.meegeren.svc.cluster.local` by the first row below that matches, reading the models from `/srv/nfs/.cache/meegeren` on the minikube host.

| Request | Pipeline |
|---------|----------|
| `/v1/upscale` | none, `real-esrgan` alone |
| `?pipeline=gpt-image-edit` | `gpt-image` |
| `?pipeline={name}` | `{name}` |
| `/v1/insert`, `/v1/replace`, `/v1/background` | `qwen-image-edit` |
| anything else | `quality` |

`restyle=true` arrives in the form body, which the routing cannot read, so a request restyling with `gpt-image` names it in `pipeline`.

```sh
$ make download MODEL_DIRECTORY=/srv/nfs/.cache/meegeren
$ curl -sSf -F image=@photo.jpg -F 'prompt=sofa.' -F 'content=navy blue velvet' -F restyle=true -o restyled.png 'http://meegeren.meegeren.svc.cluster.local/v1/replace?pipeline=gpt-image'
```
