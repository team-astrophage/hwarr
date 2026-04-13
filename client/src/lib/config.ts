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

// 불 이벤트 TTL. grid 상수(LAT_UNIT/LNG_UNIT)는 서버가 subscribe:viewport 응답의
// gridMeta 로 내려주므로 클라이언트는 보관하지 않는다.
export const TTL_SECONDS = 1800; // 30분
