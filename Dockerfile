# ---- 阶段 1：构建前端 ----
FROM node:20-alpine AS webbuilder
WORKDIR /build/web
COPY web/package.json ./
RUN npm install --registry=https://registry.npmmirror.com
COPY web/ ./
RUN npm run build

# ---- 阶段 2：编译 Go 二进制 ----
FROM golang:1.26-alpine AS gobuilder
ENV GO111MODULE=on CGO_ENABLED=0 GOOS=linux
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=webbuilder /build/web/dist ./web/dist
ARG VERSION=dev
RUN go build -trimpath -ldflags "-s -w -X deploy-lab/internal/buildinfo.Version=${VERSION}" -o /deploy-lab .

# ---- 阶段 3：运行镜像 ----
# alpine 而不是 debian：体积更小。装 ca-certificates 是为了以后调外部 HTTPS 接口。
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app app
ENV TZ=Asia/Shanghai
WORKDIR /data
COPY --from=gobuilder /deploy-lab /deploy-lab
# 不用 root 跑服务：容器被攻破时能降低影响面
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O- http://127.0.0.1:8080/api/health || exit 1
ENTRYPOINT ["/deploy-lab"]
