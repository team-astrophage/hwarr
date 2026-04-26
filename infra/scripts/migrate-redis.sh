#!/usr/bin/env bash
#
# migrate-redis.sh
#
# ElastiCache Serverless → 단일 노드 ElastiCache(t4g.micro) 마이그레이션 헬퍼.
#
# 흐름:
#   1) Serverless 캐시에서 수동 스냅샷 생성
#   2) 스냅샷을 S3 버킷으로 export
#   3) RDB ARN 출력 → terraform apply -var=redis_seed_snapshot_arn=<ARN> 에 사용
#
# 사용법:
#   ./migrate-redis.sh snapshot              # 1단계만
#   ./migrate-redis.sh export                # 2단계만 (snapshot 이름 필요)
#   ./migrate-redis.sh all                   # 1+2 한번에
#   ./migrate-redis.sh cleanup-snapshot      # ElastiCache 스냅샷 삭제 (마이그레이션 완료 후)
#   ./migrate-redis.sh cleanup-bucket        # S3 백업 버킷 삭제 (마이그레이션 완료 후)
#
# 환경변수 (override 가능):
#   REGION             default: ap-northeast-2
#   SERVERLESS_NAME    default: astrophage-hwarr-prod-redis
#   BUCKET             default: astrophage-hwarr-redis-backup-<account_id>
#   SNAPSHOT_NAME      default: hwarr-redis-migration-<timestamp>
#

set -euo pipefail

# AWS CLI v2 가 자동으로 less pager 를 띄우면 스크립트가 멈춘 것처럼 보임 → 비활성화
export AWS_PAGER=""

REGION="${REGION:-ap-northeast-2}"
SERVERLESS_NAME="${SERVERLESS_NAME:-astrophage-hwarr-prod-redis}"

# 본 프로젝트가 속한 AWS account. 다른 account 자격증명으로 실행하면
# 캐시/스냅샷이 not-found 로 떨어지고 잘못된 account에 S3 버킷이 생기므로 가드.
EXPECTED_ACCOUNT_ID="${EXPECTED_ACCOUNT_ID:-303238378572}"

ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
if [[ "$ACCOUNT_ID" != "$EXPECTED_ACCOUNT_ID" ]]; then
  printf '\033[1;31m[ERR]\033[0m AWS account mismatch.\n' >&2
  printf '       expected: %s\n' "$EXPECTED_ACCOUNT_ID" >&2
  printf '       current : %s (caller: %s)\n' \
    "$ACCOUNT_ID" \
    "$(aws sts get-caller-identity --query Arn --output text)" >&2
  printf '\n' >&2
  printf '       사용 중인 AWS profile/credentials 가 잘못된 account를 가리키고 있습니다.\n' >&2
  printf '       interactive shell 의 alias/wrapper(aws-vault 등)는 bash 스크립트로 전파되지 않습니다.\n' >&2
  printf '       올바른 profile 을 명시해서 다시 실행하세요. 예:\n' >&2
  printf '         AWS_PROFILE=<your-profile> %s %s\n' "$0" "${1:-<subcommand>}" >&2
  printf '       또는 다른 account 로 의도적으로 실행하려면:\n' >&2
  printf '         EXPECTED_ACCOUNT_ID=%s %s %s\n' "$ACCOUNT_ID" "$0" "${1:-<subcommand>}" >&2
  exit 1
fi

BUCKET="${BUCKET:-astrophage-hwarr-redis-backup-${ACCOUNT_ID}}"

# 스냅샷 이름은 stage 간 공유되어야 하므로 파일에 캐시
STATE_FILE="${TMPDIR:-/tmp}/hwarr-redis-migration.state"

log()  { printf '\033[1;36m[%s]\033[0m %s\n' "$(date +%H:%M:%S)" "$*" >&2; }
warn() { printf '\033[1;33m[WARN]\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[1;31m[ERR]\033[0m %s\n' "$*" >&2; exit 1; }

confirm() {
  local prompt="$1"
  read -r -p "$prompt [y/N] " answer
  [[ "$answer" =~ ^[Yy]$ ]] || err "사용자 취소"
}

save_snapshot_name() {
  echo "SNAPSHOT_NAME=$1" > "$STATE_FILE"
}

load_snapshot_name() {
  if [[ -n "${SNAPSHOT_NAME:-}" ]]; then
    echo "$SNAPSHOT_NAME"
    return
  fi
  if [[ -f "$STATE_FILE" ]]; then
    # shellcheck disable=SC1090
    source "$STATE_FILE"
    echo "$SNAPSHOT_NAME"
    return
  fi
  err "SNAPSHOT_NAME 미지정. 'snapshot' 단계 먼저 실행하거나 SNAPSHOT_NAME=... 환경변수로 지정하세요."
}

############################
# Stage 1: Serverless 스냅샷 생성
############################
stage_snapshot() {
  local snapshot_name="hwarr-redis-migration-$(date +%Y%m%d-%H%M%S)"
  log "Serverless 캐시 '$SERVERLESS_NAME' 에서 스냅샷 '$snapshot_name' 생성 중..."

  aws elasticache create-serverless-cache-snapshot \
    --serverless-cache-snapshot-name "$snapshot_name" \
    --serverless-cache-name "$SERVERLESS_NAME" \
    --region "$REGION" >/dev/null

  save_snapshot_name "$snapshot_name"
  log "스냅샷 생성 요청 완료. available 될 때까지 폴링..."

  while true; do
    local status
    status=$(aws elasticache describe-serverless-cache-snapshots \
      --serverless-cache-snapshot-name "$snapshot_name" \
      --region "$REGION" \
      --query 'ServerlessCacheSnapshots[0].Status' --output text 2>/dev/null || echo "unknown")
    log "  status=$status"
    case "$status" in
      available) break ;;
      failed)    err "스냅샷 생성 실패" ;;
      *)         sleep 10 ;;
    esac
  done

  log "✓ 스냅샷 준비 완료: $snapshot_name"
  log "  (다음 단계에서 동일 셸이 아니면 SNAPSHOT_NAME=$snapshot_name 로 지정 또는 $STATE_FILE 사용)"
}

############################
# Stage 2: 스냅샷을 S3로 export
############################
ensure_bucket() {
  if aws s3api head-bucket --bucket "$BUCKET" --region "$REGION" >/dev/null 2>&1; then
    log "S3 버킷 '$BUCKET' 이미 존재"
  else
    log "S3 버킷 '$BUCKET' 생성 중..."
    aws s3api create-bucket \
      --bucket "$BUCKET" \
      --region "$REGION" \
      --create-bucket-configuration "LocationConstraint=$REGION" >/dev/null
  fi

  log "ElastiCache 서비스가 export 할 수 있도록 버킷 정책 설정..."
  # NOTE: snapshot export 는 region-specific 서비스 principal 을 요구함
  # (일반 elasticache.amazonaws.com 으로는 'unable to validate access' 에러)
  local policy_file
  policy_file=$(mktemp)
  cat > "$policy_file" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowElastiCacheSnapshotExport",
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
    }
  ]
}
EOF
  aws s3api put-bucket-policy \
    --bucket "$BUCKET" \
    --policy "file://$policy_file" \
    --region "$REGION"
  rm -f "$policy_file"
}

stage_export() {
  local snapshot_name
  snapshot_name=$(load_snapshot_name)

  ensure_bucket

  log "스냅샷 '$snapshot_name' → s3://$BUCKET 로 export 요청..."
  aws elasticache export-serverless-cache-snapshot \
    --serverless-cache-snapshot-name "$snapshot_name" \
    --s3-bucket-name "$BUCKET" \
    --region "$REGION" >/dev/null

  log "S3에 .rdb 파일 도착할 때까지 대기..."
  local rdb_key=""
  for _ in $(seq 1 60); do  # 최대 ~15분
    rdb_key=$(aws s3api list-objects-v2 \
      --bucket "$BUCKET" \
      --query "Contents[?ends_with(Key, '.rdb')] | [0].Key" \
      --output text 2>/dev/null || echo "None")
    if [[ "$rdb_key" != "None" && -n "$rdb_key" ]]; then
      break
    fi
    log "  아직 없음, 15초 후 재시도..."
    sleep 15
  done

  [[ "$rdb_key" == "None" || -z "$rdb_key" ]] && err "RDB export 타임아웃 (s3://$BUCKET 직접 확인)"

  # 클러스터 모드 검증: .rdb 파일 개수가 1개여야 단일 노드로 복원 가능
  local rdb_count
  rdb_count=$(aws s3api list-objects-v2 \
    --bucket "$BUCKET" \
    --query "length(Contents[?ends_with(Key, '.rdb')])" \
    --output text)

  if [[ "$rdb_count" -gt 1 ]]; then
    warn ".rdb 파일이 ${rdb_count}개 export 됨 — Serverless가 클러스터 모드였을 가능성"
    warn "단일 노드 클러스터로는 부분만 복원되므로 SCAN+MIGRATE 방식 검토 필요"
    aws s3 ls "s3://$BUCKET/" --recursive | grep '\.rdb$' >&2
  fi

  local rdb_arn="arn:aws:s3:::$BUCKET/$rdb_key"
  log "✓ Export 완료"
  echo
  echo "============================================================"
  echo " RDB S3 ARN (terraform 변수로 사용):"
  echo
  echo "   $rdb_arn"
  echo
  echo " 다음 단계:"
  echo "   cd $(cd "$(dirname "$0")/.." && pwd)"
  echo "   terraform plan  -var=\"redis_seed_snapshot_arn=$rdb_arn\""
  echo "   terraform apply -var=\"redis_seed_snapshot_arn=$rdb_arn\""
  echo "============================================================"
}

############################
# Cleanup
############################
stage_cleanup_snapshot() {
  local snapshot_name
  snapshot_name=$(load_snapshot_name)
  confirm "ElastiCache 스냅샷 '$snapshot_name' 을 삭제할까요?"
  aws elasticache delete-serverless-cache-snapshot \
    --serverless-cache-snapshot-name "$snapshot_name" \
    --region "$REGION" >/dev/null
  log "✓ 스냅샷 삭제 요청 완료"
  rm -f "$STATE_FILE"
}

stage_cleanup_bucket() {
  confirm "S3 버킷 '$BUCKET' 의 모든 객체와 버킷 자체를 삭제할까요?"
  aws s3 rm "s3://$BUCKET" --recursive --region "$REGION"
  aws s3api delete-bucket --bucket "$BUCKET" --region "$REGION"
  log "✓ 버킷 삭제 완료"
}

############################
# Dispatch
############################
case "${1:-}" in
  snapshot)         stage_snapshot ;;
  export)           stage_export ;;
  all)              stage_snapshot; stage_export ;;
  cleanup-snapshot) stage_cleanup_snapshot ;;
  cleanup-bucket)   stage_cleanup_bucket ;;
  *)
    cat <<EOF
사용법: $0 <subcommand>

  snapshot          Serverless 캐시에서 수동 스냅샷 생성 후 available 까지 대기
  export            가장 최근 스냅샷을 S3로 export, RDB ARN 출력
  all               snapshot + export
  cleanup-snapshot  ElastiCache 스냅샷 삭제 (마이그레이션 완료 후)
  cleanup-bucket    S3 백업 버킷 삭제 (마이그레이션 완료 후)

환경변수:
  REGION           (default: ap-northeast-2)
  SERVERLESS_NAME  (default: astrophage-hwarr-prod-redis)
  BUCKET           (default: astrophage-hwarr-redis-backup-<account_id>)
  SNAPSHOT_NAME    stage 사이 셸이 다를 때 명시적으로 지정
EOF
    exit 1
    ;;
esac
