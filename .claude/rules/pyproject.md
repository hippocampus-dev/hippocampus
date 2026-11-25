---
paths:
  - "**/pyproject.toml"
---

* Put a `.python-version` written by `uv python pin {major}.{minor}` beside every `pyproject.toml` that declares `requires-python`, at the {major}.{minor} of its `requires-python` lower bound - `requires-python` only bounds the range, so where nothing in it is installed uv takes the newest managed interpreter, which is what `bin/colabofy.sh` meets on Colab since it runs uv with `UV_MANAGED_PYTHON="1"` and ships only the directory it runs from, and `pydantic-core` 2.41.5 then fails to build from source on 3.15
