# Install dependencies
apt-get -y update && apt-get -y upgrade && apt-get -y install wget curl vim git

# Install docker
wget -O install-docker.sh https://get.docker.com && \
  chmod +x install-docker.sh && \
  ./install-docker.sh

# Generate shadowsocks configurations
PASSWORD=$(openssl rand -base64 20) && echo '{
    "server": "0.0.0.0",
    "server_port": 1919,
    "password": "'"$PASSWORD"'",
    "method": "chacha20-ietf-poly1305"
}' > config.json

# Run shadowsocks server
docker run --name ssserver-rust \
  --restart always \
  -p 1919:1919/tcp \
  -p 1919:1919/udp \
  -v ./config.json:/etc/shadowsocks-rust/config.json \
  -dit ghcr.io/shadowsocks/ssserver-rust:latest

# Show shadowsocks configurations
cat config.json
