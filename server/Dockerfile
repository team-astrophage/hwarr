FROM python:3.12-slim

WORKDIR /app

RUN apt-get update \
 && apt-get install -y --no-install-recommends curl \
 && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Admin-dong GeoJSON for daily ranking (see services/admin_region.py).
# Overridable via build-arg if upstream publishes a newer version.
ARG GEOJSON_URL=https://raw.githubusercontent.com/vuski/admdongkor/master/ver20230701/HangJeongDong_ver20230701.geojson
RUN mkdir -p data \
 && curl -fsSL -o data/admin_dong.geojson "$GEOJSON_URL"

EXPOSE 8000

CMD ["uvicorn", "main:combined_app", "--host", "0.0.0.0", "--port", "8000"]
