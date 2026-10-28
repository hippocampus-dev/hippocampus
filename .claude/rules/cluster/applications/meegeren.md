---
paths:
  - "cluster/applications/meegeren/**"
---

* Narrow `make e2e` with `PIPELINES` and `ENDPOINTS` to the pipelines, upscalers and endpoints the change reaches, reading which of them `e2e.sh` sends each endpoint to, and leave the full run to changes every request passes through - the full run serves every entry in `config.yaml` and sends each one every request `e2e.sh` assigns it, at the per-request times `### Inpainter comparison` in `cluster/applications/meegeren/README.md` gives
* Put a pipeline setting that decides how one endpoint runs its stages, rather than how a stage runs its model, under that endpoint's section of `Pipeline` in `cluster/applications/meegeren/meegeren/model.py` (such as `remove`) even where a stage reads it, rejecting it in a `Pipeline` validator for pipelines it does not apply to - in a stage's configuration it reads as a parameter of that stage's model, applying to every endpoint the stage serves
