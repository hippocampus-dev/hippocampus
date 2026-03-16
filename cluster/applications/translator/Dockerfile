# syntax=docker/dockerfile:1.4

FROM python:3.11-slim-bookworm AS builder

RUN --mount=type=cache,target=/root/.cache/pip pip install uv

WORKDIR /opt/builder
COPY translator/pyproject.toml translator/uv.lock /opt/builder/

RUN uv export --frozen --no-hashes --no-dev --format requirements-txt > requirements.txt

FROM python:3.11-slim-bookworm
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends git

COPY --link --from=builder /opt/builder/requirements.txt /opt/translator/requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip bash -c "pip install --upgrade --no-deps -r <(sed -r '/^-e /d' /opt/translator/requirements.txt)"

# Dependencies
COPY packages/cortex /opt/packages/cortex

WORKDIR /opt/translator
# https://github.com/pypa/pip/issues/4995
RUN --mount=type=cache,target=/root/.cache/pip bash -c "pip install --upgrade --no-deps -r <(sed -r '/^-e /!d' /opt/translator/requirements.txt)"

COPY translator/main.py /opt/translator/main.py
COPY translator/translator /opt/translator/translator

USER 65532

ENV PYTHONPATH="/opt/translator"
ENV PYTHONUNBUFFERED=1
ENTRYPOINT ["python", "/opt/translator/main.py"]
