#!/usr/bin/env bash
#
# cutover-redis.sh
#
# ElastiCache Serverless → 단일 노드 ElastiCache 마이그레이션의 cutover 단계.
# (snapshot 생성/export 는 migrate-redis.sh 에서 처리, 본 스크립트는 그 이후)
#
# 흐름:
#   fix-bucket-policy → create-cluster → switch-ecs → verify → finalize
#
# 사용법:
#   ./cutover-redis.sh fix-bucket-policy   # S3 정책에 s3:GetObject 추가 (restore 용)
#   ./cutover-redis.sh create-cluster      # 새 replication group + subnet group 생성, available 까지 대기
#   ./cutover-redis.sh switch-ecs          # 새 task def revision 생성 + ECS service 강제 재배포 + 안정화 대기
#   ./cutover-redis.sh verify              # 새 endpoint 적용 여부 확인 + health check
#   ./cutover-redis.sh finalize            # 옛 serverless cache destroy (전체 terraform apply)
#
#   ./cutover-redis.sh all                 # fix-bucket-policy → verify (finalize 직전까지, 검증 후 사용자 confirm 필요)
#   ./cutover-redis.sh full                # all + finalize (전 과정 자동, destroy 포함)
#
# 환경변수 (override 가능):
#   AWS_PROFILE             (default: astrophage)
#   REGION                  (default: ap-northeast-2)
#   EXPECTED_ACCOUNT_ID     (default: 303238378572)
#   BUCKET                  (default: astrophage-hwarr-redis-backup-<account>)
#   RDB_ARN                 (default: 직전 export 결과 ARN — 변경 시 명시 필요)
#   REPLICATION_GROUP_ID    (default: astrophage-hwarr-prod-redis-rg)
#   CLUSTER                 (default: astrophage-hwarr-prod-cluster)
#   SERVICE                 (default: astrophage-hwarr-prod-backend)
#   TASK_FAMILY             (default: astrophage-hwarr-prod-backend)
#   HEALTH_URL              (default: https://hwarr.com/health)
#   INFRA_DIR               (default: 스크립트 기준 ../)
#

set -euo pipefail

export AWS_PAGER=""
export AWS_PROFILE="${AWS_PROFILE:-astrophage}"
export AWS_REGION="${REGION:-ap-northeast-2}"
REGION="$AWS_REGION"

EXPECTED_ACCOUNT_ID="${EXPECTED_ACCOUNT_ID:-303238378572}"
REPLICATION_GROUP_ID="${REPLICATION_GROUP_ID:-astrophage-hwarr-prod-redis-rg}"
CLUSTER="${CLUSTER:-astrophage-hwarr-prod-cluster}"
SERVICE="${SERVICE:-astrophage-hwarr-prod-backend}"
TASK_FAMILY="${TASK_FAMILY:-astrophage-hwarr-prod-backend}"
HEALTH_URL="${HEALTH_URL:-https://hwarr.com/health}"
INFRA_DIR="${INFRA_DIR:-$(cd "$(dirname "$0")/.." && pwd)}"

log()  { printf '\033[1;36m[%s]\033[0m %s\n' "$(date +%H:%M:%S)" "$*" >&2; }
warn() { printf '\033[1;33m[WARN %s]\033[0m %s\n' "$(date +%H:%M:%S)" "$*" >&2; }
err()  { printf '\033[1;31m[ERR  %s]\033[0m %s\n' "$(date +%H:%M:%S)" "$*" >&2; exit 1; }
ok()   { printf '\033[1;32m[OK   %s]\033[0m %s\n' "$(date +%H:%M:%S)" "$*" >&2; }

confirm() {
  read -r -p "$1 [y/N] " a
  [[ "$a" =~ ^[Yy]$ ]] || err "사용자 취소"
}

############################
# Account guard
############################
ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
if [[ "$ACCOUNT_ID" != "$EXPECTED_ACCOUNT_ID" ]]; then
  err "AWS account mismatch (expected=$EXPECTED_ACCOUNT_ID current=$ACCOUNT_ID). AWS_PROFILE=<profile> 로 다시 실행하세요."
fi

BUCKET="${BUCKET:-astrophage-hwarr-redis-backup-${ACCOUNT_ID}}"

# RDB_ARN — 직전 export 결과 (필요 시 override)
RDB_ARN="${RDB_ARN:-arn:aws:s3:::${BUCKET}/hwarr-redis-direct-180432/hwarr-redis-direct-180432-0001.rdb}"

############################
# Stage: fix-bucket-policy
############################
stage_fix_bucket_policy() {
  log "S3 버킷 정책 갱신 (snapshot export + restore 양쪽 권한)..."
  local pf
  pf=$(mktemp)
  cat > "$pf" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Sid": "AllowElastiCacheSnapshotExportAndRestore",
    "Effect": "Allow",
    "Principal": { "Service": "${REGION}.elasticache-snapshot.amazonaws.com" },
    "Action": [
      "s3:GetBucketAcl",
      "s3:GetObject",
      "s3:PutObject",
      "s3:DeleteObject",
      "s3:ListBucket"
    ],
    "Resource": [
      "arn:aws:s3:::$BUCKET",
      "arn:aws:s3:::$BUCKET/*"
    ]
  }]
}
EOF
  aws s3api put-bucket-policy \
    --bucket "$BUCKET" \
    --policy "file://$pf" \
    --region "$REGION"
  rm -f "$pf"
  ok "버킷 정책 갱신 완료 (s3:GetObject 포함)"
}

############################
# Stage: create-cluster
############################
ensure_state_clean() {
  cd "$INFRA_DIR"

  # 1) AWS 측 상태 확인 — create-failed 상태면 직접 삭제 후 대기
  local aws_status
  aws_status=$(aws elasticache describe-replication-groups \
    --replication-group-id "$REPLICATION_GROUP_ID" \
    --region "$REGION" \
    --query 'ReplicationGroups[0].Status' --output text 2>/dev/null || echo "absent")

  case "$aws_status" in
    absent)
      log "AWS 에 '$REPLICATION_GROUP_ID' 없음 (clean)"
      ;;
    available)
      ok "AWS 에 '$REPLICATION_GROUP_ID' 이미 available — 생성 단계 skip 예정"
      return 0
      ;;
    create-failed|deleting|modifying|*)
      warn "AWS 에 '$REPLICATION_GROUP_ID' 가 '$aws_status' 상태 — 삭제 후 재생성 필요"
      if [[ "$aws_status" != "deleting" ]]; then
        log "  delete-replication-group 호출..."
        aws elasticache delete-replication-group \
          --replication-group-id "$REPLICATION_GROUP_ID" \
          --region "$REGION" >/dev/null 2>&1 || warn "  delete 호출 실패 (이미 삭제 진행 중일 수 있음)"
      fi
      log "  완전 삭제 대기..."
      aws elasticache wait replication-group-deleted \
        --replication-group-id "$REPLICATION_GROUP_ID" \
        --region "$REGION"
      ok "  AWS 측 삭제 완료"
      ;;
  esac

  # 2) terraform state 잔재 제거
  if terraform state list 2>/dev/null | grep -q 'aws_elasticache_replication_group\.redis'; then
    log "terraform state 에 남은 replication group 항목 제거..."
    terraform state rm aws_elasticache_replication_group.redis || true
  fi
}

stage_create_cluster() {
  ensure_state_clean
  cd "$INFRA_DIR"
  log "새 replication group + subnet group 생성 (RDB seed 포함)..."
  log "  RDB_ARN: $RDB_ARN"
  terraform apply -auto-approve \
    -target=aws_elasticache_subnet_group.redis \
    -target=aws_elasticache_replication_group.redis \
    -var="redis_seed_snapshot_arn=$RDB_ARN"

  log "replication group '$REPLICATION_GROUP_ID' available 대기..."
  for _ in $(seq 1 60); do  # 최대 ~20분
    local s
    s=$(aws elasticache describe-replication-groups \
      --replication-group-id "$REPLICATION_GROUP_ID" \
      --region "$REGION" \
      --query 'ReplicationGroups[0].Status' --output text 2>/dev/null || echo "unknown")
    log "  status=$s"
    case "$s" in
      available)    ok "redis cluster ready"; return 0 ;;
      create-failed)
        warn "create-failed — 직전 이벤트:"
        aws elasticache describe-events \
          --source-type cache-cluster --region "$REGION" --duration 60 \
          --query "Events[?contains(SourceIdentifier, '${REPLICATION_GROUP_ID}')].[Date,SourceIdentifier,Message]" \
          --output table >&2 || true
        err "redis cluster 생성 실패. 위 메시지로 원인 파악 후 재시도."
        ;;
      *) sleep 20 ;;
    esac
  done
  err "redis available 대기 타임아웃"
}

############################
# Stage: switch-ecs
############################
stage_switch_ecs() {
  cd "$INFRA_DIR"
  log "새 task definition 생성 (REDIS_URL 신규 endpoint)..."
  terraform apply -auto-approve \
    -target=aws_ecs_task_definition.backend \
    -var="redis_seed_snapshot_arn=$RDB_ARN"

  log "ECS service 강제 재배포 — task family '$TASK_FAMILY' 의 latest revision 사용"
  aws ecs update-service \
    --cluster "$CLUSTER" \
    --service "$SERVICE" \
    --task-definition "$TASK_FAMILY" \
    --force-new-deployment \
    --region "$REGION" >/dev/null

  log "rolling deployment 안정화 대기 (services-stable)..."
  aws ecs wait services-stable \
    --cluster "$CLUSTER" \
    --services "$SERVICE" \
    --region "$REGION"
  ok "ECS service stable"
}

############################
# Stage: verify
############################
stage_verify() {
  log "현재 service 가 가리키는 task definition 확인..."
  local td
  td=$(aws ecs describe-services \
    --cluster "$CLUSTER" \
    --services "$SERVICE" \
    --region "$REGION" \
    --query 'services[0].taskDefinition' --output text)
  log "  taskDefinition=$td"

  log "task 들의 healthStatus..."
  local arns
  arns=$(aws ecs list-tasks --cluster "$CLUSTER" --service-name "$SERVICE" \
    --region "$REGION" --query 'taskArns' --output text)
  if [[ -n "$arns" && "$arns" != "None" ]]; then
    aws ecs describe-tasks --cluster "$CLUSTER" --tasks $arns --region "$REGION" \
      --query 'tasks[].{lastStatus:lastStatus,health:healthStatus}' --output table >&2
  fi

  log "health endpoint check: $HEALTH_URL"
  local code
  code=$(curl -s -o /dev/null -w '%{http_code}' "$HEALTH_URL" || true)
  if [[ "$code" == "200" ]]; then
    ok "health 200 OK"
  else
    warn "health endpoint 응답 코드: $code (서비스 안정화 후 다시 확인 필요할 수 있음)"
  fi
}

############################
# Stage: finalize
############################
stage_finalize() {
  cd "$INFRA_DIR"
  log "잔여 변경 사항 plan (옛 serverless destroy 가 핵심):"
  terraform plan \
    -var="redis_seed_snapshot_arn=$RDB_ARN" | tail -30

  echo
  confirm "위 plan 으로 진행할까요? (옛 serverless cache 가 삭제됩니다)"

  terraform apply -auto-approve \
    -var="redis_seed_snapshot_arn=$RDB_ARN"
  ok "finalize 완료 — serverless cache destroyed"
}

############################
# Dispatch
############################
case "${1:-}" in
  fix-bucket-policy) stage_fix_bucket_policy ;;
  create-cluster)    stage_create_cluster ;;
  switch-ecs)        stage_switch_ecs ;;
  verify)            stage_verify ;;
  finalize)          stage_finalize ;;
  all)
    stage_fix_bucket_policy
    stage_create_cluster
    stage_switch_ecs
    stage_verify
    log "전 단계 완료. finalize (serverless destroy) 는 검증 후 다음 명령으로 진행:"
    log "  $0 finalize"
    ;;
  full)
    stage_fix_bucket_policy
    stage_create_cluster
    stage_switch_ecs
    stage_verify
    stage_finalize
    ok "전체 cutover 완료"
    ;;
  *)
    cat <<EOF
사용법: $0 <subcommand>

  fix-bucket-policy   S3 정책에 s3:GetObject 추가 (restore 용)
  create-cluster      새 redis cluster 생성, available 까지 대기
  switch-ecs          새 task def 생성 + ECS service 강제 재배포 + 안정화 대기
  verify              새 endpoint 적용 여부 + health 확인
  finalize            옛 serverless cache destroy (확인 후 진행)

  all                 fix-bucket-policy → verify (finalize 제외, 안전)
  full                all + finalize (자동, destroy 포함)

환경변수:
  AWS_PROFILE             (default: astrophage)
  REGION                  (default: ap-northeast-2)
  RDB_ARN                 (default: 이전 export 결과)
  REPLICATION_GROUP_ID    (default: astrophage-hwarr-prod-redis-rg)
  CLUSTER / SERVICE / TASK_FAMILY / HEALTH_URL
EOF
    exit 1
    ;;
esac
