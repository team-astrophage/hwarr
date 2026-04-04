#!/usr/bin/env python3
"""전국 동시 방화 시연 스크립트.

httpx로 REST API (POST /api/fire)를 호출하여 전국 20여 개 지점에
단계적으로 화재를 점화·확산시키는 데모용 스크립트.
서버가 자동으로 Socket.IO broadcast를 하므로 접속한 모든 클라이언트에 실시간 반영됨.

Usage:
    python scripts/demo_fire.py
    python scripts/demo_fire.py --url http://<서버IP>:8000
    python scripts/demo_fire.py --speed 0.1
    python scripts/demo_fire.py --phase 1
    python scripts/demo_fire.py --no-reset
"""

from __future__ import annotations

import argparse
import asyncio
import random
import sys
import time

import httpx

# ---------------------------------------------------------------------------
# 전국 21개 랜드마크 좌표
# ---------------------------------------------------------------------------

LOCATIONS: list[dict] = [
    # 서울
    {"id": "gwanghwamun", "name": "광화문광장", "lat": 37.5760, "lng": 126.9769},
    {"id": "gangnam", "name": "강남역", "lat": 37.4979, "lng": 127.0276},
    {"id": "hongdae", "name": "홍대입구", "lat": 37.5563, "lng": 126.9236},
    {"id": "yeouido", "name": "여의도공원", "lat": 37.5284, "lng": 126.9344},
    {"id": "jamsil", "name": "잠실종합운동장", "lat": 37.5153, "lng": 127.0728},
    {"id": "namsan", "name": "남산타워", "lat": 37.5512, "lng": 126.9882},
    {"id": "itaewon", "name": "이태원", "lat": 37.5345, "lng": 126.9946},
    {"id": "coex", "name": "코엑스", "lat": 37.5126, "lng": 127.0590},
    # 수도권
    {"id": "incheon_songdo", "name": "송도센트럴파크", "lat": 37.3925, "lng": 126.6632},
    {"id": "suwon_hwaseong", "name": "수원화성", "lat": 37.2870, "lng": 127.0095},
    # 광역시
    {"id": "busan_haeundae", "name": "해운대해수욕장", "lat": 35.1587, "lng": 129.1604},
    {"id": "daegu_dongseong", "name": "대구 동성로", "lat": 35.8691, "lng": 128.5958},
    {"id": "gwangju_chungjang", "name": "광주 충장로", "lat": 35.1488, "lng": 126.9156},
    {"id": "daejeon_dunsan", "name": "대전 둔산동", "lat": 36.3511, "lng": 127.3782},
    {"id": "ulsan_samsan", "name": "울산 삼산동", "lat": 35.5384, "lng": 129.3114},
    # 기타 주요 도시
    {"id": "jeju_hallasan", "name": "제주 한라산", "lat": 33.3617, "lng": 126.5292},
    {"id": "gyeongju_bulguksa", "name": "경주 불국사", "lat": 35.7900, "lng": 129.3322},
    {"id": "jeonju_hanok", "name": "전주 한옥마을", "lat": 35.8151, "lng": 127.1530},
    {"id": "chuncheon_myeong", "name": "춘천 명동거리", "lat": 37.8813, "lng": 127.7298},
    {"id": "sejong_gov", "name": "세종 정부청사", "lat": 36.5040, "lng": 127.0046},
    {"id": "pohang_yeongil", "name": "포항 영일대", "lat": 36.0561, "lng": 129.3760},
]

STAGE_LABELS = {
    0: "없음",
    1: "불씨 🔥",
    2: "모닥불 🔥🔥",
    3: "화재 🔥🔥🔥",
    4: "대화재 🔥🔥🔥🔥",
    5: "전소 🔥🔥🔥🔥🔥",
}

HOTSPOT_NAMES = [
    "광화문광장",
    "강남역",
    "해운대해수욕장",
    "대전 둔산동",
    "제주 한라산",
]


def _log(msg: str) -> None:
    ts = time.strftime("%H:%M:%S")
    print(f"[{ts}] {msg}", flush=True)


def _stage_bar(stage: int) -> str:
    return STAGE_LABELS.get(stage, f"stage={stage}")


async def _ignite(client: httpx.AsyncClient, url: str, lat: float, lng: float) -> dict:
    """POST /api/fire 한 번 호출."""
    resp = await client.post(
        f"{url}/api/fire",
        json={"lat": lat, "lng": lng},
        follow_redirects=True,
    )
    resp.raise_for_status()
    return resp.json()


# ---------------------------------------------------------------------------
# Phase implementations
# ---------------------------------------------------------------------------


async def phase_1_ignite(
    client: httpx.AsyncClient,
    url: str,
    locations: list[dict],
    delay: float,
) -> None:
    """Phase 1: 전국 21개 지점에 1회씩 점화."""
    _log("=" * 60)
    _log("PHASE 1 — 점화 (Ignite)")
    _log(f"  {len(locations)}개 지점에 불씨를 놓습니다...")
    _log("=" * 60)

    for i, loc in enumerate(locations, 1):
        try:
            result = await _ignite(client, url, loc["lat"], loc["lng"])
            stage = result.get("stage", 0)
            count = result.get("active_count", "?")
        except Exception as e:
            stage, count = "?", "?"
            _log(f"  ⚠ {loc['name']} 실패: {e}")
            continue
        _log(
            f"  [{i:2d}/{len(locations)}] {loc['name']:16s}  "
            f"→ {_stage_bar(stage)}  (count: {count})"
        )
        await asyncio.sleep(delay)

    _log("")
    _log("Phase 1 완료 — 전국에 불씨가 퍼졌습니다!")
    _log("")


async def phase_2_spread(
    client: httpx.AsyncClient,
    url: str,
    locations: list[dict],
    delay: float,
) -> None:
    """Phase 2: 각 지점에 반복 요청으로 모닥불/화재까지 확산."""
    _log("=" * 60)
    _log("PHASE 2 — 확산 (Spread)")
    _log("  각 지점을 모닥불~화재 수준으로 끌어올립니다...")
    _log("=" * 60)

    clicks_per_location = 60
    batch_size = 10

    for loc in locations:
        last_result: dict = {}
        for _ in range(clicks_per_location // batch_size):
            tasks = [
                _ignite(client, url, loc["lat"], loc["lng"])
                for _ in range(batch_size)
            ]
            results = await asyncio.gather(*tasks, return_exceptions=True)
            for r in reversed(results):
                if isinstance(r, dict):
                    last_result = r
                    break
            await asyncio.sleep(delay)

        stage = last_result.get("stage", 0)
        count = last_result.get("active_count", "?")
        _log(f"  {loc['name']:16s}  → {_stage_bar(stage)}  (count: {count})")

    _log("")
    _log("Phase 2 완료 — 전국이 불바다입니다!")
    _log("")


async def phase_3_inferno(
    client: httpx.AsyncClient,
    url: str,
    locations: list[dict],
    delay: float,
) -> None:
    """Phase 3: 핵심 거점을 대화재/전소까지 집중 타격."""
    hotspots = [loc for loc in locations if loc["name"] in HOTSPOT_NAMES]
    if not hotspots:
        hotspots = locations[:5]

    _log("=" * 60)
    _log("PHASE 3 — 대화재 (Inferno)")
    _log(f"  핵심 거점 {len(hotspots)}곳을 전소 단계까지 밀어붙입니다!")
    for h in hotspots:
        _log(f"    ▸ {h['name']}")
    _log("=" * 60)

    target_clicks = 210
    batch_size = 15

    for loc in hotspots:
        _log(f"\n  ── {loc['name']} 집중 타격 시작 ──")
        rounds = target_clicks // batch_size
        for _ in range(rounds):
            tasks = [
                _ignite(client, url, loc["lat"], loc["lng"])
                for _ in range(batch_size)
            ]
            results = await asyncio.gather(*tasks, return_exceptions=True)
            last: dict = {}
            for r in reversed(results):
                if isinstance(r, dict):
                    last = r
                    break
            stage = last.get("stage", 0)
            count = last.get("active_count", 0)
            label = _stage_bar(stage)
            bar = "█" * min(count // 5, 40)
            _log(f"    {loc['name']:16s}  {label:12s}  [{bar}] {count}")
            if stage >= 5:
                _log(f"    🚒 {loc['name']} 전소 도달! 소방관 출동!")
                break
            await asyncio.sleep(delay)

    _log("")
    _log("Phase 3 완료 — 대한민국이 불타고 있습니다! 🔥🔥🔥")
    _log("")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


async def main() -> None:
    parser = argparse.ArgumentParser(
        description="전국 동시 방화 시연 스크립트",
    )
    parser.add_argument(
        "--url",
        default="http://hwarr.com",
        help="서버 URL (기본값: http://hwarr.com)",
    )
    parser.add_argument(
        "--phase",
        type=int,
        choices=[1, 2, 3],
        default=None,
        help="특정 페이즈만 실행 (미지정 시 전체 실행)",
    )
    parser.add_argument(
        "--speed",
        type=float,
        default=0.3,
        help="요청 간 딜레이(초), 기본값 0.3",
    )
    parser.add_argument(
        "--no-reset",
        action="store_true",
        help="기존 화재 데이터를 초기화하지 않음",
    )
    args = parser.parse_args()

    url: str = args.url.rstrip("/")
    run_phases: list[int] = [args.phase] if args.phase else [1, 2, 3]

    _log("🔥 전국불판 시연 스크립트 시작")
    _log(f"  서버: {url}")
    _log(f"  페이즈: {run_phases}")
    _log(f"  딜레이: {args.speed}s")
    _log(f"  위치 수: {len(LOCATIONS)}")
    _log("")

    async with httpx.AsyncClient(timeout=30.0) as client:
        # Health check
        try:
            resp = await client.get(f"{url}/health", follow_redirects=True)
            resp.raise_for_status()
            _log(f"서버 연결 확인 ✓  ({url}/health → {resp.status_code})")
        except Exception as e:
            _log(f"서버 연결 실패: {e}")
            sys.exit(1)
        _log("")

        # Reset
        if not args.no_reset:
            _log("기존 화재 데이터 초기화 중...")
            try:
                resp = await client.post(f"{url}/api/reset", follow_redirects=True)
                if resp.status_code == 200:
                    body = resp.json()
                    _log(f"  초기화 완료: {body.get('deleted_keys', 0)}개 키 삭제")
                else:
                    _log(f"  초기화 실패 (HTTP {resp.status_code})")
            except Exception as e:
                _log(f"  초기화 요청 실패: {e}")
            _log("")

        locations = LOCATIONS[:]
        random.shuffle(locations)

        if 1 in run_phases:
            await phase_1_ignite(client, url, locations, args.speed)
            await asyncio.sleep(1)

        if 2 in run_phases:
            await phase_2_spread(client, url, locations, args.speed)
            await asyncio.sleep(1)

        if 3 in run_phases:
            await phase_3_inferno(client, url, locations, args.speed)

    _log("=" * 60)
    _log("시연 완료! 🇰🇷🔥")
    _log("=" * 60)


if __name__ == "__main__":
    asyncio.run(main())
