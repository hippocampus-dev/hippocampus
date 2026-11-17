# syntax=docker/dockerfile:1.4

# scikit-image==0.21.0 in stable-diffusion-webui-forge's requirements_versions.txt publishes wheels only up to Python 3.11: https://pypi.org/project/scikit-image/0.21.0/#files
FROM python:3.11-slim-bookworm

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends git sudo libgoogle-perftools-dev ffmpeg libglfw3-dev libgles2-mesa-dev pkg-config libcairo2 libcairo2-dev build-essential xdg-utils bc

RUN echo "nonroot:!:0:0:99999:7:::" >> /etc/shadow
RUN echo "%nonroot ALL=(ALL:ALL) NOPASSWD: ALL" | EDITOR="tee -a" visudo

# webui.sh must not be launched as root
USER 65532

WORKDIR /home/nonroot

RUN git init /home/nonroot/stable-diffusion-webui-forge && \
    git -C /home/nonroot/stable-diffusion-webui-forge fetch --depth 1 https://github.com/lllyasviel/stable-diffusion-webui-forge dfdcbab685e57677014f05a3309b48cc87383167 && \
    git -C /home/nonroot/stable-diffusion-webui-forge checkout FETCH_HEAD

COPY prepare.py /home/nonroot/stable-diffusion-webui-forge/prepare.py

# The default openai/CLIP commit imports pkg_resources in setup.py, which setuptools removed in v82.0.0, while this one no longer does: https://setuptools.pypa.io/en/stable/history.html#v82-0-0
# Without a constraint the venv ends up with numpy 2 although requirements_versions.txt pins numpy==1.26.2, and scikit-image==0.21.0 cannot import against it
# webui.sh exits 0 even when prepare.py raises, so importing gradio and skimage from requirements_versions.txt is what fails the build
RUN echo "numpy==1.26.2" > /tmp/constraints.txt && \
    PIP_CONSTRAINT=/tmp/constraints.txt CLIP_PACKAGE=https://github.com/openai/CLIP/archive/ded190a052fdf4585bd685cee5bc96e0310d2c93.zip LAUNCH_SCRIPT=/home/nonroot/stable-diffusion-webui-forge/prepare.py bash /home/nonroot/stable-diffusion-webui-forge/webui.sh --xformers --skip-torch-cuda-test && \
    /home/nonroot/stable-diffusion-webui-forge/venv/bin/python -c "import gradio, skimage"

RUN git clone https://github.com/Zyin055/Config-Presets --depth 1 /home/nonroot/stable-diffusion-webui-forge/extensions/Config-Presets
RUN git clone https://github.com/DominikDoom/a1111-sd-webui-tagcomplete --depth 1 /home/nonroot/stable-diffusion-webui-forge/extensions/a1111-sd-webui-tagcomplete

ENTRYPOINT ["bash", "/home/nonroot/stable-diffusion-webui-forge/webui.sh", "--skip-prepare-environment"]
CMD ["--allow-code", "--enable-insecure-extension-access", "--api", "--listen"]
