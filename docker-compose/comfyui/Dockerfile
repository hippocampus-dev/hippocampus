# syntax=docker/dockerfile:1.4

FROM python:3.11-slim-bookworm

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends git build-essential python3-dev libglfw3-dev libgles2-mesa-dev pkg-config ffmpeg

USER 65532

WORKDIR /home/nonroot

RUN git clone https://github.com/comfyanonymous/ComfyUI -b v0.3.43 --depth 1 /home/nonroot/ComfyUI

WORKDIR /home/nonroot/ComfyUI

RUN pip install --no-cache-dir -r requirements.txt xformers

RUN git clone https://github.com/ltdrdata/ComfyUI-Manager /home/nonroot/ComfyUI/custom_nodes/comfyui-manager

ENTRYPOINT ["python", "main.py"]
CMD ["--listen", "0.0.0.0", "--port", "8188"]
