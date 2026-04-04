/**
 * MELTTOWN 환경 설정
 *
 * 선택 이유:
 * - 개발 시 서버 URL 하드코딩 (환경변수 셋업보다 빠름)
 * - 프로덕션에서는 VITE_API_URL 환경변수로 오버라이드
 */

export const API_URL = import.meta.env.VITE_API_URL ?? 'https://hwarr.com';

export const SOCKET_URL =
  import.meta.env.VITE_SOCKET_URL ?? 'https://hwarr.com';

// 불 시스템 상수 — 서버(grid.py)와 반드시 동일해야 함
export const LAT_UNIT = 0.0009; // ~100m (위도)
export const LNG_UNIT = 0.0011; // ~100m (경도, 한국 기준 ~37°N)
export const TTL_SECONDS = 1800; // 30분
