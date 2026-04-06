#!/bin/bash
set -e

GEOJSON_PATH="data/admin_dong.geojson"
GEOJSON_URL="${GEOJSON_URL:-https://raw.githubusercontent.com/vuski/admdongkor/master/ver20230701/HangJeongDong_ver20230701.geojson}"

if [ ! -f "$GEOJSON_PATH" ]; then
    echo "Downloading admin GeoJSON..."
    mkdir -p data
    curl -fsSL -o "$GEOJSON_PATH" "$GEOJSON_URL"
    echo "Done."
fi

exec "$@"
