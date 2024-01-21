echo "Running 'install-xray-mac.sh' script..."

BASE=$( dirname -- "$0"; )
DIR="${BASE}/xray-macos-arm64"
FILE="${DIR}.zip"
rm -rf "$DIR";
mkdir -p "$DIR"
wget -qNc https://github.com/XTLS/Xray-core/releases/download/v1.8.7/Xray-macos-arm64-v8a.zip -O "$FILE"
unzip "$FILE" -d "$DIR"
echo "${FILE}"
rm -rf "${FILE}"

