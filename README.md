# deploy-lab

一个用来**练习部署**的最小全栈项目：Go + Gin 后端、Vite 前端、Postgres + Redis、Docker 多阶段构建。后端代码只搭好骨架，数据库接入和业务你自己写。

后端原理详解见 [docs/BACKEND.md](docs/BACKEND.md)。

## 目录

```
main.go                   入口：配置 -> 依赖初始化 -> HTTP 服务 -> 优雅退出
internal/config/          环境变量读取
internal/db/              数据库初始化  ← ★ 留给你实现
internal/api/health.go    健康检查接口
internal/router/          路由 + 中间件 + 前端静态资源
web/                      前端（Vite）
Dockerfile                三阶段构建
docker-compose.yml        app + postgres + redis（独立命名，不碰其他项目）
.env.example              本地运行的环境变量模板
docs/BACKEND.md           后端详解
```

## 这个环境不会碰到你的现有数据库

compose 里所有资源都加了 `deploy-lab` 前缀，与已有环境错开：

| 资源 | 值 |
|---|---|
| compose 项目名 | `deploy-lab` |
| Postgres 容器 / 端口 | `deploy-lab-pg` / **5433** |
| Postgres 库名 / 用户 | `deploy_lab` / `lab` |
| Postgres 数据卷 | `deploy-lab-pgdata` |
| Redis 容器 / 端口 | `deploy-lab-redis` / **6380** |
| 应用容器 / 端口 | `deploy-lab-app` / **8090** |
| 网络 | `deploy-lab-net` |

## 第 0 步：先跑起来（不需要 Docker）

```bash
cd web && npm install && npm run build && cd ..
go mod download
go run .
```

打开 http://localhost:8080 ，点“检查后端健康”，能看到 JSON 返回即成功。

此时 `db.Init` 还是空实现，启动日志里会出现一条 `db.Init 还没实现` 的 warn，属于预期。

## 第 1 步：安装 Docker（你自己装，我不代劳）

Windows 装 Docker Desktop，装完在终端里执行：

```bash
docker version
docker compose version
```

两个都输出信息才算装好。注意 Docker Desktop 需要 WSL2，安装过程中会让你确认。

## 第 2 步：起独立的 Postgres + Redis

**只起依赖，不起 app**（app 里的数据库代码你还没写）：

```bash
docker compose up -d postgres redis
docker compose ps
```

等两个容器都显示 `healthy`。验证连通：

```bash
# 进 psql 看库在不在（密码 labpass）
docker exec -it deploy-lab-pg psql -U lab -d deploy_lab -c "\l"
# 测 redis
docker exec -it deploy-lab-redis redis-cli -a labredis ping
```

从宿主机连的话用 `localhost:5433`（不是 5432）。

## 第 3 步：把数据库接上（你自己写）

打开 [internal/db/db.go](internal/db/db.go)，里面的注释就是你这次的作业清单，核心是：

1. `go get` 三个依赖（注释里给了命令）；
2. `SQL_DSN` 为空走 SQLite，`postgres://` 开头走 Postgres；
3. 设置连接池、`Ping`、`AutoMigrate`；
4. 实现 `Close()`。

写完后用宿主机身份连容器里的库验证：

```bash
# Windows 下用 PowerShell 设置环境变量
$env:SQL_DSN="postgres://lab:labpass@localhost:5433/deploy_lab?sslmode=disable"
go run .
# 另一个终端
docker exec -it deploy-lab-pg psql -U lab -d deploy_lab -c "\dt"
```

能看到你 AutoMigrate 建出来的表就过关。

## 第 4 步：写业务接口

在 [internal/api/](internal/api/) 加 handler，在 [internal/router/router.go](internal/router/router.go) 的 `TODO` 位置注册路由。参考顺序：Create → List → Get → Update → Delete，每写一个就用 `curl` 验一遍。

```bash
curl -X POST http://localhost:8080/api/notes \
  -H "Content-Type: application/json" \
  -d '{"title":"第一条","content":"hello"}'
```

## 第 5 步：构建镜像

```bash
docker build -t deploy-lab:0.1.0 .
docker images deploy-lab
```

重点观察两件事：

- 构建日志里三个阶段依次执行（前端 → Go → 运行镜像）；
- 最终镜像大小（应该在几十 MB 量级）；`docker run --rm deploy-lab:0.1.0 ls /` 里只有二进制，没有源码。

## 第 6 步：整套 compose 跑起来

```bash
docker compose up -d --build
docker compose ps          # app 应该是 healthy
docker compose logs -f app
```

访问 http://localhost:8090 。

注意 app 容器里的 `SQL_DSN` 用的是服务名 `postgres:5432`，不是 `localhost:5433` —— 容器之间走 Docker DNS。

## 第 7 步：故意搞坏它（这才是练部署的核心）

每一项都先**预判**会怎样，再动手，再对照结果：

1. 改 compose 里 postgres 密码但不同步改 `SQL_DSN` → 看 app 怎么报错、容器状态是 `restarting` 还是 `exited`；
2. `docker compose stop postgres` → 再请求 `/api/health`，看健康检查是否变红；
3. `docker compose down` 后 `docker volume ls`，再 `docker compose up -d` → 数据还在不在？然后 `docker volume rm deploy-lab-pgdata` 再起一次，体会无名卷和有名字卷的区别；
4. 把 `ports` 里的 `8090` 改成 `8091` → 验证端口映射改的是什么；
5. `docker exec -it deploy-lab-app sh` 进去看看文件系统，理解为什么容器里没有源码；
6. 修改代码不重新 build 直接 `up` → 观察镜像是不是没变（这是新手最常犯的错，`--build` 才是重新构建）。

## 第 8 步：上服务器 + 反代 + HTTPS

在一台干净的 Linux 服务器（或 WSL）上：

1. 装 Docker，复制项目上去；
2. `docker compose up -d`；
3. 装 Nginx，把 `server { proxy_pass http://127.0.0.1:8090; }` 配上，带上 `Host`、`X-Real-IP`、`X-Forwarded-For`、`X-Forwarded-Proto`；
4. 用 Let's Encrypt（certbot）签发证书，确认 80 跳 443；
5. 验证：`systemctl restart docker` 之后服务能自己起来；服务器重启后也能自己起来（`restart: unless-stopped` 的作用）。

## 常见问题

**`go build` 报 `pattern all:web/dist: no matching files found`**
前端还没构建。`web/dist/index.html` 已经放了一个占位文件，正常不会触发；如果你清理了它，先 `cd web && npm run build`。

**容器里连不上数据库**
先确认：容器内用服务名 `postgres:5432`；宿主机用 `localhost:5433`；`sslmode=disable` 写了。

**端口被占用**
`docker compose down` 后检查 `netstat -ano | findstr :5433`，或者直接改 compose 里的宿主端口。

**改了代码容器里没变**
镜像不会自动更新，`docker compose up -d --build` 才会重建。
