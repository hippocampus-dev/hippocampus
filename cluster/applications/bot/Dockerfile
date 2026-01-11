# syntax=docker/dockerfile:1.4

FROM python:3.11-slim-bookworm AS builder

RUN --mount=type=cache,target=/root/.cache/pip pip install uv

WORKDIR /opt/builder
COPY bot/pyproject.toml bot/uv.lock /opt/builder/

RUN uv export --frozen --no-hashes --no-dev --format requirements-txt > requirements.txt

FROM python:3.11-slim-bookworm
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends git gcc libc6-dev cmake fonts-noto-cjk libglib2.0-0 libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libdbus-1-3 libatspi2.0-0 libx11-6 libxcomposite1 libxdamage1 libxext6 libxfixes3 libxrandr2 libgbm1 libxcb1 libxkbcommon0 libpango-1.0-0 libcairo2 libasound2 libmagic-dev libgl1-mesa-dev poppler-utils tesseract-ocr tesseract-ocr-eng tesseract-ocr-jpn libreoffice

COPY --link --from=builder /opt/builder/requirements.txt /opt/bot/requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip bash -c "pip install --upgrade --no-deps -r <(sed -r '/^-e /d' /opt/bot/requirements.txt)"
RUN python -m nltk.downloader punkt averaged_perceptron_tagger

ENV PATH="/root/.local/bin:${PATH}"
RUN --mount=type=cache,target=/root/.cache/ms-playwright,sharing=locked playwright install chromium && cp -r /root/.cache/ms-playwright /usr/local/share/ms-playwright
ENV PLAYWRIGHT_BROWSERS_PATH="/usr/local/share/ms-playwright"

# Dependencies
COPY embedding-retrieval /opt/embedding-retrieval
COPY packages/cortex /opt/packages/cortex

WORKDIR /opt/bot
# https://github.com/pypa/pip/issues/4995
RUN --mount=type=cache,target=/root/.cache/pip bash -c "pip install --upgrade --no-deps -r <(sed -r '/^-e /!d' /opt/bot/requirements.txt)"

COPY bot/main.py /opt/bot/main.py
COPY bot/bot /opt/bot/bot

USER 65532

ENV PYTHONPATH="/opt/bot"
ENV PYTHONUNBUFFERED=1
ENTRYPOINT ["python", "/opt/bot/main.py"]
