#!/bin/bash

# Run the end-to-end test in a privileged Linux container, which is what Relay
# RR2PQ needs (paqet sends raw packets as root). The repos are bind-mounted, so
# P-Node must be checked out next to P-Manager (or set P_NODE_DIR).

set -e

ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")
NODE_DIR=$(realpath "${P_NODE_DIR:-$ROOT/../p-node}")

XRAY_VERSION="v26.3.27"
PAQET_VERSION="v1.0.0-alpha.21"

# The Linux amd64 binaries are tracked; other container architectures are
# downloaded into the (ignored) third_party directories on first use.
case "$(docker info --format '{{.Architecture}}')" in
    aarch64|arm64)
        XRAY_DIR="xray-linux-arm64"; XRAY_ASSET="Xray-linux-arm64-v8a.zip"
        PAQET_DIR="paqet-linux-arm64"; PAQET_ASSET="paqet-linux-arm64-${PAQET_VERSION}.tar.gz"
        ;;
    *)
        XRAY_DIR="xray-linux-64"; XRAY_ASSET="Xray-linux-64.zip"
        PAQET_DIR="paqet-linux-64"; PAQET_ASSET="paqet-linux-amd64-${PAQET_VERSION}.tar.gz"
        ;;
esac

fetch() {
    local repo="$1"
    local xray="$repo/third_party/$XRAY_DIR"
    if [ ! -x "$xray/xray" ]; then
        echo "Downloading Xray $XRAY_VERSION into $xray ..."
        rm -rf "$xray" && mkdir -p "$xray"
        curl -sL "https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}/${XRAY_ASSET}" -o "$xray.zip"
        unzip -q "$xray.zip" -d "$xray" && rm -f "$xray.zip"
    fi
    local paqet="$repo/third_party/$PAQET_DIR"
    if [ ! -x "$paqet/paqet" ]; then
        echo "Downloading paqet $PAQET_VERSION into $paqet ..."
        rm -rf "$paqet" && mkdir -p "$paqet"
        curl -sL "https://github.com/hanselime/paqet/releases/download/${PAQET_VERSION}/${PAQET_ASSET}" -o "$paqet.tgz"
        tar xzf "$paqet.tgz" -C "$paqet" && rm -f "$paqet.tgz"
        mv "$paqet"/paqet_linux_* "$paqet/paqet" && chmod +x "$paqet/paqet"
    fi
}
fetch "$ROOT"
fetch "$NODE_DIR"

docker build -q -t p-manager-e2e "$ROOT/e2e" >/dev/null

exec docker run --rm --privileged \
    -v "$ROOT:/work/p-manager" \
    -v "$NODE_DIR:/work/p-node" \
    -v p-manager-e2e-gomod:/go/pkg/mod \
    -v p-manager-e2e-gocache:/root/.cache/go-build \
    -w /work/p-manager \
    -e E2E_PQ=1 \
    -e P_NODE_DIR=/work/p-node \
    p-manager-e2e go test -tags e2e -count=1 -timeout 15m -v ./e2e/
