# syntax=docker/dockerfile:1.4

FROM python:3.14-slim-bookworm AS builder

RUN --mount=type=cache,target=/root/.cache/pip pip install uv

WORKDIR /opt/builder
COPY pyproject.toml uv.lock /opt/builder/

ARG TORCH_BACKEND=cu130
RUN uv export --frozen --no-hashes --no-dev --format requirements-txt --emit-index-url --extra ${TORCH_BACKEND} > requirements.txt

FROM python:3.14-slim-bookworm
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends libgomp1

COPY --link --from=builder /opt/builder/requirements.txt /opt/meegeren/requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip pip install --upgrade --no-deps -r /opt/meegeren/requirements.txt

COPY main.py /opt/meegeren/main.py
COPY meegeren /opt/meegeren/meegeren
COPY config.yaml /etc/meegeren/config.yaml

USER 65532

ENV PYTHONPATH="/opt/meegeren"
ENV PYTHONUNBUFFERED=1
# torch rounds pinned memory up to a power of two, which took 29.2GiB for the 16.6GB text encoder of qwen-image-edit kept off the GPU
ENV PYTORCH_ALLOC_CONF=pinned_max_round_threshold_mb:0
ENTRYPOINT ["python", "/opt/meegeren/main.py"]
