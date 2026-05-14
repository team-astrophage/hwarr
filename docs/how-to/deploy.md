# 배포 가이드

화르르(hwarr) 프로젝트의 배포 파이프라인과 인프라 관리 방법을 설명한다.

---

## 1. GitHub Actions 워크플로우 전체 개요

GitHub Actions 워크플로우 5개가 CI/CD 및 자동화를 담당한다.

### 배포 워크플로우

| 워크플로우 | 파일 | 트리거 | 역할 |
|---|---|---|---|
| **CI** | `.github/workflows/ci.yml` | `main` branch push / PR (`server/**` 변경 시) | Lint (go vet/gofmt) + Test (go test) |
| **Backend CI/CD** | `.github/workflows/backend-deploy.yml` | `main` branch push (`server/**` 변경 시) / 수동 | Docker build → ECR push → ECS deploy |
| **Frontend CI/CD** | `.github/workflows/frontend-deploy.yml` | `main` branch push (`client/**` 변경 시) / 수동 | Vite build → S3 sync → CloudFront invalidation |

Backend/Frontend 워크플로우는 `workflow_dispatch`를 지원하므로 GitHub Actions UI에서 수동 실행이 가능하다.
Backend 워크플로우에는 `concurrency` 설정이 적용되어 동일 branch에 대한 중복 배포를 자동 취소한다.

### 자동화 워크플로우

| 워크플로우 | 파일 | 트리거 | 역할 |
|---|---|---|---|
| **Docs Auto-Update** | `.github/workflows/docs-update.yml` | `main` 대상 PR 생성/업데이트 시 (`docs/**` 제외) | Claude Code로 문서 자동 업데이트 |
| **PR Labeler** | `.github/workflows/pr-labeler.yml` | PR 생성/업데이트 시 | 변경 파일 경로 기반 자동 라벨링 |

---

## 2. 백엔드 배포

### 배포 흐름

```
git push (server/** 변경)
  → GitHub Actions 트리거
    → OIDC로 AWS 인증 (assume role)
    → ECR 로그인
    → Docker build & push (tag: commit SHA + latest)
    → 현재 ECS task definition 다운로드
    → 새 image로 task definition 갱신
    → ECS service 업데이트 (rolling deploy)
    → 서비스 안정성 대기 (최대 5분)
    → Discord 알림 전송
```

### Docker 이미지

`server/Dockerfile` 기반으로 빌드된다.

- Multi-stage build: Go 빌드 후 Alpine 3.20 런타임 이미지 사용
- 행정동 GeoJSON 데이터를 build 시 다운로드하여 `data/` 에 포함
- 엔트리포인트: `./hwarr-server` (Go 바이너리)
- Health check: `/health` 엔드포인트

### ECS 구성

| 항목 | 값 |
|---|---|
| Cluster | `astrophage-hwarr-prod-cluster` |
| Service | `astrophage-hwarr-prod-backend` |
| Launch type | Fargate |
| CPU / Memory | 256 (0.25 vCPU) / 512 MiB |
| Desired count | 1 |
| Network | Private subnet, ALB를 통한 트래픽 수신 |

ECS service의 `task_definition`은 Terraform에서 `ignore_changes`로 설정되어 있어 CI/CD가 task definition을 업데이트해도 Terraform drift가 발생하지 않는다.

---

## 3. 프론트엔드 배포

### 배포 흐름

```
git push (client/** 변경)
  → GitHub Actions 트리거
    → Node.js 설정
    → npm ci (의존성 설치)
    → tsc -b && vite build (TypeScript 컴파일 + Vite 번들링)
    → OIDC로 AWS 인증
    → S3 sync (dist/ → S3 bucket)
    → CloudFront cache invalidation (/*)
    → Discord 알림 전송
```

### S3 캐시 전략

| 파일 | Cache-Control |
|---|---|
| JS, CSS 등 해시된 에셋 | `public, max-age=31536000, immutable` (1년) |
| `index.html` | `no-cache, no-store, must-revalidate` |
| `*.json` (manifest 등) | `no-cache, no-store, must-revalidate` |

`--delete` 플래그로 이전 빌드의 잔여 파일을 자동 정리한다.

### CloudFront 라우팅

CloudFront distribution은 단일 도메인(`hwarr.com`)에서 프론트엔드와 백엔드를 모두 서빙한다.

| 경로 패턴 | Origin | 캐싱 |
|---|---|---|
| `/` (기본) | S3 (정적 파일) | TTL 최대 1년 |
| `/api/*` | ALB (REST API) | 캐싱 비활성화 (TTL=0) |
| `/socket.io/*` | ALB (WebSocket) | 캐싱 비활성화, 모든 헤더/쿠키 전달 |
| `/health` | ALB | 캐싱 비활성화 |

SPA 라우팅을 위해 403/404 응답 시 `/index.html`로 리다이렉트한다.

---

## 4. Terraform 인프라 관리

### 디렉토리 구조

```
infra/
  bootstrap/        # Terraform remote state용 S3 + DynamoDB (최초 1회 실행)
  main.tf           # Data sources, locals (SSM prefix 등)
  versions.tf       # Provider 설정, S3 backend
  variables.tf      # 입력 변수 정의
  networking.tf     # VPC, Subnet, Security Groups
  compute.tf        # ECR, ECS, ALB, IAM roles
  storage.tf        # S3, CloudFront, ElastiCache (Redis Serverless)
  dns.tf            # Route53, ACM 인증서
  budget.tf         # AWS Budget 알림
  outputs.tf        # 주요 리소스 ID/URL 출력
  terraform.tfvars.example  # 변수 예시
```

### 초기 설정

```bash
# 1. Remote state 인프라 생성 (최초 1회)
cd infra/bootstrap
terraform init
terraform apply

# 2. 메인 인프라 프로비저닝
cd infra
cp terraform.tfvars.example terraform.tfvars
# terraform.tfvars 편집 (budget_alert_email 등)
terraform init
terraform apply
```

### State 관리

- Backend: S3 (`astrophage-hwarr-tfstate` bucket)
- Lock: DynamoDB (`astrophage-hwarr-tflock` table)
- 암호화 활성화 (`encrypt = true`)

### 주요 리소스 요약

| 리소스 | 서비스 | 비고 |
|---|---|---|
| VPC | `10.0.0.0/16` | Public 2개 + Private 2개 subnet |
| ALB | Public subnet | WebSocket sticky session 활성화 (24h) |
| ECS Fargate | Public subnet (`assignPublicIp`) | 단일 task. NAT Gateway 제거로 월 ~$40 절감 — 보안은 SG 기반 (ALB SG → ECS SG ingress만) |
| ECR | - | Lifecycle policy: 최근 5개 이미지만 보관 |
| ElastiCache | Redis Serverless v7 | TLS 필수 (`rediss://`), Private subnet |
| S3 | Frontend 정적 파일 | OAC를 통한 CloudFront 전용 접근 |
| CloudFront | CDN | 커스텀 도메인 + ACM (us-east-1) |
| Route53 | DNS | `hwarr.com` + `www.hwarr.com` |
| Budget | 월 $50 | 50%, 80%, 100% 알림 |

---

## 5. 필요한 GitHub Secrets

| Secret 이름 | 설명 | 예시 |
|---|---|---|
| `AWS_ROLE_ARN` | GitHub Actions OIDC용 IAM role ARN | `arn:aws:iam::123456789012:role/astrophage-github-actions` |
| `DISCORD_WEBHOOK_URL` | 배포 알림용 Discord webhook URL | `https://discord.com/api/webhooks/...` |
| `CLOUDFRONT_DISTRIBUTION_ID` | CloudFront distribution ID | `E1234ABCDEF` |
| `API_URL` | 백엔드 API URL (Vite build 시 주입) | `https://hwarr.com` |
| `WS_URL` | WebSocket URL (Vite build 시 주입) | `wss://hwarr.com` |

> AWS 인증은 OIDC (OpenID Connect)를 사용한다. Access key가 아닌 `role-to-assume` 방식이므로 별도 AWS access key secret은 불필요하다.

---

## 6. 수동 배포 방법 (긴급 시)

### GitHub Actions UI에서 수동 실행

모든 배포 워크플로우가 `workflow_dispatch`를 지원한다.

1. GitHub 저장소 > **Actions** 탭
2. 좌측에서 워크플로우 선택 (`Backend CI/CD` 또는 `Frontend CI/CD`)
3. **Run workflow** 버튼 클릭 > branch 선택 > 실행

### CLI에서 백엔드 수동 배포

```bash
# AWS 인증 (SSO 등)
aws sso login --profile your-profile

# 1. Docker build & ECR push
cd server
aws ecr get-login-password --region ap-northeast-2 | \
  docker login --username AWS --password-stdin <ACCOUNT_ID>.dkr.ecr.ap-northeast-2.amazonaws.com

IMAGE_TAG=$(git rev-parse --short HEAD)
ECR_REPO=<ACCOUNT_ID>.dkr.ecr.ap-northeast-2.amazonaws.com/astrophage-hwarr-prod-backend

docker build -t $ECR_REPO:$IMAGE_TAG -t $ECR_REPO:latest .
docker push $ECR_REPO:$IMAGE_TAG
docker push $ECR_REPO:latest

# 2. ECS force deploy (현재 latest 이미지로 재배포)
aws ecs update-service \
  --cluster astrophage-hwarr-prod-cluster \
  --service astrophage-hwarr-prod-backend \
  --force-new-deployment \
  --region ap-northeast-2
```

### CLI에서 프론트엔드 수동 배포

```bash
# 1. Build
cd client
VITE_API_URL=https://hwarr.com VITE_WS_URL=wss://hwarr.com npm run build

# 2. S3 sync
aws s3 sync dist/ s3://astrophage-hwarr-frontend-unique-suffix/ \
  --delete \
  --cache-control "public, max-age=31536000, immutable" \
  --exclude "index.html" --exclude "*.json"

aws s3 cp dist/index.html s3://astrophage-hwarr-frontend-unique-suffix/index.html \
  --cache-control "no-cache, no-store, must-revalidate"

# 3. CloudFront invalidation
aws cloudfront create-invalidation \
  --distribution-id <DISTRIBUTION_ID> \
  --paths "/*"
```

---

## 7. 환경변수 관리 (SSM Parameter Store)

민감한 값은 AWS SSM Parameter Store에 SecureString으로 저장하고, ECS task definition의 `secrets` 블록을 통해 컨테이너에 주입된다.

### SSM 경로 규칙

```
/astrophage/hwarr/prod/<parameter_name>
```

### 현재 등록된 파라미터

| SSM 경로 | 컨테이너 환경변수 | 용도 |
|---|---|---|
| `/astrophage/hwarr/prod/feedback_discord_webhook_url` | `FEEDBACK_DISCORD_WEBHOOK_URL` | 사용자 피드백 Discord webhook |

### 파라미터 등록/변경

`scripts/put-feedback-secrets.sh` 스크립트를 사용한다.

```bash
# 최초 등록
export DISCORD_WEBHOOK_URL='https://discord.com/api/webhooks/XXXX/YYYY'
./scripts/put-feedback-secrets.sh

# 값 변경 (기존 파라미터 덮어쓰기)
OVERWRITE=1 ./scripts/put-feedback-secrets.sh
```

### 파라미터 확인

```bash
aws ssm get-parameter \
  --name '/astrophage/hwarr/prod/feedback_discord_webhook_url' \
  --with-decryption \
  --region ap-northeast-2 \
  --query 'Parameter.Value' --output text
```

### 새 파라미터 추가 시 체크리스트

1. SSM Parameter Store에 SecureString으로 값 등록
2. `infra/compute.tf`의 ECS task definition `secrets` 블록에 항목 추가
3. ECS execution role에 해당 SSM 경로에 대한 `ssm:GetParameters` 권한 확인
4. ECS service 재배포 (새 task definition 반영)

### 비민감 환경변수

SSM을 거치지 않는 일반 환경변수는 task definition의 `environment` 블록에 직접 정의되어 있다.

| 환경변수 | 값 | 설명 |
|---|---|---|
| `PORT` | `8000` | 컨테이너 리스닝 포트 |
| `ENVIRONMENT` | `prod` | 실행 환경 |
| `REDIS_URL` | `rediss://<endpoint>:6379/0` | ElastiCache Serverless (TLS) |

---

## 8. Discord 배포 알림

백엔드와 프론트엔드 배포 워크플로우 모두 마지막 단계에서 Discord로 결과를 전송한다.

### 알림 형식

- 성공 시: 초록색 embed, "Backend/Frontend Deploy SUCCESS"
- 실패 시: 빨간색 embed, "Backend/Frontend Deploy FAILED"
- 포함 정보: commit SHA (7자리), branch 이름, 실행한 사용자

### 알림 조건

`if: always()` 로 설정되어 있어 배포 성공/실패에 관계없이 항상 전송된다.

### 설정

Discord 서버에서 webhook URL을 생성하고 GitHub Secrets의 `DISCORD_WEBHOOK_URL`에 등록한다.

```
Discord 서버 설정 > Integrations > Webhooks > New Webhook > Copy URL
```

---

## 9. Docs Auto-Update 워크플로우

PR이 `main` 브랜치를 대상으로 생성되거나 업데이트되면, Claude Code가 변경된 코드를 분석하여 관련 문서를 자동으로 업데이트한다.

### 동작 흐름

```
PR 생성/업데이트 (main 대상, docs/** 외 파일 변경)
  → PR 브랜치 checkout
  → 변경된 파일 목록 추출 (git diff)
  → Claude Code 실행
    → 변경 사항 분석
    → docs/ 관련 문서 업데이트
    → README.md 동기화 확인
    → PR 브랜치에 커밋 추가
  → Discord 알림 전송
```

### 트리거 조건

- `main` 대상 PR의 `opened`, `synchronize` 이벤트
- `docs/**`와 `.github/workflows/docs-update.yml` 변경만 있는 PR은 **제외** (`paths-ignore`)
- PR 제목에 `[skip-docs]`를 포함하면 **스킵**

### 동시성 제어

동일 PR에 대해 `concurrency` 그룹이 설정되어 있어, PR에 새 커밋이 추가되면 진행 중이던 이전 실행을 자동 취소한다.

### Claude Code가 수행하는 작업

1. 변경된 파일을 읽고 내용 파악
2. `docs/` 디렉토리에서 관련 문서 탐색
3. 관련 문서가 있으면 변경사항 반영하여 업데이트
4. 새 기능 추가 시 적절한 문서 신규 작성
5. 단순 리팩토링/스타일 변경이면 아무 변경 없이 종료
6. docs/ 업데이트 시 README.md의 기술 스택, 프로젝트 구조 등도 함께 동기화

### 필요한 Secret

| Secret 이름 | 설명 |
|---|---|
| `ANTHROPIC_API_KEY` | Anthropic API 키 (Claude Code 실행용) |
| `DISCORD_WEBHOOK_URL` | 결과 알림용 Discord webhook URL |

### 권한

`GITHUB_TOKEN`을 명시적으로 전달하며, 별도 GitHub App 설치 없이 동작한다.

```yaml
permissions:
  contents: write       # PR 브랜치에 커밋 푸시
  pull-requests: write  # PR 코멘트
```

---

## 10. PR Labeler 워크플로우

PR이 생성되거나 업데이트되면 변경된 파일 경로를 기반으로 자동으로 라벨을 부여한다.

### 라벨 규칙

`.github/labeler.yml`에 정의된 규칙:

| 라벨 | 트리거 경로 |
|---|---|
| `scope/client` | `client/**` |
| `scope/server` | `server/**` |
| `scope/infra` | `infra/**`, `.github/workflows/**`, `Dockerfile`, `docker-compose*.yml` |
| `scope/docs` | `docs/**` |

### 설정

- `sync-labels: false` — 이미 부여된 라벨은 제거하지 않고, 새로 매칭되는 라벨만 추가한다.
- `pull_request_target` 이벤트를 사용하여 fork에서 온 PR에도 라벨링이 동작한다.
