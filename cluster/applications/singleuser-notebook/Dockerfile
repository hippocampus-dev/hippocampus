# syntax=docker/dockerfile:1.4

FROM quay.io/jupyter/scipy-notebook:hub-5.2.1

RUN --mount=type=cache,target=/home/jovyan/.cache/pip \
    --mount=type=cache,target=/home/jovyan/.cache/conda,uid=1000 \
    pip install jupyterhub==5.2.1 openai==1.69.0 ipyvuetify==1.11.0 nest-asyncio==1.6.0 pydub==0.25.1 pyannote.audio==3.3.1 && \
    conda install -c conda-forge -y jupyterlab-lsp==5.0.1 python-lsp-server==1.9.0
