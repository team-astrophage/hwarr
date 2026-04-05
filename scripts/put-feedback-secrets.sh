#!/usr/bin/env bash
#
# Store feedback Discord webhook URL in AWS SSM Parameter Store.
#
# Usage:
#   ./scripts/put-feedback-secrets.sh
#
# Environment variables (required):
#   DISCORD_WEBHOOK_URL  — Discord incoming webhook URL
#                          (Channel Settings → Integrations → Webhooks → New Webhook → Copy URL)
#
# Environment variables (optional):
#   AWS_REGION   — default: ap-northeast-2
#   SSM_PREFIX   — default: /astrophage/hwarr/prod
#   OVERWRITE    — "1" to update existing param; otherwise create-only
#
# 실행 예시:
#   최초 생성:
#     export DISCORD_WEBHOOK_URL='https://discord.com/api/webhooks/XXXX/YYYY'
#     ./scripts/put-feedback-secrets.sh
#
#   값 변경:
#     export DISCORD_WEBHOOK_URL='https://discord.com/api/webhooks/...'
#     OVERWRITE=1 ./scripts/put-feedback-secrets.sh

set -euo pipefail

AWS_REGION="${AWS_REGION:-ap-northeast-2}"
SSM_PREFIX="${SSM_PREFIX:-/astrophage/hwarr/prod}"
OVERWRITE="${OVERWRITE:-0}"

# ---- validate inputs ----
if [[ -z "${DISCORD_WEBHOOK_URL:-}" ]]; then
  echo "❌ missing required env var: DISCORD_WEBHOOK_URL" >&2
  echo "" >&2
  echo "example:" >&2
  echo "  export DISCORD_WEBHOOK_URL='https://discord.com/api/webhooks/XXXX/YYYY'" >&2
  echo "  ./scripts/put-feedback-secrets.sh" >&2
  exit 1
fi

if [[ "$DISCORD_WEBHOOK_URL" != https://discord.com/api/webhooks/* ]] \
   && [[ "$DISCORD_WEBHOOK_URL" != https://discordapp.com/api/webhooks/* ]]; then
  echo "⚠️  DISCORD_WEBHOOK_URL doesn't look like a Discord webhook URL:" >&2
  echo "    $DISCORD_WEBHOOK_URL" >&2
  echo "    proceeding anyway..." >&2
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

put "feedback_discord_webhook_url" "$DISCORD_WEBHOOK_URL"

echo ""
echo "✅ done. Verify:"
echo "  aws ssm get-parameter \\"
echo "    --name '${SSM_PREFIX}/feedback_discord_webhook_url' \\"
echo "    --with-decryption --region ${AWS_REGION} \\"
echo "    --query 'Parameter.Value' --output text"
