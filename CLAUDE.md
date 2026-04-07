# CLAUDE.md

## 금지사항

- 커밋 메시지에 `Co-Authored-By`, `Signed-off-by` 등 모든 형태의 서명을 **절대** 포함하지 않는다.
- PR 본문, 이슈 코멘트에도 AI 생성 표시(🤖 Generated with 등)를 포함하지 않는다.
- WIP 커밋 금지. 완성된 단위로만 커밋한다.
- 하나의 커밋에 여러 목적을 섞지 않는다.

## 커밋 컨벤션

```
<타입>(<스코프>): <제목>
```

- **타입**: `feat`, `fix`, `refactor`, `style`, `test`, `docs`, `chore`, `design`
- **스코프**: `client`, `server`, `infra`, `docs` (여러 영역이면 생략 가능)
- 상세 가이드: `docs/commit-guide.md`

## 프로젝트 구조

```
client/          # React 19 + Vite + TanStack Router/Query + Zustand + Tailwind
server/          # Go (Gin) + Socket.IO + Redis
infra/           # Terraform (AWS ECS, S3, CloudFront)
docs/            # Diátaxis 기반 문서
```

## 주요 명령어

```bash
# 프론트엔드
cd client && npm install && npm run dev
cd client && npx tsc --noEmit          # 타입 체크

# 백엔드
cd server && go run .                  # 서버 실행
cd server && go vet ./...              # 린트
cd server && go test ./... -v          # 테스트
```

## 언어

- 문서, 커밋 메시지, PR, 이슈: **한국어**
- 코드, 변수명, 주석: **영어**
