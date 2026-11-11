---
paths:
  - "docker-compose/docker-compose.stable-diffusion-webui*.yaml"
  - "docker-compose/stable-diffusion-webui*/downloader/links.txt"
---

* Point a main service's `--ckpt` at a path its downloader's `links.txt` writes with `out=`, and change both together - stable-diffusion-webui and its Forge only log `Checkpoint in --ckpt argument not found` and load another model, so a checkpoint no downloader fetches fails nowhere on a fresh volume
