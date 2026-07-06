#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
OUT_DIR=${1:-"$ROOT_DIR/build/boxlite"}
TMP_DIR=${BOXLITE_WORKDIR:-${AGENT_COMPOSE_BOXLITE_WORKDIR:-"$ROOT_DIR/build/.agent-compose-boxlite-src"}}
BOXLITE_VERSION=${BOXLITE_VERSION:-v0.9.7}
GO_TOOLCHAIN_ENV="$ROOT_DIR/scripts/with-go-toolchain.sh"
HOST_GOROOT=${HOST_GOROOT:-$("$GO_TOOLCHAIN_ENV" go env GOROOT)}
HOST_GOPROXY=${HOST_GOPROXY:-$("$GO_TOOLCHAIN_ENV" go env GOPROXY 2>/dev/null || echo 'https://goproxy.cn,direct')}
HOST_GOSUMDB=${HOST_GOSUMDB:-$("$GO_TOOLCHAIN_ENV" go env GOSUMDB 2>/dev/null || echo 'sum.golang.org')}

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

mkdir -p "$OUT_DIR/include" "$OUT_DIR/lib" "$OUT_DIR/runtime"
rm -rf "$TMP_DIR"
mkdir -p "$TMP_DIR"

docker run --rm \
  -v "$OUT_DIR:/out" \
  -v "$TMP_DIR:/work" \
  -v "$HOST_GOROOT:/usr/local/go:ro" \
  -e GOPROXY="$HOST_GOPROXY" \
  -e GOSUMDB="$HOST_GOSUMDB" \
  -e BOXLITE_BUILD_LIBKRUNFW=1 \
  docker.1ms.run/library/rust:1.88-bookworm \
  bash -lc '
    set -euo pipefail
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      build-essential git curl wget file pkg-config patchelf libssl-dev \
      musl-tools python3 python3-pip python3-venv llvm libclang-dev \
      flex bison bc libelf-dev python3-pyelftools meson ninja-build \
      libcap-dev protobuf-compiler ca-certificates xz-utils
    export PATH=/usr/local/go/bin:/root/.cargo/bin:$PATH
    git clone --branch '"$BOXLITE_VERSION"' --depth 1 https://github.com/boxlite-ai/boxlite.git /work/boxlite
    cd /work/boxlite
    git submodule update --init --recursive --depth 1
    rustup target add x86_64-unknown-linux-musl
    cargo build --release -p boxlite-c
    bash scripts/build/build-guest.sh --profile release
    bash scripts/build/build-runtime.sh --profile release --dest-dir /out/runtime
    cp sdks/c/include/boxlite.h /out/include/boxlite.h
    cp -a target/release/libboxlite.* /out/lib/
  '

echo "Prepared BoxLite C SDK and runtime at $OUT_DIR"
