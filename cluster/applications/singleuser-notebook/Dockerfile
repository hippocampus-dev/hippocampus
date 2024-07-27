# syntax=docker/dockerfile:1.4

FROM quay.io/jupyter/scipy-notebook:hub-4.0.2

RUN --mount=type=cache,target=/home/jovyan/.cache/pip \
    pip install jupyterhub==4.0.2 openai && \
    conda install -c conda-forge -y jupyterlab-lsp==5.0.1 python-lsp-server==1.9.0
