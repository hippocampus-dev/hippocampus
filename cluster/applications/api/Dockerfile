# syntax=docker/dockerfile:1.4

FROM python:3.11-slim-bookworm AS builder

RUN pip install --no-cache-dir poetry

WORKDIR /opt/builder
COPY api/pyproject.toml api/poetry.lock /opt/builder/

RUN poetry export --output requirements.txt --without-hashes

FROM python:3.11-slim-bookworm

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends gcc libc6-dev fonts-noto-cjk libglib2.0-0 libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libdbus-1-3 libatspi2.0-0 libx11-6 libxcomposite1 libxdamage1 libxext6 libxfixes3 libxrandr2 libgbm1 libxcb1 libxkbcommon0 libpango-1.0-0 libcairo2 libasound2 libmagic-dev libgl1-mesa-dev poppler-utils tesseract-ocr tesseract-ocr-eng tesseract-ocr-jpn libreoffice

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

USER 65532

WORKDIR /home/nonroot

COPY --link --from=builder /opt/builder/requirements.txt /opt/api/requirements.txt
RUN bash -c "pip install --no-cache-dir --upgrade --no-deps -r <(sed '\| file:///|d' /opt/api/requirements.txt)"
RUN python -m nltk.downloader punkt averaged_perceptron_tagger

ENV PATH="/home/nonroot/.local/bin:${PATH}"
RUN playwright install chromium

# Dependencies
COPY embedding-retrieval /opt/embedding-retrieval
COPY packages/cortex /opt/packages/cortex

COPY --link --from=builder /opt/builder/requirements.txt /opt/api/requirements.txt
RUN --mount=type=cache,target=/home/nonroot/.cache/pip pip install --upgrade --no-deps -r /opt/api/requirements.txt

COPY api/api /opt/api/api

WORKDIR /opt/api

ENV PYTHONPATH /opt/api

ENTRYPOINT ["python", "api/main.py"]
