FROM python:3.11-slim-bullseye AS builder

RUN pip install --no-cache-dir poetry

WORKDIR /opt/builder
COPY pyproject.toml poetry.lock /opt/builder/

RUN poetry export --output requirements.txt --without-hashes

FROM python:3.11-slim-bullseye

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends curl && \
    curl -fsSL https://developer.download.nvidia.com/compute/cuda/repos/debian11/x86_64/cuda-keyring_1.1-1_all.deb -o /tmp/cuda-keyring_1.1-1_all.deb && \
    dpkg -i /tmp/cuda-keyring_1.1-1_all.deb && \
    rm /tmp/cuda-keyring_1.1-1_all.deb && \
    apt-get update -y && \
    apt-get install -y --no-install-recommends libcudnn8 libcublas-12-3

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

USER 65532

WORKDIR /home/nonroot

COPY --link --from=builder /opt/builder/requirements.txt /opt/whisper-worker/requirements.txt
RUN --mount=type=cache,target=/home/nonroot/.cache/pip pip install --upgrade --no-deps -r /opt/whisper-worker/requirements.txt

COPY whisper_worker /opt/whisper-worker/whisper_worker

WORKDIR /opt/whisper-worker

ENV PYTHONPATH /opt/whisper-worker

ENTRYPOINT ["python", "whisper_worker/main.py"]
