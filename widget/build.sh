#!/bin/bash
# Build the Teampulse Chat Widget
# Produces a minified single-file bundle at dist/teampulse-widget.min.js

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SRC="$SCRIPT_DIR/src/teampulse-widget.js"
DIST_DIR="$SCRIPT_DIR/dist"
OUT="$DIST_DIR/teampulse-widget.min.js"

mkdir -p "$DIST_DIR"

# Check for terser (preferred minifier)
if command -v npx &>/dev/null && npx terser --version &>/dev/null 2>&1; then
  echo "Minifying with terser..."
  npx terser "$SRC" \
    --compress passes=2,drop_console=true \
    --mangle \
    --output "$OUT" \
    --comments false
else
  echo "terser not found, copying source as-is..."
  cp "$SRC" "$OUT"
fi

SIZE=$(wc -c < "$OUT")
GZIP_SIZE=$(gzip -c "$OUT" | wc -c)

echo ""
echo "Built: $OUT"
echo "  Raw:  ${SIZE} bytes ($(( SIZE / 1024 )) KB)"
echo "  Gzip: ${GZIP_SIZE} bytes ($(( GZIP_SIZE / 1024 )) KB)"
echo ""
echo "Done."
