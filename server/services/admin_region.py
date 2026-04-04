"""Admin region resolver — maps (lat, lng) to "{시도} {구} {동}" label.

Loads a static admin-dong GeoJSON at startup and builds a Shapely STRtree
spatial index. Queries run in microseconds and require no external API.

Expected GeoJSON format (FeatureCollection):
- Each feature's `properties` contains a full name field that looks like
  "서울특별시 강남구 역삼1동" (or similar variants).
- Supported field names (in priority order): adm_nm, ADM_NM, EMD_KOR_NM,
  adm_nm_kor, name.

Label format: we return the full name "{시도} {구|군|시} {동|읍|면}".
"""

from __future__ import annotations

import json
import logging
from pathlib import Path

from shapely.geometry import Point, shape
from shapely.geometry.base import BaseGeometry
from shapely.strtree import STRtree

logger = logging.getLogger(__name__)

_NAME_FIELDS = ("adm_nm", "ADM_NM", "EMD_KOR_NM", "adm_nm_kor", "name")


def _extract_name(properties: dict) -> str | None:
    for field in _NAME_FIELDS:
        value = properties.get(field)
        if isinstance(value, str) and value.strip():
            return value.strip()
    return None


def _shorten(adm_nm: str) -> str:
    """'서울특별시 강남구 역삼1동' → '서울특별시 강남구 역삼1동'.

    Returns the full "{시도} {구|군|시} {동|읍|면}" name. Kept as a function
    (rather than inlined) so source-format variants can be normalized here
    later (e.g. trimming internal whitespace).
    """
    return " ".join(adm_nm.split())


class AdminRegionResolver:
    """Spatial index of admin-dong polygons for (lat, lng) → label lookup."""

    def __init__(
        self,
        geometries: list[BaseGeometry],
        labels: list[str],
    ) -> None:
        self._geometries = geometries
        self._labels = labels
        self._tree = STRtree(geometries)

    @property
    def region_count(self) -> int:
        return len(self._labels)

    def resolve(self, lat: float, lng: float) -> str | None:
        """Return short label for the point, or None if outside all polygons."""
        point = Point(lng, lat)  # GeoJSON uses (lng, lat) order
        candidate_idxs = self._tree.query(point)
        for idx in candidate_idxs:
            if self._geometries[idx].contains(point):
                return self._labels[idx]
        return None

    @classmethod
    def load_or_none(cls, path: Path) -> "AdminRegionResolver | None":
        """Build resolver from GeoJSON file. Returns None if file is missing."""
        if not path.exists():
            logger.warning(
                "admin region resolver disabled: GeoJSON not found at %s", path
            )
            return None

        try:
            with path.open("r", encoding="utf-8") as f:
                data = json.load(f)
        except (OSError, json.JSONDecodeError):
            logger.exception("failed to load admin GeoJSON from %s", path)
            return None

        features = data.get("features") or []
        geometries: list[BaseGeometry] = []
        labels: list[str] = []

        for feature in features:
            props = feature.get("properties") or {}
            raw_name = _extract_name(props)
            if not raw_name:
                continue
            geom_data = feature.get("geometry")
            if not geom_data:
                continue
            try:
                geom = shape(geom_data)
            except (ValueError, TypeError):
                continue
            if geom.is_empty:
                continue
            geometries.append(geom)
            labels.append(_shorten(raw_name))

        if not geometries:
            logger.warning(
                "admin region resolver loaded 0 features from %s", path
            )
            return None

        logger.info(
            "loaded %d admin regions from GeoJSON: %s", len(geometries), path
        )
        return cls(geometries, labels)
