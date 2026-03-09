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

COPY --link --from=builder /opt/builder/requirements.txt /opt/slack-bolt-proxy/requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip pip install --upgrade --no-deps -r /opt/slack-bolt-proxy/requirements.txt

COPY main.py /opt/slack-bolt-proxy/main.py
COPY slack_bolt_proxy /opt/slack-bolt-proxy/slack_bolt_proxy

USER 65532

ENV PYTHONPATH="/opt/slack-bolt-proxy"
ENV PYTHONUNBUFFERED=1
ENTRYPOINT ["python", "/opt/slack-bolt-proxy/main.py"]
