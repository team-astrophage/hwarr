# Product Requirements Document: MELTTOWN

**Author**: Team MELTTOWN
**Date**: 2026-04-04
**Status**: Draft
**Stakeholders**: Frontend (2), Backend (1)

---

## 1. Executive Summary

MELTTOWN은 GPS 기반 전국 실시간 스트레스 해소 웹 서비스다. 사용자는 자신이 물리적으로 있는 위치에서만 "불을 지를" 수 있고, 전국 지도 위에서 같은 순간 불을 지르는 사람들을 실시간으로 볼 수 있다. 로그인 없이 URL 접속만으로 즉시 사용 가능하며, 캡처 공유를 통한 자연스러운 바이럴 구조를 갖는다.

---

## 2. Background & Context

### 문제 정의

흡연자들은 스트레스를 받을 때 "담배 타임"이라는 합법적인 이탈 시간이 있다. 비흡연자에게는 그런 시간이 없다. 온라인 담타(담배 타임 시뮬레이터)가 화제가 된 이유는 바로 이 박탈감을 건드렸기 때문이다.

### 시장 컨텍스트

- 온라인 담타의 화제성이 "스트레스 해소 + 집단적 공감" 니즈를 증명
- 기존 스트레스 해소 앱들은 명상, 호흡 등 정적인 접근 → MELTTOWN은 파괴적/능동적 접근
- GPS 기반 현장 제한 + 실시간 집단 행동은 기존 서비스에 없는 차별점

### 핵심 인사이트

```
스트레스 받는 순간
   ↓
내가 지금 있는 이 자리에 불을 지름
   ↓
전국 지도에서 같은 순간 불을 지르는 사람들이 보임
   ↓
혼자가 아니라는 느낌 + 집단적 카타르시스
```

### 서비스명

**MELTTOWN** = MELTDOWN + TOWN. "도시가 녹아내린다"는 컨셉. 지도 기반 서비스와 자연스럽게 연결되며, 바이럴 문장("오늘 강남 MELTTOWN 됐다")에도 적합.

---

## 3. Objectives & Success Metrics

### Goals

1. 해커톤 당일 완성 가능한 MVP를 제작하여 라이브 데모 시연
2. 발표장에서 청중이 실시간으로 참여하는 인터랙티브 데모 구현
3. 캡처 공유가 자연스럽게 발생하는 바이럴 구조 확보

### Non-Goals

1. 회원 시스템, 로그인, 프로필 — 완전 익명 서비스
2. 소방관 기능 로직 — 시각 이펙트만 (P1), 실제 불 단계 감소 기능 없음
3. 네이티브 앱 — 웹 전용 (모바일 브라우저 최적화)
4. 수익화, 광고 — 해커톤 프로젝트로 고려하지 않음
5. 글로벌 지원 — 한국 지도 영역만 지원

### Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| 데모 참여율 | 발표장 청중 50%+ 접속 | 실시간 동시 접속자 수 |
| 불 지르기 전환율 | 접속자 중 80%+ 불 지르기 | 불 이벤트 / 접속자 비율 |
| 재방문 액션 | 인당 평균 3회+ 불 지르기 | 이벤트 수 / 접속자 수 |
| 바이럴 지표 | 캡처/공유 발생 | 정성적 관찰 |

---

## 4. Target Users & Segments

### Primary: 누구나

- 로그인 없이 URL 접속만으로 즉시 사용
- 연령, 직업, 성별 무관
- 핵심 동기: 스트레스 해소, 재미, 공감

### Key Scenarios

| 페르소나 | 상황 | 동기 |
|----------|------|------|
| 회사원 | 회의 직후 자리로 돌아오며 | 스트레스 해소, "나만 힘든 게 아니구나" |
| SNS 유저 | 친구에게 링크 받고 접속 | 재미, 캡처 공유 욕구 |
| 해커톤 청중 | 발표장에서 QR 스캔 | 참여형 데모 경험 |

---

## 5. User Stories & Requirements

### P0 — Must Have

| # | User Story | Acceptance Criteria |
|---|-----------|-------------------|
| 1 | 사용자가 랜딩 페이지에서 전국 화재 현황 통계를 본다 | 전국 화재 N건, 실시간 N명 수치 표시. REST API로 fetch |
| 2 | 사용자가 "불 지르러 가기" 버튼을 눌러 지도 페이지로 이동한다 | 랜딩 → /map 라우팅. WebSocket은 지도 진입 시에만 연결 |
| 3 | 지도 페이지 진입 시 한반도 전체 뷰에서 내 위치로 줌인된다 | 초기 줌 ~7 (한반도 전체) → GPS 위치로 줌인 애니메이션. 한국 영역 바운더리 제한 |
| 4 | 사용자가 현재 GPS 위치의 100m 격자에 불을 지른다 | Web Geolocation API로 위치 감지 → 100m 격자 매핑 → 해당 격자에만 불 생성 |
| 5 | 탭하면 성냥 던지는 애니메이션으로 불이 생긴다 | 탭 → 성냥이 날아가는 애니메이션 → 격자에 불 생성 이펙트 |
| 6 | 꾹 누르면 화염방사기 애니메이션이 나온다 | long press → 누르는 지점에서 화염방사기/토치 느낌 불꽃 연출 |
| 7 | 불 단계가 활성 인원에 따라 시각적으로 변한다 | 프로덕션: 1단계(불씨, 1~5) → 2단계(모닥불, 6~20) → 3단계(화재, 21~50) → 4단계(대형화재, 51~100) → 5단계(전소, 100+). 개발용: 1/3/6/10/15. Canvas 기반 화염 파티클 애니메이션. 축소 시 빨간색 격자 색상으로 강도 표현, 확대 시 격자 안에서 화염 애니메이션 재생 |
| 8 | 쿨타임 없음 | 쿨타임 제거됨. 연속 클릭 가능 |
| 9 | 각 불 이벤트는 30분 TTL로 자동 소멸된다 | Redis Sorted Set 기반, 30분 경과 후 자동 제거 |
| 10 | 전국 지도에서 활성 격자의 불이 실시간 동기화된다 | WebSocket으로 전체 브로드캐스트 (MVP). 다른 사람이 불 지르면 즉시 반영 |
| 11 | 개발자 콘솔에서 GPS 위치를 오버라이드할 수 있다 | `window.__MELTTOWN_GPS = {lat, lng}` 등으로 위치 조작 가능. 데모/테스트용 |
| 12 | 지도에서 내 위치가 항상 표시된다 | 파란 점 + 펄스 애니메이션으로 현재 GPS 위치 표시 (네이버지도 스타일) |
| 13 | 방화범 카운트는 실시간 접속자 수를 표시한다 | Socket.IO 연결 수 기반. 접속/해제 시 전체 브로드캐스트 |

### P1 — Should Have

| # | User Story | Acceptance Criteria |
|---|-----------|-------------------|
| 14 | 4단계 이상 격자에 소방차 애니메이션이 표시된다 | 4단계+ 격자에 소방차 시각 이펙트만 표시. 실제 불 단계 감소 기능 없음 |
| 15 | 랜딩 페이지에 뉴스 형식의 화재 현황이 표시된다 | AI 기반 자연어 현황 ("구로 디지털단지 14시부터 대형화재 발생" 등). 데이터 + AI로 생성 |

### P2 — Nice to Have / Future

| # | User Story | Acceptance Criteria |
|---|-----------|-------------------|
| 14 | 같은 지역을 보는 사용자끼리 휘발성 채팅을 한다 | 완전 익명, 현재 보고 있는 격자 기준 채팅방 자동 입장. 24시간 후 자동 삭제 |
| 15 | 5단계 도달 시 특별 폭발 이펙트가 나온다 | 전소 달성 시 특별 애니메이션 (폭발, 화면 효과) |
| 16 | 히트맵 아카이브로 과거 화재 현황을 볼 수 있다 | 일별/월별 히트맵, 역대 TOP 10 격자 |

---

## 6. Solution Overview

### 화면 흐름 (User Flow)

```
[URL 접속]
    ↓
[랜딩 페이지]
  - 전국 화재 통계 수치
  - "불 지르러 가기" CTA
    ↓
[지도 페이지]
  - 한반도 전체 뷰
    ↓ (줌인 애니메이션)
  - 내 위치 중심 지도
  - 주변 격자의 불 현황 실시간 표시
    ↓
[불 지르기]
  - 탭: 성냥 던지기 → 불 생성
  - 꾹: 화염방사기 → 불 생성
  - 쿨타임 없음 → 연속 클릭 가능
    ↓
[결과 확인]
  - 내 격자 불 단계 변화
  - 전국 지도에서 불바다 확인
  - (자연스럽게 캡처 → 공유)
```

### 핵심 기술 설계

#### 그리드 시스템
```javascript
// 100m x 100m 격자 계산
function getGridId(lat, lng) {
  const gridLat = Math.floor(lat / 0.001);
  const gridLng = Math.floor(lng / 0.001);
  return `${gridLat}:${gridLng}`;
}
```

#### 불 이벤트 저장 (Redis Sorted Set)
```
key: fire:{gridId}
score: 만료 시각 (unix timestamp, now + 1800초)
value: 이벤트 고유 ID

불 지르기  → ZADD fire:{gridId} {now+1800} {eventId}
활성 카운트 → ZCOUNT fire:{gridId} {now} +inf
만료 정리  → ZREMRANGEBYSCORE fire:{gridId} 0 {now}
```

#### 불 단계 계산
```javascript
function getFireStage(activeCount) {
  if (activeCount >= 100) return 5; // 전소
  if (activeCount >= 51)  return 4; // 대형화재
  if (activeCount >= 21)  return 3; // 화재
  if (activeCount >= 6)   return 2; // 모닥불
  if (activeCount >= 1)   return 1; // 불씨
  return 0;
}
```

#### 실시간 동기화
```
불 지르기 이벤트 발생
   ↓
서버: Redis 업데이트
   ↓
서버: 해당 격자 주변 클라이언트에만 브로드캐스트
(전체 브로드캐스트 X → 뷰포트 안 격자만)
```

### 기술 스택

| Layer | Stack |
|-------|-------|
| **Frontend** | Vite + React + TypeScript |
| **Routing** | TanStack Router |
| **Styling** | Tailwind CSS (Spotify DESIGN.md 참고) |
| **State** | Zustand (client) + TanStack Query (server) |
| **Map** | react-leaflet + 다크 테마 타일 |
| **Realtime** | socket.io-client |
| **Animation** | Canvas API |
| **Backend** | Python FastAPI + python-socketio |
| **Database** | Redis Sorted Set (TTL 기반 이벤트) |
| **Infra** | AWS ECS Fargate + ElastiCache + Terraform |

### 아키텍처 (Bulletproof React)

```
src/
├── app/                   # 앱 진입점, 프로바이더, 라우터
├── features/
│   ├── fire-map/          # 지도 + 불 지르기 핵심 기능
│   │   ├── components/    # FireMap, FireOverlay, FireButton
│   │   ├── hooks/         # useFireMap, useGeolocation, useSocket
│   │   ├── api/           # 소켓 이벤트, API 호출
│   │   ├── stores/        # 불 상태, 격자 상태 (Zustand)
│   │   ├── utils/         # 격자 계산, 단계 계산
│   │   └── types/
│   └── landing/           # 랜딩 페이지
│       ├── components/    # StatsCard
│       ├── api/           # 통계 API
│       └── types/
├── components/            # 공통 UI
├── hooks/                 # 공통 훅
├── lib/                   # 설정, 유틸리티
├── routes/                # TanStack Router 라우트
├── stores/                # 전역 상태 (소켓 연결)
├── styles/                # 글로벌 스타일
└── types/                 # 공통 타입
```

### 디바이스 & 디자인

- **모바일 퍼스트** — 모바일 최대 크기까지만 대응
- **다크 테마** — Spotify DESIGN.md 기반, 어두운 배경 위에 불빛이 돋보이는 구조
- **지도**: 다크 테마 OpenStreetMap 타일

---

## 7. Open Questions

| Question | Owner | Deadline |
|----------|-------|----------|
| 백엔드 API 스펙 (엔드포인트, WebSocket 이벤트명) | Backend | 개발 시작 전 |
| ~~Canvas 불 애니메이션 구체적 비주얼~~ (구현 완료: 5단계 화염 파티클) | Frontend | 완료 |
| 배포 도메인 및 SSL 설정 | Backend | 데모 전일 |
| 동시 접속자 수 예상 및 WebSocket 스케일링 전략 | Backend | 개발 중 |

---

## 8. Timeline & Phasing

### Phase 1: MVP (해커톤 당일) — P0 전체

| Milestone | 내용 |
|-----------|------|
| 프로젝트 세팅 | Vite + React + 전체 스택 설치, Bulletproof React 구조 |
| 랜딩 페이지 | 통계 UI + CTA 버튼 |
| 지도 페이지 | 다크 OSM + GPS 위치 감지 + 줌인 애니메이션 |
| 불 지르기 | Canvas 화염 파티클 + 빨간색 격자 + 쿨타임 없음 + 내 위치 마커 |
| 실시간 동기화 | WebSocket 연결 + 불 상태 실시간 반영 |
| GPS 백도어 | 콘솔 위치 오버라이드 |
| 데모 | 라이브 배포 + QR 코드 |

### Phase 2: Enhancement — P1

- AI 뉴스 피드 (자연어 화재 현황)
- 소방차 시각 이펙트

### Phase 3: Expansion — P2

- 지역 휘발성 채팅
- 5단계 폭발 이펙트
- 히트맵/아카이브

---

## 바이럴 구조

```
강남 테헤란로 불바다 지도 캡처
   ↓
"우리 회사 MELTTOWN 됐다 ㅋㅋ" 인스타 스토리
   ↓
스토리 보고 친구들 접속
   ↓
자기 회사도 불 지름
   ↓
더 넓은 지역이 불바다
   ↓
더 강렬한 캡처 → 더 많은 공유
```

## 심사 기준 대응 (해커톤)

| 기준 | 대응 |
|------|------|
| 실용성 | 스트레스 해소 + 커뮤니티. 담타 화제성이 니즈 증명 |
| 창의성 | GPS 현장 제한 + 시간 기반 TTL + 집단행동 줄다리기 |
| 완성도 | 지도 + 불 + 실시간 동기화 MVP 완성 |
| 임팩트 | 발표장 라이브 데모 + 캡처 공유로 즉각 바이럴 |
