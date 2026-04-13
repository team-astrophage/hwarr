# hwarr loadtest

`docker compose up --build` 으로 올라간 로컬 스택에 대해
여러 가상 유저가 붙어 viewport 를 구독하고 `fire:ignite` 를 지속적으로 쏘는 시뮬레이터.

브라우저 수동 클릭을 자동화하고, viewport 브로드캐스트/불쏘기 부하 특성을 재현할 때 사용한다.

## 요구사항
- Node.js 18 이상 (글로벌 `fetch` 사용)
- 로컬 서버가 `http://localhost:8000` 에서 실행 중 (`docker compose up --build`)

## 실행 (권장: 루트 Makefile)

프로젝트 루트에서:

```bash
make up                        # 도커 기동 + 서버 ready 대기
make loadtest-smoke            # 5명 / 15초 스모크
make loadtest-run              # 200명 / 5분 기본 부하
USERS=50 DURATION=60 make loadtest-run-custom  # override
make down                      # 정리
```

의존성(`node_modules`)은 최초 실행 시 자동 설치됩니다.
`make help` 로 전체 타겟 확인.

## 실행 (Makefile 없이 직접)

```bash
cd scripts/loadtest
npm install

node run.js                                    # 기본 200명 / 5분
node run.js --users 5 --duration-sec 15        # 스모크
node run.js --users 200 --duration-sec 300 \
  --ramp-ms 50 --server http://localhost:8000 \
  --min-interval-ms 2000 --max-interval-ms 10000
```

환경변수 `SERVER_URL` 로도 서버 주소 override 가능. `Ctrl+C` 로 언제든 정리 후 종료.

## 출력 예시

```
[loadtest] starting: server=http://localhost:8000 users=200 duration=300s ramp=50ms ...
[loadtest] ramp complete: 200 users spawned
[loadtest] t=005s users=200/200 fires sent=187 ok=186 err=1 rps=37.2 recv={ignite:912, update:410, spread:55}
...
[loadtest] DONE duration=300s users=200/200 fires ok=10842 err=7 avg_rps=36.1 recv={...}
```

## 동작 요약
- 프로덕션 클라이언트(`client/src/lib/socketManager.ts`) 와 동일한 핸드셰이크:
  `/api/token` 에서 HMAC 토큰을 유저별로 받아 `auth: { user_id, token }` 로 연결.
- 10초 heartbeat 유지 (서버 reaper 30s 회피).
- 연결 직후 한국 내 랜덤 bbox 로 `subscribe:viewport` 1회.
- 유저별 2~10초 랜덤 간격으로 `fire:ignite { lat, lng }` 발사.
- 서버가 브로드캐스트하는 `fire:ignite`/`fire:update`/`fire:spread`/`users:count` 수신 카운트 집계.
- 5초마다 프로그레스 라인 출력, 종료 시 최종 리포트.
