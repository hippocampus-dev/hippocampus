# syntax=docker/dockerfile:1.4

FROM python:3.11-slim-bookworm AS builder

RUN --mount=type=cache,target=/root/.cache/pip pip install uv

WORKDIR /opt/builder
COPY pyproject.toml uv.lock /opt/builder/

RUN uv export --frozen --no-hashes --no-dev --format requirements-txt > requirements.txt

FROM python:3.11-slim-bookworm
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends fonts-noto-cjk libglib2.0-0 libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libdbus-1-3 libatspi2.0-0 libx11-6 libxcomposite1 libxdamage1 libxext6 libxfixes3 libxrandr2 libgbm1 libxcb1 libxkbcommon0 libpango-1.0-0 libcairo2 libasound2

COPY --link --from=builder /opt/builder/requirements.txt /opt/headless-page-renderer/requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip pip install --upgrade --no-deps -r /opt/headless-page-renderer/requirements.txt

ENV PATH="/root/.local/bin:${PATH}"
RUN --mount=type=cache,target=/root/.cache/ms-playwright,sharing=locked playwright install chromium && cp -r /root/.cache/ms-playwright /usr/local/share/ms-playwright
ENV PLAYWRIGHT_BROWSERS_PATH="/usr/local/share/ms-playwright"

COPY main.py /opt/headless-page-renderer/main.py

USER 65532

ENV PYTHONPATH="/opt/headless-page-renderer"
ENV PYTHONUNBUFFERED=1
ENTRYPOINT ["python", "/opt/headless-page-renderer/main.py"]
