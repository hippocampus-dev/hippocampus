# syntax=docker/dockerfile:1.4

FROM python:3.11-slim-bookworm AS builder

RUN --mount=type=cache,target=/root/.cache/pip pip install uv

WORKDIR /opt/builder
COPY slackug/pyproject.toml slackug/uv.lock /opt/builder/

RUN uv export --frozen --no-hashes --no-dev --format requirements-txt > requirements.txt

FROM python:3.11-slim-bookworm

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

COPY --link --from=builder /opt/builder/requirements.txt /opt/slackug/requirements.txt
RUN --mount=type=cache,target=/root/.cache/pip bash -c "pip install --upgrade --no-deps -r <(sed -r '/^-e /d' /opt/slackug/requirements.txt)"

WORKDIR /opt/slackug
# https://github.com/pypa/pip/issues/4995
RUN --mount=type=cache,target=/root/.cache/pip bash -c "pip install --upgrade --no-deps -r <(sed -r '/^-e /!d' /opt/slackug/requirements.txt)"

COPY slackug/main.py /opt/slackug/main.py
COPY slackug/slackug /opt/slackug/slackug

USER 65532

ENV PYTHONPATH="/opt/slackug"
ENV PYTHONUNBUFFERED=1
ENTRYPOINT ["python", "/opt/slackug/main.py"]
