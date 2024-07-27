# syntax=docker/dockerfile:1.4

FROM python:3.11-slim-bookworm

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends git sudo libgoogle-perftools-dev ffmpeg libglfw3-dev libgles2-mesa-dev pkg-config libcairo2 libcairo2-dev build-essential

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN echo "nonroot:!:0:0:99999:7:::" >> /etc/shadow
RUN echo "%nonroot ALL=(ALL:ALL) NOPASSWD: ALL" | EDITOR="tee -a" visudo

USER 65532

WORKDIR /home/nonroot

RUN git clone https://github.com/AUTOMATIC1111/stable-diffusion-webui -b v1.7.0 --single-branch --depth=1

WORKDIR /home/nonroot/stable-diffusion-webui

COPY prepare.py /home/nonroot/stable-diffusion-webui/prepare.py

RUN LAUNCH_SCRIPT=prepare.py bash webui.sh --xformers --skip-torch-cuda-test

RUN git clone https://github.com/wcde/sd-webui-refiner.git extensions/sd-webui-refiner --depth=1
RUN git clone https://github.com/Mikubill/sd-webui-controlnet.git extensions/sd-webui-controlnet --depth=1
RUN git clone https://github.com/Zyin055/Config-Presets extensions/Config-Presets --depth=1

COPY --chown=nonroot:nonroot extensions/Config-Presets/config-txt2img.json extensions/Config-Presets/config-txt2img.json
COPY --chown=nonroot:nonroot extensions/Config-Presets/config-txt2img.json extensions/Config-Presets/config-img2img.json

ENTRYPOINT ["bash", "webui.sh", "--xformers", "--skip-prepare-environment"]
CMD ["--allow-code", "--medvram", "--enable-insecure-extension-access", "--api", "--listen"]
