# syntax=docker/dockerfile:1.4

FROM python:3.14-slim-bookworm AS builder

RUN --mount=type=cache,target=/root/.cache/pip pip install uv

WORKDIR /opt/builder
COPY pyproject.toml uv.lock /opt/builder/

RUN uv export --frozen --no-hashes --no-dev --format requirements-txt > requirements.txt

RUN --mount=type=cache,target=/var/cache/apt/archives \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get install -y --no-install-recommends g++ libc6-dev

RUN --mount=type=cache,target=/root/.cache/pip pip install --upgrade --no-deps -r requirements.txt

FROM python:3.14-slim-bookworm
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends libgomp1

COPY --link --from=builder /opt/builder/requirements.txt /opt/promphet/requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip pip install --upgrade --no-deps -r /opt/promphet/requirements.txt

COPY promphet/ /opt/promphet/promphet/
COPY main.py /opt/promphet/main.py

USER 65532

ENV PYTHONPATH="/opt/promphet"
ENV PYTHONUNBUFFERED=1
ENTRYPOINT ["python", "/opt/promphet/main.py"]
