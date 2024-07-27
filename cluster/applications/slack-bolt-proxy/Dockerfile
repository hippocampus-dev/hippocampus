FROM python:3.11-slim-bookworm AS builder

RUN pip install --no-cache-dir poetry

WORKDIR /opt/builder
COPY pyproject.toml poetry.lock /opt/builder/

RUN poetry export --output requirements.txt --without-hashes

FROM python:3.11-slim-bookworm

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

USER 65532

WORKDIR /home/nonroot

COPY --link --from=builder /opt/builder/requirements.txt /opt/slack-bolt-proxy/requirements.txt
RUN --mount=type=cache,target=/home/nonroot/.cache/pip pip install --upgrade --no-deps -r /opt/slack-bolt-proxy/requirements.txt

COPY slack_bolt_proxy /opt/slack-bolt-proxy/slack_bolt_proxy

WORKDIR /opt/slack-bolt-proxy

ENV PYTHONPATH /opt/slack-bolt-proxy

ENTRYPOINT ["python", "slack_bolt_proxy/main.py"]
