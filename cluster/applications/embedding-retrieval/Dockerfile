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
    apt-get install -y --no-install-recommends gcc libc6-dev

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

USER 65532

WORKDIR /home/nonroot

COPY --link --from=builder /opt/builder/requirements.txt /opt/embedding-retrieval/requirements.txt
RUN --mount=type=cache,target=/home/nonroot/.cache/pip pip install --upgrade --no-deps -r /opt/embedding-retrieval/requirements.txt

COPY embedding_retrieval /opt/embedding-retrieval/embedding_retrieval

WORKDIR /opt/embedding-retrieval

ENV PYTHONPATH /opt/embedding-retrieval

ENTRYPOINT ["python", "embedding_retrieval/main.py"]
