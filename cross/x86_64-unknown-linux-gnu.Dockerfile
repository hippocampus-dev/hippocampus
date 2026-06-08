# syntax=docker/dockerfile:1.4

# Use old glibc because RUSTFLAGS="-C target-feature=+crt-static" is now unstable
# GNU C Library (GNU libc) stable release version 2.17
FROM ghcr.io/cross-rs/x86_64-unknown-linux-gnu:0.2.5-centos

ENV CLANG_VERSION=15.0.7
ENV PATH=/opt/rh/devtoolset-11/root/bin:$PATH

ENV PROTOBUF_VERSION=21.11

ENV BPFTOOL_VERSION=v7.2.0

RUN --mount=type=cache,target=/var/cache/yum --mount=type=cache,target=/root/.cache/pip \
    sed -ri 's/^mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-* && \
    sed -ri 's|^# ?baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-* && \
    yum update -y && \
    yum install -y curl unzip pkg-config elfutils-libelf-devel python3 centos-release-scl && \
    sed -ri 's/^mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-* && \
    sed -ri 's|^# ?baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-* && \
    yum install -y devtoolset-11-gcc devtoolset-11-gcc-c++ && \
    pip3 install ninja && \
    curl -fsSL https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOBUF_VERSION}/protoc-${PROTOBUF_VERSION}-linux-x86_64.zip -o /tmp/protoc.zip && \
    unzip -o /tmp/protoc.zip -d /usr/local && \
    rm /tmp/protoc.zip

RUN --mount=type=cache,target=/usr/local/src \
    [ -d /usr/local/src/llvm-project-${CLANG_VERSION} ] || git clone https://github.com/llvm/llvm-project.git -b llvmorg-${CLANG_VERSION} --single-branch --depth 1 /usr/local/src/llvm-project-${CLANG_VERSION} && \
    mkdir -p /usr/local/src/llvm-project-${CLANG_VERSION}/build && \
    cd /usr/local/src/llvm-project-${CLANG_VERSION}/build && \
    cmake -DLLVM_ENABLE_PROJECTS=clang -DCMAKE_BUILD_TYPE=Release -G "Ninja" -DCMAKE_MAKE_PROGRAM=ninja /usr/local/src/llvm-project-${CLANG_VERSION}/llvm && \
    ninja -j$(nproc) && \
    ninja install

RUN --mount=type=cache,target=/usr/local/src \
    [ -d /usr/local/src/bpftool-${BPFTOOL_VERSION} ] || git clone https://github.com/libbpf/bpftool.git --recurse-submodules -b ${BPFTOOL_VERSION} --single-branch --depth 1 /usr/local/src/bpftool-${BPFTOOL_VERSION} && \
    cd /usr/local/src/bpftool-${BPFTOOL_VERSION}/src && \
    make -j$(nproc) && \
    make install

RUN bpftool btf dump file /sys/kernel/btf/vmlinux format c > /usr/include/vmlinux.h
