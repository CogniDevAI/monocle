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

ASSET="monocle_${OS}_${ARCH}.tar.gz"

# Si VERSION está seteada, descargamos esa específica.
# Si no, usamos la URL "latest/download" que redirige al asset de la última
# release sin tocar la API de GitHub (que tiene rate limit de 60/h por IP).
if [ -n "$VERSION" ]; then
  URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET"
  VERSION_LABEL="$VERSION"
else
  URL="https://github.com/$REPO/releases/latest/download/$ASSET"
  VERSION_LABEL="latest"
fi

mkdir -p "$INSTALL_DIR"
TARGET="$INSTALL_DIR/$BINARY"

# Si el destino ya es un directorio (por una instalación previa rota),
# fallar con un mensaje claro en vez de instalar mal en silencio.
if [ -d "$TARGET" ]; then
  echo "✗ $TARGET es un directorio — probablemente quedó así por una instalación rota."
  echo "  Eliminalo con: rm -rf \"$TARGET\""
  echo "  Luego volvé a correr el install."
  exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "▸ descargando $ASSET ($VERSION_LABEL)..."
curl -fsSL "$URL" -o "$tmp/$ASSET"
tar -xzf "$tmp/$ASSET" -C "$tmp"

# Removemos el archivo previo si existía para evitar issues de permisos
# o "Text file busy" si estás reinstalando sobre un binario corriendo.
rm -f "$TARGET"
mv "$tmp/$BINARY" "$TARGET"
chmod +x "$TARGET"

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
