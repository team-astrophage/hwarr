# 커밋 가이드

> Agent용 간결한 규칙은 프로젝트 루트 `CLAUDE.md`에 정의되어 있다.
> 이 문서는 개발자 온보딩을 위한 상세 가이드다.

## 커밋 메시지 형식

```
<타입>(<스코프>): <제목>

<본문> (선택)
```

### 스코프

프로젝트 내 영향 범위를 명시한다.

| 스코프   | 대상                          | 예시                                    |
| -------- | ----------------------------- | --------------------------------------- |
| `client` | `client/` (React 프론트엔드)  | `feat(client): 랜딩 페이지 통계 구현`   |
| `server` | `server/` (Go Gin 백엔드)     | `fix(server): 불 단계 계산 오류 수정`   |
| `infra`  | Terraform, Docker, CI/CD      | `chore(infra): ECS 메모리 제한 조정`    |
| `docs`   | `docs/` 문서                  | `docs: Socket.IO 이벤트 문서 추가`      |

스코프 없이 사용할 수 있는 경우: 여러 영역에 걸친 변경 (예: `feat: 사용자 피드백 기능 추가`)

## 커밋 타입

| 타입       | 설명      | 예시                                              |
| ---------- | --------- | ------------------------------------------------- |
| `feat`     | 새 기능   | `feat(client): 채팅 메시지 전송 구현`             |
| `fix`      | 버그 수정 | `fix(server): Redis TTL 만료 후 크래시 수정`      |
| `refactor` | 리팩토링  | `refactor(client): 불 애니메이션 Canvas 최적화`   |
| `style`    | 코드 포맷 | `style(server): ruff 포매팅 적용`                 |
| `test`     | 테스트    | `test(server): 격자 변환 유닛 테스트 추가`        |
| `docs`     | 문서      | `docs: API 레퍼런스 업데이트`                     |
| `chore`    | 빌드/설정 | `chore(infra): GitHub Actions 워크플로우 수정`    |
| `design`   | UI/UX     | `design(client): 랜딩 페이지 카드 레이아웃 개선`  |

## 원칙

### DO

**1. 하나의 커밋 = 하나의 목적**

```bash
# Good
feat(server): 랭킹 API 엔드포인트 구현
```

**2. 구체적인 제목**

```bash
# Good
feat(client): 불 지도 격자 오버레이 구현
fix(server): 동시 불 이벤트 Redis race condition 수정

# Bad
feat(client): 기능 추가
fix(server): 수정
```

**3. 의미 있는 단위**

- 독립적으로 이해 가능
- 테스트 가능한 단위
- 롤백 가능한 단위

**4. 본문 작성 (변경 이유가 제목만으로 불명확할 때)**

```
refactor(server): 피드백 전송을 Discord 웹훅으로 변경

SMTP 서버 유지보수 부담을 줄이고
팀 Discord에서 실시간으로 피드백을 확인하기 위해
Discord 웹훅 방식으로 전환.
```

### DON'T

**1. 여러 기능 한번에**

```bash
# Bad
feat(client): 채팅, 피드백, 랭킹 구현

# Good — 3개로 분리
feat(client): 채팅 메시지 전송 구현
feat(client): 피드백 모달 구현
feat(client): 랭킹 리스트 구현
```

**2. 모호한 메시지**

```bash
# Bad
fix(server): 수정
refactor(client): 코드 정리

# Good
fix(server): 격자 ID 계산 시 경도 부호 오류 수정
refactor(client): 불 상태를 Zustand 스토어로 통합
```

**3. WIP 커밋**

```bash
# Bad
WIP: 작업중

# Good — 완성된 단위로 커밋
feat(client): 불 지도 기본 구조 구현
```

**4. 포맷 + 로직 섞기**

```bash
# Bad — 한 커밋에 포맷팅과 로직 변경

# Good — 분리
style(server): ruff 포매팅 적용
feat(server): 소방차 출동 로직 구현
```

## 실전 예시

### 기능 개발

```
feat(client): 불 지도 실시간 격자 오버레이 구현

- Leaflet 격자 레이어 렌더링
- Socket.IO로 실시간 불 상태 구독
- 불 단계별 색상 그라데이션
```

### 버그 수정

```
fix(server,client): 클라이언트 IP가 프록시 주소로 표시되는 문제

- X-Forwarded-For 헤더 파싱 추가
- Discord 임베드에 실제 IP 표시
```

### 리팩토링

```
refactor(client): 피드백 모달을 portal로 분리

- createPortal로 DOM 트리 최상위에 렌더링
- clipped container에서 모달이 잘리는 문제 해결
```

## 대규모 기능 개발 시 단계별 커밋

```bash
feat(server): 불 이벤트 Redis 저장 구조 설계
feat(server): 불 단계 계산 로직 구현
feat(client): 불 지도 기본 레이아웃 구현
feat(client): 불 던지기 버튼 및 쿨다운 구현
feat(client): 불 단계별 Canvas 애니메이션 구현
feat(client): 소방차 오버레이 렌더링 구현
```

## 커밋 전 체크리스트

- [ ] 하나의 목적만 포함?
- [ ] 제목이 구체적인가?
- [ ] 타입과 스코프가 올바른가?
- [ ] 프론트: 타입 체크 통과 (`cd client && npx tsc --noEmit`)
- [ ] 백엔드: 린트 통과 (`cd server && go vet ./...`)
