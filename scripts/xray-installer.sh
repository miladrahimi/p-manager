VER=v1.8.4
URL=https://github.com/XTLS/Xray-core/releases/download

OS=$(uname)
ARCH=$(uname -m)

if [ "$OS-$ARCH" == "Darwin-arm64" ]; then
  TARGET=Xray-macos-arm64-v8a
  DIR="xray-macos-arm64"
elif [ "$OS-$ARCH" == "Linux-x86_64" ]; then
  TARGET=Xray-linux-64
  DIR="xray-linux-amd64"
else
  echo "The platform/architecture '$OS'/'$ARCH' is not supported."
  exit 1
fi

cd "$(dirname -- "$0")/../third_party" || exit
wget -qc "${URL}/${VER}/${TARGET}.zip"
unzip -qq -o "${TARGET}.zip" -d "${DIR}"
exit 0
