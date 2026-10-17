# Image for `make e2e-docker`: Go plus the tools Relay RR2PQ needs (ip for the
# veth pair, iptables for the paqet server rules).
FROM golang:1.27
RUN apt-get update \
    && apt-get install -y --no-install-recommends iproute2 iptables ca-certificates \
    && rm -rf /var/lib/apt/lists/*
