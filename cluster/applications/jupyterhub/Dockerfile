# syntax=docker/dockerfile:1.4

FROM python:3.11-slim-bookworm AS builder

RUN --mount=type=cache,target=/root/.cache/pip pip install poetry poetry-plugin-export

WORKDIR /opt/builder
COPY pyproject.toml poetry.lock /opt/builder/

RUN poetry export --output requirements.txt

FROM python:3.11-slim-bookworm
LABEL org.opencontainers.image.source="https://github.com/hippocampus-dev/hippocampus"

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends libcurl4 libcurl4-openssl-dev libssl-dev gcc libc6-dev sqlite3

COPY --link --from=builder /opt/builder/requirements.txt /opt/jupyterhub/requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip pip install --upgrade --no-deps -r /opt/jupyterhub/requirements.txt

USER 65532

WORKDIR /srv/jupyterhub

ENV PYTHONPATH="/srv/jupyterhub"
ENV PYTHONUNBUFFERED=1
ENTRYPOINT ["python", "-m", "jupyterhub", "--config", "/usr/local/etc/jupyterhub/jupyterhub_config.py"]
