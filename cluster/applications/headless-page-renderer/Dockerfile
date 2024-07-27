# syntax=docker/dockerfile:1.4

FROM python:3.11-slim-bookworm AS builder

RUN pip install --no-cache-dir poetry

WORKDIR /opt/builder
COPY pyproject.toml poetry.lock /opt/builder/

RUN poetry export --output requirements.txt --without-hashes

FROM python:3.11-slim-bookworm

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends fonts-noto-cjk libglib2.0-0 libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libdbus-1-3 libatspi2.0-0 libx11-6 libxcomposite1 libxdamage1 libxext6 libxfixes3 libxrandr2 libgbm1 libxcb1 libxkbcommon0 libpango-1.0-0 libcairo2 libasound2

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

USER 65532

WORKDIR /home/nonroot

COPY --link --from=builder /opt/builder/requirements.txt /opt/headless-page-renderer/requirements.txt
RUN bash -c "pip install --no-cache-dir --upgrade --no-deps -r <(sed '\| @ file:///|d' /opt/headless-page-renderer/requirements.txt)"

ENV PATH="/home/nonroot/.local/bin:${PATH}"
RUN playwright install chromium

COPY --link --from=builder /opt/builder/requirements.txt /opt/headless-page-renderer/requirements.txt
RUN --mount=type=cache,target=/home/nonroot/.cache/pip pip install --upgrade --no-deps -r /opt/headless-page-renderer/requirements.txt

COPY main.py /opt/headless-page-renderer/main.py

WORKDIR /opt/headless-page-renderer

ENV PYTHONPATH /opt/headless-page-renderer

ENTRYPOINT ["python", "main.py"]
