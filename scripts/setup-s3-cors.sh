#!/usr/bin/env bash
#
# One-time setup: installs AWS CLI (if missing) and configures CORS
# on the Hetzner S3 bucket so browsers can upload/download via presigned URLs.
#
# Usage:
#   ./scripts/setup-s3-cors.sh
#
# Reads S3 credentials from server/.env (or environment variables).
# ──────────────────────────────────────────────────────────────────

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/../server/.env"

# ── Load .env if present ────────────────────────────────────────
if [[ -f "$ENV_FILE" ]]; then
  echo "Loading env from $ENV_FILE"
  set -a
  # shellcheck source=/dev/null
  source "$ENV_FILE"
  set +a
fi

# ── Validate required vars ──────────────────────────────────────
: "${AWS_ACCESS_KEY_ID:?Set AWS_ACCESS_KEY_ID in server/.env}"
: "${AWS_SECRET_ACCESS_KEY:?Set AWS_SECRET_ACCESS_KEY in server/.env}"
: "${AWS_S3_BUCKET_NAME:?Set AWS_S3_BUCKET_NAME in server/.env}"
: "${AWS_REGION:?Set AWS_REGION in server/.env}"
: "${AWS_S3_ENDPOINT_URL:?Set AWS_S3_ENDPOINT_URL in server/.env}"

# ── Install AWS CLI if not present ──────────────────────────────
if ! command -v aws &>/dev/null; then
  echo "AWS CLI not found. Installing..."
  apt-get update -qq && apt-get install -y -qq awscli 2>/dev/null \
    || (curl -s "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o /tmp/awscliv2.zip \
        && unzip -qo /tmp/awscliv2.zip -d /tmp \
        && /tmp/aws/install \
        && rm -rf /tmp/awscliv2.zip /tmp/aws)
  echo "AWS CLI installed: $(aws --version)"
fi

# ── Configure AWS credentials ──────────────────────────────────
export AWS_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY
export AWS_DEFAULT_REGION="$AWS_REGION"

# ── Detect server public IP ────────────────────────────────────
SERVER_IP=$(curl -s --max-time 5 https://ifconfig.me/ip || echo "")
if [[ -n "$SERVER_IP" ]]; then
  echo "Server public IP: $SERVER_IP"
else
  echo "Warning: could not detect server public IP"
fi

# ── Build allowed origins ──────────────────────────────────────
ORIGINS=(
  "https://helpin.ai"
  "https://stage.helpin.ai"
  "http://localhost:5173"
)
if [[ -n "$SERVER_IP" ]]; then
  ORIGINS+=("http://${SERVER_IP}:5173")
  ORIGINS+=("http://${SERVER_IP}")
fi
# Add CORS_ORIGINS from env if set (comma-separated)
if [[ -n "${CORS_ORIGINS:-}" ]]; then
  IFS=',' read -ra EXTRA_ORIGINS <<< "$CORS_ORIGINS"
  for o in "${EXTRA_ORIGINS[@]}"; do
    o="$(echo "$o" | xargs)"  # trim whitespace
    [[ -n "$o" ]] && ORIGINS+=("$o")
  done
fi

# Deduplicate
ORIGINS=($(printf "%s\n" "${ORIGINS[@]}" | sort -u))

echo ""
echo "Configuring CORS on bucket: $AWS_S3_BUCKET_NAME"
echo "Endpoint: $AWS_S3_ENDPOINT_URL"
echo "Allowed origins:"
for o in "${ORIGINS[@]}"; do echo "  - $o"; done
echo ""

# ── Build CORS JSON ────────────────────────────────────────────
ORIGINS_JSON=$(printf '"%s",' "${ORIGINS[@]}")
ORIGINS_JSON="[${ORIGINS_JSON%,}]"

CORS_CONFIG=$(cat <<EOF
{
  "CORSRules": [
    {
      "AllowedOrigins": ${ORIGINS_JSON},
      "AllowedMethods": ["GET", "PUT"],
      "AllowedHeaders": ["*"],
      "ExposeHeaders": ["ETag"],
      "MaxAgeSeconds": 3600
    }
  ]
}
EOF
)

# ── Apply CORS ──────────────────────────────────────────────────
aws s3api put-bucket-cors \
  --bucket "$AWS_S3_BUCKET_NAME" \
  --endpoint-url "$AWS_S3_ENDPOINT_URL" \
  --cors-configuration "$CORS_CONFIG"

echo "CORS configured successfully!"
echo ""

# ── Verify ──────────────────────────────────────────────────────
echo "Verifying CORS configuration:"
aws s3api get-bucket-cors \
  --bucket "$AWS_S3_BUCKET_NAME" \
  --endpoint-url "$AWS_S3_ENDPOINT_URL"
