# syntax=docker/dockerfile:1.4

FROM python:3.11-slim-bookworm AS builder

RUN --mount=type=cache,target=/root/.cache/pip pip install uv

WORKDIR /opt/builder
COPY pyproject.toml uv.lock /opt/builder/

RUN uv export --frozen --no-hashes --no-dev --format requirements-txt > requirements.txt

FROM python:3.11-slim-bookworm

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends gcc libc6-dev

COPY --link --from=builder /opt/builder/requirements.txt /opt/embedding-retrieval/requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip pip install --upgrade --no-deps -r /opt/embedding-retrieval/requirements.txt

COPY main.py /opt/embedding-retrieval/main.py
COPY embedding_retrieval /opt/embedding-retrieval/embedding_retrieval

USER 65532

ENV PYTHONPATH="/opt/embedding-retrieval"
ENV PYTHONUNBUFFERED=1
ENTRYPOINT ["python", "/opt/embedding-retrieval/main.py"]
