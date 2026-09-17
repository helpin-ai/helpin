#!/bin/bash
# Build the Helpin Chat Widget
# Produces a minified widget bundle and the lazy-loaded emoji data bundle.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SRC="$SCRIPT_DIR/src/helpin-widget.js"
EMOJI_SRC="$SCRIPT_DIR/src/emoji-data.js"
DIST_DIR="$SCRIPT_DIR/dist"
OUT="$DIST_DIR/helpin-widget.min.js"
EMOJI_OUT="$DIST_DIR/emoji-data.js"

mkdir -p "$DIST_DIR"

# Prefer a locally installed terser. Do not shell out through npx because it
# may block while trying to download packages.
TERSER_BIN=""
if command -v terser >/dev/null 2>&1; then
  TERSER_BIN="$(command -v terser)"
elif [ -x "$SCRIPT_DIR/node_modules/.bin/terser" ]; then
  TERSER_BIN="$SCRIPT_DIR/node_modules/.bin/terser"
elif [ -x "$SCRIPT_DIR/../node_modules/.bin/terser" ]; then
  TERSER_BIN="$SCRIPT_DIR/../node_modules/.bin/terser"
fi

if [ -n "$TERSER_BIN" ]; then
  echo "Minifying with terser..."
  "$TERSER_BIN" "$SRC" \
    --compress passes=2,drop_console=true \
    --mangle \
    --output "$OUT" \
    --comments false
else
  echo "terser not found, copying source as-is..."
  cp "$SRC" "$OUT"
fi

cp "$EMOJI_SRC" "$EMOJI_OUT"
cp "$SCRIPT_DIR/LICENSE" "$SCRIPT_DIR/NOTICE" "$DIST_DIR/"

SIZE=$(wc -c < "$OUT")
GZIP_SIZE=$(gzip -c "$OUT" | wc -c)

echo ""
echo "Built: $OUT"
echo "  Raw:  ${SIZE} bytes ($(( SIZE / 1024 )) KB)"
echo "  Gzip: ${GZIP_SIZE} bytes ($(( GZIP_SIZE / 1024 )) KB)"
echo "Built: $EMOJI_OUT"
echo ""
echo "Done."
