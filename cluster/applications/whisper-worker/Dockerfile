# syntax=docker/dockerfile:1.4

FROM python:3.11-slim-bullseye AS builder

RUN --mount=type=cache,target=/root/.cache/pip pip install uv

WORKDIR /opt/builder
COPY pyproject.toml uv.lock /opt/builder/

RUN uv export --frozen --no-hashes --no-dev --format requirements-txt > requirements.txt

FROM python:3.11-slim-bullseye
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends curl && \
    curl -fsSL https://developer.download.nvidia.com/compute/cuda/repos/debian11/x86_64/cuda-keyring_1.1-1_all.deb -o /tmp/cuda-keyring_1.1-1_all.deb && \
    dpkg -i /tmp/cuda-keyring_1.1-1_all.deb && \
    rm /tmp/cuda-keyring_1.1-1_all.deb && \
    apt-get update -y && \
    apt-get install -y --no-install-recommends libcudnn8 libcublas-12-3

COPY --link --from=builder /opt/builder/requirements.txt /opt/whisper-worker/requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip pip install --upgrade --no-deps -r /opt/whisper-worker/requirements.txt

COPY main.py /opt/whisper-worker/main.py
COPY whisper_worker /opt/whisper-worker/whisper_worker

USER 65532

ENV PYTHONPATH="/opt/whisper-worker"
ENV PYTHONUNBUFFERED=1
ENTRYPOINT ["python", "/opt/whisper-worker/main.py"]
