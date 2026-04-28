#!/usr/bin/env sh
# Monocle installer — uso:
#   curl -fsSL https://raw.githubusercontent.com/CogniDevAI/monocle/main/install.sh | sh
#
# Variables opcionales:
#   INSTALL_DIR   destino del binario (default: $HOME/.local/bin)
#   VERSION       versión específica (default: última release)
set -e

REPO="CogniDevAI/monocle"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
BINARY="monocle"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "arquitectura no soportada: $ARCH"; exit 1 ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) echo "sistema operativo no soportado: $OS"; exit 1 ;;
esac

if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' \
    | head -n1 \
    | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
fi

if [ -z "$VERSION" ]; then
  echo "no se pudo determinar la versión más reciente"
  exit 1
fi

ASSET="monocle_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET"

mkdir -p "$INSTALL_DIR"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "▸ descargando $ASSET ($VERSION)..."
curl -fsSL "$URL" -o "$tmp/$ASSET"
tar -xzf "$tmp/$ASSET" -C "$tmp"
mv "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

echo "✓ instalado en $INSTALL_DIR/$BINARY"

case ":$PATH:" in
  *:"$INSTALL_DIR":*) ;;
  *)
    echo
    echo "agregá $INSTALL_DIR a tu PATH:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    ;;
esac

echo
echo "ejecutá: monocle"
