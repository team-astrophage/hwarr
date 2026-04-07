# 화르르(hwarr) 문서

> GPS 기반 전국 실시간 스트레스 해소 웹 서비스

---

## 무엇을 찾고 있나요?

| 목적 | 카테고리 |
|------|----------|
| API나 설정값을 찾고 있다면 | [레퍼런스](#레퍼런스) |
| 왜 이렇게 설계했는지 알고 싶다면 | [설명](#설명) |
| 특정 작업을 수행하고 싶다면 | [하우투 가이드](#하우투-가이드) |
| 처음 시작한다면 | [튜토리얼](#튜토리얼) |

---

## 레퍼런스

- [REST API](reference/api-rest.md) — 전체 HTTP 엔드포인트
- [Socket.IO 이벤트](reference/api-socketio.md) — 실시간 이벤트 계약
- [Redis 스키마](reference/redis-schema.md) — 데이터 저장 구조
- [불 단계 시스템](reference/fire-stages.md) — 단계 0-5 규칙
- [격자 시스템](reference/grid-system.md) — 100m x 100m 셀 변환
- [설정값](reference/config-reference.md) — 환경변수 및 상수
- [Zustand 스토어](reference/zustand-stores.md) — 클라이언트 상태 관리

## 설명

- [아키텍처 개요](explanation/architecture-overview.md)
- [불 시스템 설계](explanation/fire-system-design.md)
- [실시간 통신 전략](explanation/realtime-strategy.md)
- [프론트엔드 상태 흐름](explanation/frontend-state-flow.md)

## 하우투 가이드

- [로컬 개발 환경 구축](how-to/local-dev-setup.md)
- [배포](how-to/deploy.md)
- [Socket.IO 이벤트 추가](how-to/add-socketio-event.md)
- [불 시스템 디버깅](how-to/debug-fire-system.md)

## 튜토리얼

- [첫 기여 가이드](tutorial/first-contribution.md)

## 기능 명세

- [랜딩 페이지](features/landing-page.md) — 메인 화면 UI/UX 및 통계 대시보드
- [불 지도](features/fire-map.md) — GPS 기반 격자 지도와 불 애니메이션
- [채팅](features/chat.md) — 익명 실시간 글로벌 채팅
- [피드백 & 아카이브](features/feedback-and-archive.md) — 사용자 피드백 제출 및 랭킹
- [글로벌 UI](features/global-ui.md) — 앱 구조, 라우팅, 소켓 관리, 디자인 토큰

---

## 기술 스택

| 영역 | 기술 |
|------|------|
| 프론트엔드 | React 19, Vite, TanStack Router/Query, Zustand, Tailwind CSS, Leaflet |
| 백엔드 | Go (Gin), Socket.IO, Redis |
| 인프라 | Terraform, Docker |
| 실시간 통신 | Socket.IO |

---

## 문서 업데이트 정책

이 문서는 PR이 `main` 브랜치에 머지될 때 Claude Code가 자동으로 변경 사항을 감지하고 문서 업데이트 PR을 생성합니다. 코드 변경 시 별도로 문서를 수정할 필요가 없습니다.
