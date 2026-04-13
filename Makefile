# hwarr — 루트 Makefile
#
# 개발용 편의 래퍼. 현재는 docker compose 제어 + loadtest 시뮬레이터만 제공.
# 향후 client-*, server-* 타겟을 같은 네임스페이스 규칙으로 확장할 수 있도록
# 기능별 prefix 를 붙여둔다.

SHELL := /bin/bash

# ---- Paths ------------------------------------------------------------------
LOADTEST_DIR := scripts/loadtest
COMPOSE      := docker compose

# ---- Loadtest knobs (override via env) --------------------------------------
USERS            ?= 200
DURATION         ?= 300
RAMP_MS          ?= 250
MIN_INTERVAL_MS  ?= 200
MAX_INTERVAL_MS  ?= 500
SERVER           ?= http://localhost:8000

# ---- Pretty output ----------------------------------------------------------
BOLD   := \033[1m
DIM    := \033[2m
CYAN   := \033[36m
GREEN  := \033[32m
YELLOW := \033[33m
RED    := \033[31m
RESET  := \033[0m

define say
	@printf "$(CYAN)▸$(RESET) %s\n" "$(1)"
endef

.DEFAULT_GOAL := help
.PHONY: help \
        up down logs status reset \
        loadtest-install loadtest-smoke loadtest-run loadtest-run-custom loadtest-clean \
        _check-node _check-loadtest-deps _check-server

# ---- Help -------------------------------------------------------------------
help:
	@printf "$(BOLD)hwarr$(RESET) — 개발 편의 Makefile\n"
	@printf "\n"
	@printf "$(BOLD)빠른 시작$(RESET)\n"
	@printf "  $(CYAN)make up$(RESET)                 $(DIM)# 도커로 서버 기동 (백엔드+프론트+레디스)$(RESET)\n"
	@printf "  $(CYAN)make loadtest-smoke$(RESET)     $(DIM)# 5명 / 15초 스모크$(RESET)\n"
	@printf "  $(CYAN)make loadtest-run$(RESET)       $(DIM)# $(USERS)명 / $(DURATION)초 기본 부하$(RESET)\n"
	@printf "  $(CYAN)make down$(RESET)               $(DIM)# 도커 정리$(RESET)\n"
	@printf "\n"
	@printf "$(BOLD)도커 / 환경$(RESET)\n"
	@printf "  up                      docker compose up --build -d + 서버 ready 대기\n"
	@printf "  down                    docker compose down\n"
	@printf "  logs                    backend 로그 tail -f\n"
	@printf "  status                  서버 헬스 체크 ($(SERVER)/api/token)\n"
	@printf "  reset              Redis FLUSHDB — 불/카운터 등 테스트 데이터 초기화\n"
	@printf "\n"
	@printf "$(BOLD)loadtest$(RESET) (Socket.IO 다중 유저 시뮬레이터)\n"
	@printf "  loadtest-install        scripts/loadtest/ 의존성 설치 (보통은 자동)\n"
	@printf "  loadtest-smoke          5명 / 15초 (사전 검증)\n"
	@printf "  loadtest-run            기본값으로 풀 부하\n"
	@printf "  loadtest-run-custom     USERS/DURATION/... 환경변수로 override\n"
	@printf "                          예: $(DIM)USERS=50 DURATION=60 make loadtest-run-custom$(RESET)\n"
	@printf "  loadtest-clean          node_modules 삭제\n"
	@printf "\n"
	@printf "$(DIM)튜너블: USERS=$(USERS) DURATION=$(DURATION) RAMP_MS=$(RAMP_MS) SERVER=$(SERVER)$(RESET)\n"

# ---- Preflight checks -------------------------------------------------------
_check-node:
	@command -v node >/dev/null 2>&1 || { \
	  printf "$(RED)✗$(RESET) Node.js 가 설치돼 있지 않습니다. Node 18+ 필요.\n"; exit 1; }
	@node -e 'const v=process.versions.node.split(".").map(Number); if (v[0]<18) { console.error("Node "+process.versions.node+" — 18 이상이 필요합니다"); process.exit(1); }'

_check-loadtest-deps: _check-node
	@if [ ! -d $(LOADTEST_DIR)/node_modules ]; then \
	  printf "$(YELLOW)!$(RESET) $(LOADTEST_DIR)/node_modules 없음 → $(CYAN)npm install$(RESET) 자동 실행\n"; \
	  (cd $(LOADTEST_DIR) && npm install --silent) || { printf "$(RED)✗$(RESET) npm install 실패\n"; exit 1; }; \
	  printf "$(GREEN)✓$(RESET) 의존성 설치 완료\n"; \
	fi

_check-server:
	@if ! curl -fsS -o /dev/null -m 2 "$(SERVER)/api/token"; then \
	  printf "$(RED)✗$(RESET) 서버 응답 없음: $(SERVER)/api/token\n"; \
	  printf "$(DIM)  먼저 $(CYAN)make up$(RESET)$(DIM) 으로 도커를 올리거나, SERVER=... 로 주소를 지정하세요.$(RESET)\n"; \
	  exit 1; \
	fi
	@printf "$(GREEN)✓$(RESET) 서버 정상: $(SERVER)\n"

# ---- Docker compose ---------------------------------------------------------
up:
	$(call say,docker compose up --build -d)
	@$(COMPOSE) up --build -d
	$(call say,서버 기동 대기 (최대 60s)...)
	@for i in $$(seq 1 60); do \
	  if curl -fsS -o /dev/null -m 1 "$(SERVER)/api/token"; then \
	    printf "$(GREEN)✓$(RESET) 서버 준비 완료 ($(SERVER))\n"; exit 0; \
	  fi; \
	  sleep 1; \
	done; \
	printf "$(YELLOW)!$(RESET) 60초 내 서버 응답 없음 — $(CYAN)make logs$(RESET) 로 확인하세요\n"; \
	exit 1

down:
	$(call say,docker compose down)
	@$(COMPOSE) down

logs:
	@$(COMPOSE) logs -f backend

status:
	@if curl -fsS -o /dev/null -m 2 "$(SERVER)/api/token"; then \
	  printf "$(GREEN)✓$(RESET) UP    $(SERVER)\n"; \
	else \
	  printf "$(RED)✗$(RESET) DOWN  $(SERVER)\n"; exit 1; \
	fi

reset:
	$(call say,Redis FLUSHDB — 모든 테스트 데이터 삭제)
	@$(COMPOSE) exec -T redis redis-cli FLUSHDB
	@printf "$(GREEN)✓$(RESET) 초기화 완료. 브라우저는 viewport 를 다시 구독하면 깨끗한 상태가 보입니다.\n"

# ---- Loadtest ---------------------------------------------------------------
loadtest-install: _check-node
	$(call say,$(LOADTEST_DIR) 의존성 설치)
	@cd $(LOADTEST_DIR) && npm install

loadtest-clean:
	$(call say,$(LOADTEST_DIR)/node_modules 삭제)
	@rm -rf $(LOADTEST_DIR)/node_modules $(LOADTEST_DIR)/package-lock.json

loadtest-smoke: _check-loadtest-deps _check-server
	$(call say,스모크 테스트: 5명 / 15초)
	@cd $(LOADTEST_DIR) && node run.js --users 5 --duration-sec 15 --server $(SERVER)

loadtest-run: _check-loadtest-deps _check-server
	$(call say,부하 테스트: $(USERS)명 / $(DURATION)초)
	@cd $(LOADTEST_DIR) && node run.js \
	  --users $(USERS) \
	  --duration-sec $(DURATION) \
	  --ramp-ms $(RAMP_MS) \
	  --min-interval-ms $(MIN_INTERVAL_MS) \
	  --max-interval-ms $(MAX_INTERVAL_MS) \
	  --server $(SERVER)

loadtest-run-custom: loadtest-run
