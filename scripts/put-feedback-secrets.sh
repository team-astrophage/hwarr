#!/usr/bin/env bash
#
# Store feedback/SMTP secrets in AWS SSM Parameter Store.
#
# Usage:
#   ./scripts/put-feedback-secrets.sh
#
# Environment variables (required):
#   SMTP_USER         — Gmail address (e.g. you@gmail.com)
#   SMTP_PASSWORD     — Gmail App Password (16 chars, spaces OK)
#   DEVELOPER_EMAIL   — Where feedback emails are delivered
#
# Environment variables (optional):
#   AWS_REGION        — default: ap-northeast-2
#   SSM_PREFIX        — default: /astrophage/hwarr/prod
#   OVERWRITE         — "1" to update existing params; otherwise create-only


# 실행 예시

#   최초 생성:
#   export SMTP_USER='your-gmail@gmail.com'
#   export SMTP_PASSWORD='abcd efgh ijkl mnop'
#   export DEVELOPER_EMAIL='your-gmail@gmail.com'

#   ./scripts/put-feedback-secrets.sh

#   값 변경 (예: 비밀번호 교체):
#   export SMTP_USER='your-gmail@gmail.com'
#   export SMTP_PASSWORD='새로운 앱 비밀번호'
#   export DEVELOPER_EMAIL='your-gmail@gmail.com'
#   OVERWRITE=1 ./scripts/put-feedback-secrets.sh

set -euo pipefail

AWS_REGION="${AWS_REGION:-ap-northeast-2}"
SSM_PREFIX="${SSM_PREFIX:-/astrophage/hwarr/prod}"
OVERWRITE="${OVERWRITE:-0}"

# ---- validate inputs ----
missing=()
[[ -z "${SMTP_USER:-}" ]]       && missing+=("SMTP_USER")
[[ -z "${SMTP_PASSWORD:-}" ]]   && missing+=("SMTP_PASSWORD")
[[ -z "${DEVELOPER_EMAIL:-}" ]] && missing+=("DEVELOPER_EMAIL")

if (( ${#missing[@]} > 0 )); then
  echo "❌ missing required env vars: ${missing[*]}" >&2
  echo "" >&2
  echo "example:" >&2
  echo "  export SMTP_USER='you@gmail.com'" >&2
  echo "  export SMTP_PASSWORD='xxxx xxxx xxxx xxxx'" >&2
  echo "  export DEVELOPER_EMAIL='you@gmail.com'" >&2
  echo "  ./scripts/put-feedback-secrets.sh" >&2
  exit 1
fi

overwrite_flag=""
if [[ "$OVERWRITE" == "1" ]]; then
  overwrite_flag="--overwrite"
fi

put() {
  local name="$1"
  local value="$2"
  echo "→ ${SSM_PREFIX}/${name}"
  aws ssm put-parameter \
    --region "$AWS_REGION" \
    --name "${SSM_PREFIX}/${name}" \
    --value "$value" \
    --type SecureString \
    $overwrite_flag \
    --output text \
    --query 'Version' > /dev/null
}

echo "Writing SecureString params to SSM (region=${AWS_REGION}, prefix=${SSM_PREFIX})"
echo "OVERWRITE=${OVERWRITE} (set OVERWRITE=1 to update existing)"
echo ""

put "smtp_user"       "$SMTP_USER"
put "smtp_password"   "$SMTP_PASSWORD"
put "developer_email" "$DEVELOPER_EMAIL"

echo ""
echo "✅ done. Verify:"
echo "  aws ssm get-parameters-by-path \\"
echo "    --path '${SSM_PREFIX}/' --with-decryption \\"
echo "    --region ${AWS_REGION} \\"
echo "    --query 'Parameters[].[Name,Value]' --output table"
