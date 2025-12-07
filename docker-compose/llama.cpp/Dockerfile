# syntax=docker/dockerfile:1.4

FROM python:3.11-slim-bullseye

RUN echo "nonroot:x:65532:65532::/home/nonroot:/usr/sbin/nologin" >> /etc/passwd
RUN echo "nonroot:x:65532:" >> /etc/group
RUN mkdir /home/nonroot && chown nonroot:nonroot /home/nonroot

RUN --mount=type=cache,target=/var/cache/apt/archives --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends curl git make gcc g++ libcurl4-openssl-dev && \
    curl -fsSL https://developer.download.nvidia.com/compute/cuda/repos/debian11/x86_64/cuda-keyring_1.1-1_all.deb -o /tmp/cuda-keyring_1.1-1_all.deb && \
    dpkg -i /tmp/cuda-keyring_1.1-1_all.deb && \
    rm /tmp/cuda-keyring_1.1-1_all.deb && \
    apt-get update -y && \
    apt-get install -y --no-install-recommends libcublas-dev-12-3 cuda-nvcc-12-3

ENV LIBRARY_PATH=/usr/local/cuda/lib64/stubs:$LIBRARY_PATH
ENV PATH=/usr/local/cuda/bin:$PATH

RUN --mount=type=cache,target=/usr/local/src --mount=type=cache,target=/root/.cache/pip \
    [ -d /usr/local/src/llama.cpp ] || git clone https://github.com/ggerganov/llama.cpp -b b2755 --single-branch --depth 1 /usr/local/src/llama.cpp && \
    cd /usr/local/src/llama.cpp \
    pip install --upgrade -r requirements.txt && \
    make clean && \
    make -j$(nproc) main && \
    make -j$(nproc) server && \
    make -j$(nproc) quantize && \
    make -j$(nproc) finetune && \
    mv main /usr/local/bin/llama && \
    mv server /usr/local/bin/llama-server && \
    mv quantize /usr/local/bin/llama-quantize && \
    mv finetune /usr/local/bin/llama-finetune && \
    make clean && \
    make -j$(nproc) CUDA_DOCKER_ARCH=all LLAMA_CUDA=1 main && \
    make -j$(nproc) CUDA_DOCKER_ARCH=all LLAMA_CUDA=1 server && \
    make -j$(nproc) CUDA_DOCKER_ARCH=all LLAMA_CUDA=1 quantize && \
    make -j$(nproc) CUDA_DOCKER_ARCH=all LLAMA_CUDA=1 finetune && \
    mv main /usr/local/bin/llama.cuda && \
    mv server /usr/local/bin/llama-server.cuda && \
    mv quantize /usr/local/bin/llama-quantize.cuda && \
    mv finetune /usr/local/bin/llama-finetune.cuda && \
    cp convert-*.py /usr/local/bin/

USER 65532

WORKDIR /home/nonroot/llama.cpp

ENTRYPOINT ["llama-server.cuda"]
CMD ["-ngl", "32"]
