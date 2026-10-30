# 1. 构建前端（构建上下文 = 仓库根，Dockerfile 位于根目录）
FROM node:20-alpine AS frontend
WORKDIR /build
RUN corepack enable && corepack prepare pnpm@9.15.9 --activate
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY packages/api-client/package.json packages/api-client/package.json
COPY apps/h5/package.json apps/h5/package.json
COPY apps/admin/package.json apps/admin/package.json
RUN pnpm install --frozen-lockfile --ignore-scripts
COPY packages packages
COPY apps apps
RUN pnpm --filter @canteen/h5 build && pnpm --filter @canteen/admin build

# 2. 构建 Go（embed 用 all: 前缀；源码内 cmd/api/static 放 .gitkeep 保证空目录可嵌入）
FROM golang:1.22-alpine AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
COPY --from=frontend /build/apps/h5/dist     ./cmd/api/static/h5
COPY --from=frontend /build/apps/admin/dist  ./cmd/api/static/admin
RUN CGO_ENABLED=0 go build -o /app ./cmd/api

# 3. 运行
FROM alpine:3.20
RUN apk add --no-cache tzdata ca-certificates && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
COPY --from=server /app /app
ENV TZ=Asia/Shanghai
EXPOSE 8080
ENTRYPOINT ["/app"]
