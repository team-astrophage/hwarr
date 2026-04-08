# 첫 번째 기여: 랜딩 페이지에 새 통계 카드 추가하기

이 튜토리얼은 화르르(hwarr) 프로젝트에 처음 기여하는 개발자(또는 AI agent)를 위한 실전 가이드입니다. "접속 중인 사용자 수" 통계 카드를 랜딩 페이지에 추가하는 과정을 단계별로 따라가며, 프로젝트의 server-client 흐름 전체를 익힙니다.

> **예제 시나리오:** `onlineUsers` 값은 이미 API 응답에 포함되어 있지만 랜딩 페이지에 표시되지 않습니다. 이 값을 새 `StatCard`로 보여주는 것이 목표입니다.

---

## 1. 프로젝트 구조 이해

통계 카드 하나를 추가하려면 세 곳을 건드립니다.

```
houston/
├── server/
│   ├── internal/config/config.go   # 설정
│   └── internal/handler/stats.go   # GET /api/stats 핸들러
└── client/
    └── src/features/landing/
        ├── api/useStats.ts            # TanStack Query 훅 & 타입 정의
        └── components/
            ├── LandingPage.tsx        # 상황판 섹션 (StatCard 배치)
            └── StatCard.tsx           # 개별 통계 카드 컴포넌트
```

데이터 흐름은 다음과 같습니다.

```
Redis → server/internal/handler/stats.go → JSON 응답 → useStats 훅 → LandingPage → StatCard
```

---

## 2. 서버: stats endpoint에 새 필드 추가

> 이번 예제에서는 `onlineUsers`가 이미 반환되고 있으므로 서버 수정이 필요 없습니다. 하지만 완전히 새로운 통계를 추가한다고 가정하고 과정을 설명합니다.

### 2-1. Redis key 등록 (필요시)

새 통계가 Redis에 저장되는 값이라면, `server/internal/engine/constants.go`에 key 상수를 등록합니다.

### 2-2. handler 응답에 필드 추가

`server/internal/handler/stats.go`의 핸들러에서 새 값을 조회하고 응답에 포함시킵니다.

**핵심 규칙:**
- 응답 필드명은 **camelCase**를 사용합니다 (frontend 호환, `json:"camelCase"` 태그).
- Redis에서 가져온 값이 빈 문자열일 수 있으므로 반드시 fallback 처리합니다.

---

## 3. 클라이언트: useStats 훅의 반환 타입 업데이트

`client/src/features/landing/api/useStats.ts`의 `StatsData` interface에 새 필드를 추가합니다.

```typescript
// client/src/features/landing/api/useStats.ts

interface StatsData {
  activeGrids: number
  totalFires: number
  cumulativeFires: number
  dailyFires: number
  onlineUsers: number
  peakUsers: number          // 새 필드 추가
}
```

나머지 코드는 수정할 필요가 없습니다. `useQuery`가 자동으로 새 필드를 포함한 객체를 반환합니다.

> **TIP:** 이 훅은 `refetchInterval: 10_000` (10초)으로 설정되어 있어 자동으로 최신 값을 가져옵니다. 별도의 polling 로직을 작성할 필요가 없습니다.

---

## 4. 클라이언트: StatCard 컴포넌트로 표시

### 4-1. StatCard 컴포넌트 이해

`StatCard`는 다음 props를 받습니다:

```typescript
interface StatCardProps {
  label: string               // 카드 상단 라벨 텍스트
  value: number | undefined   // 표시할 숫자 (로딩 중이면 undefined → "--" 표시)
  unit: string                // 숫자 옆 단위 텍스트
  tone: 'accent' | 'warning' | 'negative' | 'base'  // 숫자 색상
}
```

`tone`에 따라 숫자 색상이 결정됩니다:
| tone | CSS 변수 | 용도 |
|------|----------|------|
| `accent` | `--color-accent` | 주요 강조 (예: 활성 구역) |
| `warning` | `--color-warning` | 경고성 수치 (예: 실시간 건수) |
| `negative` | `--color-negative` | 부정적 수치 (예: 오늘 방화) |
| `base` | `--color-text-base` | 중립 수치 (예: 누적 건수) |

새로운 tone이 필요하다면 `StatCard.tsx`의 `StatTone` type과 `TONE_COLOR` 매핑에 추가하면 됩니다.

### 4-2. LandingPage에 카드 배치

`client/src/features/landing/components/LandingPage.tsx`의 상황판 grid에 새 `StatCard`를 추가합니다.

```tsx
// client/src/features/landing/components/LandingPage.tsx

{/* 상황판 */}
<section className="flex flex-col gap-4">
  <h2 className="text-[1rem] font-bold leading-none tracking-tight text-[var(--color-text-base)]">
    상황판
  </h2>
  <div className="grid grid-cols-2 gap-2.5">
    <StatCard
      label="실시간 화재 구역"
      value={stats?.activeGrids}
      unit="곳"
      tone="accent"
    />
    <StatCard
      label="실시간 불타는 건수"
      value={stats?.totalFires}
      unit="건"
      tone="warning"
    />
    <StatCard
      label="오늘 방화 건수"
      value={stats?.dailyFires}
      unit="건"
      tone="negative"
    />
    <StatCard
      label="누적 총 방화 건수"
      value={stats?.cumulativeFires}
      unit="건"
      tone="base"
    />
    {/* 새 카드 추가 */}
    <StatCard
      label="동시 접속자"
      value={stats?.onlineUsers}
      unit="명"
      tone="accent"
    />
  </div>
</section>
```

> **참고:** grid는 `grid-cols-2`로 설정되어 있습니다. 카드가 홀수 개가 되면 마지막 카드가 한 열을 차지합니다. 짝수로 맞추거나, 홀수 카드에 `col-span-2`를 적용하는 것을 고려하세요.

---

## 5. 테스트 및 확인

### 5-1. 서버 실행

```bash
# 프로젝트 루트에서
cd server
go run .
```

API 응답을 직접 확인합니다:

```bash
curl http://localhost:8000/api/stats | jq .
```

기대 응답:

```json
{
  "activeGrids": 3,
  "totalFires": 42,
  "cumulativeFires": 1234,
  "dailyFires": 15,
  "onlineUsers": 7,
  "peakUsers": 25
}
```

### 5-2. 클라이언트 실행

```bash
cd client
pnpm install    # 첫 실행시
pnpm dev
```

브라우저에서 랜딩 페이지를 열고 상황판에 새 카드가 표시되는지 확인합니다.

### 5-3. 체크리스트

- [ ] 새 필드가 API 응답에 포함되는가?
- [ ] `StatsData` interface에 타입이 정의되어 있는가?
- [ ] 카드에 숫자가 표시되는가? (로딩 중에는 `--`가 보여야 합니다)
- [ ] 10초 후 자동 갱신 시 값이 업데이트되는가?
- [ ] TypeScript compile error가 없는가? (`pnpm tsc --noEmit`)
- [ ] 카드 레이아웃이 2열 grid에서 깨지지 않는가?

---

## 6. PR 만들기

### 6-1. branch 생성 및 commit

```bash
git checkout -b feat/add-online-users-stat

git add server/internal/handler/stats.go server/internal/engine/constants.go
git add client/src/features/landing/api/useStats.ts
git add client/src/features/landing/components/LandingPage.tsx

git commit -m "feat(landing): add online users stat card to dashboard"
```

### 6-2. PR 생성

```bash
git push -u origin feat/add-online-users-stat

gh pr create \
  --base main \
  --title "feat(landing): add online users stat card" \
  --body "## Summary
- 랜딩 상황판에 동시 접속자 수 StatCard 추가
- useStats 훅의 StatsData interface에 onlineUsers 타입 반영

## Test plan
- [ ] curl로 /api/stats 응답에 onlineUsers 필드 확인
- [ ] 랜딩 페이지에서 카드 렌더링 확인
- [ ] 10초 자동 갱신 동작 확인"
```

### 6-3. commit message 컨벤션

이 프로젝트는 [Conventional Commits](https://www.conventionalcommits.org/) 형식을 사용합니다:

```
<type>(<scope>): <설명>

# 예시
feat(landing): add online users stat card
fix(server): handle None value in peak users stat
refactor(client): extract stat tone config
```

자주 쓰는 type: `feat`, `fix`, `refactor`, `docs`, `chore`

자주 쓰는 scope: `client`, `server`, `landing`, `map`, `infra`

---

## 요약: 수정 파일 목록

| 파일 | 변경 내용 |
|------|----------|
| `server/internal/engine/constants.go` | Redis key 상수 추가 (새 통계인 경우) |
| `server/internal/handler/stats.go` | handler 응답에 새 필드 포함 |
| `client/src/features/landing/api/useStats.ts` | `StatsData` interface에 필드 추가 |
| `client/src/features/landing/components/LandingPage.tsx` | `StatCard` 추가 |
| `client/src/features/landing/components/StatCard.tsx` | 새 tone 추가 (필요시만) |
