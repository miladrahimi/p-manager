echo "Running 'install-xray.sh' script..."

BASE=$( dirname -- "$0"; )

URL=https://github.com/XTLS/Xray-core/releases/download/v1.8.6/Xray-macos-arm64-v8a.zip
DIR="${BASE}/xray-macos-arm64"
FILE="${DIR}.zip"
rm -rf "$DIR";
mkdir -p "$DIR"
wget -qNc $URL -O "$FILE"
unzip "$FILE" -d "$DIR"
rm -rf "$FILE"

URL=https://github.com/XTLS/Xray-core/releases/download/v1.8.6/Xray-linux-64.zip
DIR="${BASE}/xray-linux-64"
FILE="${DIR}.zip"
rm -rf "$DIR";
mkdir -p "$DIR"
wget -qNc $URL -O "$FILE"
unzip "$FILE" -d "$DIR"
rm -rf "$FILE"
