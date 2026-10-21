# deploy-lab 后端说明

这份文档以你手头这个最小项目为样本，讲清楚一个 Go 后端**从启动到处理完一个请求**之间都发生了什么，以及在这个项目里每一块代码该放在哪、为什么这么放。部署操作步骤见 [README](../README.md)。

---

## 1. 项目结构与职责划分

```
deploy-lab/
├── main.go                  进程入口：读配置 -> 初始化依赖 -> 起 HTTP 服务 -> 优雅退出
├── internal/
│   ├── config/config.go     环境变量 -> Config 结构体
│   ├── buildinfo/           编译期注入的版本号
│   ├── db/db.go             数据库/Redis 初始化（**留给你实现**）
│   ├── api/health.go        HTTP 处理器（handler）
│   └── router/router.go     路由注册 + 中间件 + 静态资源兜底
├── web/                     前端，构建产物 dist/ 会被编进二进制
├── Dockerfile               三阶段构建
├── docker-compose.yml       app + postgres + redis
└── docs/BACKEND.md          本文档
```

`internal/` 是 Go 的目录约定：这个目录下的包只能被**同一个模块内**的代码 import，外部模块 import 不了，等于天然的访问控制。公共库才放 `pkg/`。

分层原则（和你熟的前端分层是一一对应的）：

| 后端概念 | 职责 | 前端类比 |
|---|---|---|
| router | URL -> handler 的映射表 | 路由表 |
| middleware | 请求进入 handler 前/后的横切逻辑 | axios 拦截器 |
| api / handler | 解析参数、调业务、写响应 | 页面组件里的请求函数 |
| service | 业务规则（本项目暂时没拆） | 状态管理 / store |
| model | 数据结构 + 数据库操作 | TS interface + api 层 |

现在项目小，handler 里直接操作数据库没问题。**但只要你发现 handler 超过 50 行、或者两个 handler 要复用同一段逻辑，就该拆 service 层。** 拆分的信号是复用和长度，不是“架构规范”。

---

## 2. 一次请求的完整生命周期

以 `GET /api/health` 为例：

```
main.go
  └─ config.Load()                 读环境变量
  └─ db.Init(cfg)                  连数据库（你来实现）
  └─ router.New(buildFS)           组装路由和中间件
       └─ gin.New()
       └─ r.Use(gin.Recovery())    panic 时返回 500，进程不退出
       └─ r.Use(requestLogger)     after 阶段记录耗时和状态码
  └─ srv.ListenAndServe()          监听端口，每个请求一个 goroutine
                 │
                 ▼  请求到达
       Recovery -> requestLogger -> 路由匹配 -> api.Health
                                              └─ c.JSON(200, ...)
       requestLogger 的 c.Next() 返回，打印日志并 flush 响应
```

几个关键点：

**每个请求跑在独立的 goroutine 里。** 这是 Go HTTP 服务的基本模型，`sync.Mutex` 锁的是数据结构而不是请求。好处是天然并发；代价是**同一个全局变量被两个请求同时读写时会出数据竞争**（见第 9 节）。不要用共享变量存请求级别的数据，要用 `gin.Context`。

**中间件的顺序就是代码顺序。** `Recovery` 必须最外层，否则 handler panic 时内层中间件已经被跳过，日志和响应都写不出去。鉴权必须在业务路由之前注册，静态资源兜底（`NoRoute`）永远最后。

**`c.Next()` 是分界线。** 中间件函数里 `c.Next()` 之前的代码在 handler 执行前跑，之后的在 handler 执行后跑。计时、日志、改响应头都靠这个机制。

**`gin.Context` 是一次请求的上下文对象**，等价于前端的 `req` + `res`，外加一个 `Set/Get` 用的键值袋（用来在中间件之间传数据，比如 `c.Set("userID", 1)`）。

---

## 3. 配置：为什么全部走环境变量

[internal/config/config.go](../internal/config/config.go) 里没有任何写死的地址或密码，全部来自环境变量。

原因很实际：同一份二进制要能在你本机、Docker 容器、测试服务器、生产服务器上跑，**只有配置不同**。这就是十二要素应用里的「配置与代码分离」。如果密码写进代码：

- 会被提交进 git，永久留在历史里；
- 换环境必须重新编译；
- 无法做多副本差异化部署。

本地开发用 `.env.example` 拷成 `.env`，用 shell 或 `godotenv` 加载。注意 `.gitignore` 必须排除 `.env`。

`config.Load()` 里用了 `os.LookupEnv` 而不是 `os.Getenv`，因为要区分三种状态：

- 变量不存在；
- 变量存在但是空字符串；
- 变量有值。

比如 `SQL_DSN=` 显式设为空，和完全不设，语义可以不同（本项目里都当作“不配库”）。这个区分在排查“为什么读不到配置”时很有用。

---

## 4. 数据库（这一块是你主要要练的）

### 4.1 连接字符串 DSN

PostgreSQL 的 DSN 格式：

```
postgres://用户:密码@主机:端口/库名?sslmode=disable
```

项目里 compose 给的是：

```
postgres://lab:labpass@postgres:5432/deploy_lab?sslmode=disable
```

注意几个容易踩的点：

- **`postgres` 是 compose 里的服务名**，不是 `localhost`。容器之间通过 Docker 内置 DNS 用服务名互相访问。只有从**宿主机**连，才用 `localhost:5433`。
- **`sslmode=disable`**：很多驱动默认要求 SSL，本地/内网环境关掉，生产走内网时也关，走公网必须开。连不上时报 `SSL is not enabled on the server` 就是这个参数的问题。
- **密码里如果有 `@`、`:`、`/`** 必须做 URL 编码，否则解析错位。

SQLite 的 DSN 就是文件路径（本项目里是 `lab.db`）。它的特点是**零配置、单文件**，非常适合本地开发；但它**不支持多副本并发写**，所以生产环境不能用。

### 4.2 连接池：上生产前的必修项

GORM 底层是 `database/sql` 的连接池。不给参数，默认 `MaxOpenConns` 是**无限**，并发一高就会把数据库的连接数打满，导致整个库不可用。

[internal/db/db.go](../internal/db/db.go) 的注释里让你加这三行：

```go
sqlDB.SetMaxOpenConns(20)     // 同时最多 20 个连接
sqlDB.SetMaxIdleConns(10)     // 空闲最多保留 10 个，多了会被回收
sqlDB.SetConnMaxLifetime(time.Hour) // 连接最长活 1 小时，防中间层掐断
```

经验值：

- 单实例 10~30 个连接足够撑住普通业务；
- **总连接数 = 单实例连接数 × 实例数**，不能超过 Postgres 的 `max_connections`（默认 100）；
- `ConnMaxLifetime` 一定要设，否则连接可能被防火墙/NAT 静默掐断，之后所有请求都报 `broken pipe`。

改完之后你应该能看到一个现象：并发压测时数据库连接数稳定在上限附近而不是无限涨。用 `docker exec deploy-lab-pg psql -U lab -d deploy_lab -c "select count(*) from pg_stat_activity"` 观察。

### 4.3 建表：AutoMigrate 与 SQL 迁移

GORM 的 `AutoMigrate` 会根据 struct 自动建表/加列，开发阶段很方便。

```go
db.AutoMigrate(&Note{})
```

它**只增不减**：加列会加，删列、改类型不会做。所以：

- 本地开发、玩具项目：可以直接用；
- 有真实数据之后：应该换成 SQL 迁移文件（常见工具：`golang-migrate`、`goose`），每次变更一个带版本号的 `xxx.up.sql`，可回滚、可审计。

**你至少要理解一件事**：线上改表的时候，`ALTER TABLE` 可能锁表，大表上执行会让服务卡住几十秒到几十分钟。所以真实项目里加索引、改字段通常都在低峰期跑，并且用 `CREATE INDEX CONCURRENTLY` 这类不锁表的写法。

### 4.4 事务

什么时候必须用事务：**一组写操作要么全成功要么全失败**。典型场景是转账（扣钱 + 加钱）、下单（扣库存 + 建订单）。

GORM 的写法：

```go
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&order).Error; err != nil {
        return err         // 返回 error 自动回滚
    }
    if err := tx.Model(&stock).Update("count", gorm.Expr("count - ?", 1)).Error; err != nil {
        return err
    }
    return nil             // 返回 nil 自动提交
})
```

注意：**事务里的所有操作必须用 `tx` 而不是外层的 `db`**，用 `db` 就跑到事务外面去了，这是最常见的错误。`Update` 里的扣减必须用 SQL 表达式 `count - 1` 而不是先读后写，否则并发下会丢更新。

### 4.5 索引和查询

- 经常作为查询条件的字段（`WHERE`、`ORDER BY`、`JOIN`）要建索引；
- 联合索引遵循**最左前缀**原则：`(user_id, created_at)` 能支持 `WHERE user_id=?`，也能支持 `WHERE user_id=? ORDER BY created_at`，但单独的 `created_at` 用不上；
- 用 `EXPLAIN ANALYZE 你的SQL` 看执行计划，出现 `Seq Scan` 就要警惕。

**N+1 查询**是后端最常见的性能坑：查 20 条评论，再为每条评论单独查一次用户，一共 21 次查询。

```go
var comments []Comment
db.Find(&comments)          // 1 次
for _, c := range comments {
    var u User
    db.First(&u, c.UserID)  // N 次
}
```

正确做法是用 `Preload` 关联查询，或者先把所有 `user_id` 收集起来一次 `WHERE id IN (...)` 查出来。

### 4.6 学习路线

按这个顺序写，每一步都能独立验证：

1. 接上 SQLite，启动能创建 `lab.db`；
2. 定义 `Note` 结构体，`AutoMigrate` 后能建出表；
3. 写 `Create` / `List` / `Get` / `Update` / `Delete` 五个 handler，用 `curl` 验证；
4. 换成 Postgres（compose 里那个），确认代码不用改、只改环境变量；
5. 给 `List` 加分页（`LIMIT` + `OFFSET`，再做 `COUNT`）；
6. 加一个需要事务的场景（比如“创建文章时同时写一条操作日志”）。

---

## 5. Redis：缓存和“降级”

Redis 在本项目里暂时没有用上，但你应该知道它解决什么问题：

| 用途 | 说明 |
|---|---|
| 缓存 | 热点数据放内存，减少数据库压力 |
| 会话 | 多实例共享登录态（单实例可以放内存） |
| 限流 | `INCR` + `EXPIRE` 做计数器 |
| 分布式锁 | 防止多实例同时执行同一任务 |
| 队列 | 简单任务队列（复杂场景用专门的消息队列） |

**关键设计原则：Redis 挂了，服务要能降级而不是挂掉。** 所以 `db.go` 的注释里写的是：连不上就记 warn 日志、把句柄置空，业务层判断 `Redis == nil` 后直接走数据库。缓存永远是可选的加速层，不能成为唯一的数据来源。

反过来，**数据库连不上就应该启动失败**，因为没有它服务没有意义，让它启动只会被负载均衡打进来然后全部报错。

---

## 6. 错误处理与 HTTP 状态码

Go 的错误是值，`if err != nil` 满屏都是，这是语言设计，不是写得不优雅。看后端代码时关注三件事：

1. **错误有没有被吞掉**。`_ = doSomething()` 意味着作者认为这里出错也无所谓，要判断这个假设成不成立。
2. **错误有没有带上上下文**。`fmt.Errorf("创建用户: %w", err)` 用 `%w` 包装，调用方还能用 `errors.Is/As` 判断原始错误类型；直接 `return err` 会让你在日志里只看到 `record not found` 而不知道为什么。
3. **错误有没有转成合适的状态码**。

状态码的选择是 API 契约的一部分，前端会依赖它：

| 场景 | 状态码 |
|---|---|
| 成功 | 200（查询/更新）、201（创建）、204（删除且无返回体） |
| 参数校验失败 | 400 |
| 未登录 / token 无效 | 401 |
| 已登录但没权限 | 403 |
| 资源不存在 | 404 |
| 业务冲突（重复创建、状态不允许） | 409 |
| 请求体超大 / 限流 | 413 / 429 |
| 服务内部错误 | 500 |
| 依赖不可用（数据库挂了） | 503 |

**不要把错误细节原样返回给前端**（比如 SQL 报错全文），会泄露表结构；详细原因写日志，响应只给一句人能看懂的话加一个错误码。

统一的响应结构能省掉前端一堆判断：

```go
type Response struct {
    Success bool   `json:"success"`
    Data    any    `json:"data,omitempty"`
    Message string `json:"message,omitempty"`
}
```

---

## 7. 上下文（context）：超时、取消、请求级数据

`context.Context` 是 Go 后端必须理解的机制，它解决两件事：

1. **把一个请求的存活范围串起来**。请求被客户端取消时，ctx 变成 `Done`，所有下游操作（数据库查询、HTTP 调用）都应该跟着停，不然会白白占用资源。
2. **携带超时和截止时间**。

本项目的用法：

```go
// 从请求里派生一个最多 2 秒的 ctx，数据库慢了也不会把整个连接池占死
ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
defer cancel()
sqlDB.PingContext(ctx)
```

两个要点：

- **`defer cancel()` 必须写**，否则定时器和 ctx 链不会被回收，是典型的内存泄漏。
- **任何外部调用都应该传 ctx**。你在 newapi 里看到的对上游大模型的请求、`relay/` 里的流式转发，全部依赖 ctx 在客户端断开时及时中止，否则会一边客户已经走了、一边还在给上游扣钱。

前端类比：ctx 类似 `AbortController` 的 signal，只是它会沿调用链自动往下传。

---

## 8. 日志

[main.go](../main.go) 里用的是标准库 `log/slog`，输出 JSON。

为什么是 JSON：容器化以后日志不是给人翻的，是被采集系统（Loki、ELK、云厂商日志服务）收走做检索的。JSON 才能按字段过滤，比如“所有 `status=500` 且 `path=/api/notes` 的日志”。

原则：

- **结构化字段优于拼字符串**：`slog.Info("创建失败", "user_id", id, "err", err)` 好过 `fmt.Sprintf("用户 %d 创建失败: %v", id, err)`；
- **级别要克制**：请求进出是 INFO，慢查询/重试是 WARN，影响用户功能的才是 ERROR。所有请求都打 ERROR 等于没有 ERROR；
- **绝不打印密码、token、身份证、手机号**。需要排查时打印 hash 或后四位；
- 日志写 **stdout**，不要自己写文件。容器里由 Docker/平台统一收集，写文件会导致日志随容器销毁一起丢。

---

## 9. 并发：goroutine 和数据竞争

这是 Go 相对 Node 最大的区别，也是你最需要建立直觉的部分。

Node 是单线程事件循环，一段同步代码永远不会被其他请求打断。Go 里每个请求一个 goroutine，**两个请求可能同时改同一个变量**。

最常见的三类问题：

**1. 全局变量并发写**

```go
var counter int
counter++ // 两个请求同时执行，结果可能少加
```

用 `sync.Mutex` 保护，或者用 `sync/atomic`。

**2. 闭包捕获循环变量**（Go 1.22 之后已修复，但老代码里常见）

**3. goroutine 泄漏**

```go
go func() {
    for { /* 没有退出条件 */ }
}()
```

所有起 goroutine 的地方都必须有退出路径，通常是监听 `ctx.Done()`。

排查工具：`go test -race`、`go run -race .`。**带 `-race` 跑一次，是上线前的必做项。**

并发也不是越多越好。本项目的健康检查用 `timeout 2s` 而不是起一个 goroutine 去查，原因是简单、可预测；复杂任务才用 worker pool（newapi 里的 `gopool` 就是干这个的）。

---

## 10. 鉴权：你需要知道的最小集合

现在项目没有登录，但这是绕不开的，提前建立模型：

**Session（服务端会话）**：登录后在服务端存一条记录，Cookie 里只放 session id。**优点**：可以随时踢下线、权限实时生效。**缺点**：多实例需要共享（Redis），否则请求打到别的实例就不认识这个 session。

**JWT（自包含令牌）**：把用户信息签名后塞进 token 里，服务端不存状态。**优点**：无状态、好扩展。**缺点**：签发后到过期前无法作废（除非再加黑名单，那就又有状态了）。

**API Key**：长期有效的密钥，适合机器调用。**必须只存 hash**，并且支持过期和吊销。

不管用哪种，**中间件读取 → 校验 → `c.Set("user", ...)` → handler 读取**这个模式是一样的。校验逻辑里最常见的漏洞是：只验了身份没验资源归属。`GET /api/notes/123` 如果只检查“已登录”，那任何登录用户都能读别人的笔记 —— 这叫**越权（IDOR）**，是业务后端最高频的安全问题。每条按 id 读写的接口都要问一句：**这个 id 属于当前用户吗？**

---

## 11. 部署相关（后端视角）

详细的动手步骤在 [README](../README.md)，这里只讲原理，这样你操作时知道自己在做什么。

### 11.1 多阶段构建

[Dockerfile](../Dockerfile) 三个阶段：

1. `webbuilder`：Node 环境装依赖、`npm run build` 产出 `web/dist`；
2. `gobuilder`：Go 环境编译，把上一步的 `dist` 拷进来，`//go:embed` 把它编进二进制；
3. 运行镜像：只保留二进制和必要证书。

为什么这么分：**最终镜像里没有源码、没有 node_modules、没有编译器**，体积从几百 MB 降到 ~20MB，攻击面也小。你在 newapi 里看到的是同一个套路，只是基镜像是 debian。

几个参数的意思：

- `CGO_ENABLED=0`：静态编译，不依赖宿主机 glibc，所以能跑在 alpine 上。如果你选的驱动依赖 CGO（比如 `github.com/mattn/go-sqlite3`），这一步会失败 —— 这也是项目里让你用 `glebarez/sqlite` 的原因，它是纯 Go 实现。
- `-trimpath`：去掉二进制里的本地路径信息，构建可复现。
- `-ldflags "-s -w"`：去掉符号表和调试信息，体积更小（代价是没法用 `dlv` 调试）。
- `-X ...Version=`：编译期给变量赋值，就是版本号的来源。

### 11.2 容器与宿主机的边界

理解这一层，90% 的“容器里连不上数据库”都能自己解决：

- 容器有自己的网络命名空间，`localhost` **指容器自己**；
- 容器之间用**服务名**互访（`postgres:5432`）；
- 从宿主机访问，靠 `ports` 映射（`5433:5432` 表示宿主机 5433 → 容器 5432）；
- 容器删了，里面的文件就没了，只有 `volumes` 里的数据能活下来。

### 11.3 健康检查

`HEALTHCHECK` 和 compose 里的 `healthcheck` 是两种不同的东西：

- 镜像里的 `HEALTHCHECK`：Docker 据此标记容器 `healthy/unhealthy`；
- compose 的 `depends_on: condition: service_healthy`：启动顺序依赖，postgres 没就绪就不启动 app。

**健康检查接口必须检查真实依赖**（数据库能 ping 通），只返回 200 的假健康检查会让编排器把流量持续打到一个实际不可用的节点上。

### 11.4 优雅退出

`main.go` 里的 `signal.NotifyContext` + `srv.Shutdown` 处理的是：`docker stop` 默认先发 SIGTERM，等 10 秒，然后 SIGKILL。这 10 秒就是留给正在处理的请求完成的时间。少了这段，每次滚动更新都会有一批请求被硬中断。

### 11.5 环境变量与密钥

compose 里的密码是练习用的明文。真实项目的做法：

- 数据库密码、第三方 API key 放密钥管理服务或 `.env`（不进 git）；
- compose 可以用 `env_file:` 引入；
- 生产环境更推荐 `docker secret` / 云厂商的密钥管理，避免密码出现在 `docker inspect` 里。

### 11.6 反向代理与 HTTPS

生产环境不会把 Go 的 8080 直接暴露到公网，前面通常有 Nginx：

```
用户 --HTTPS--> Nginx(:443) --HTTP--> 应用(:8080)
```

Nginx 负责 TLS 终止、域名路由、静态文件可选的加速、gzip、基础限流。反代时必须带这几个头：

```
proxy_set_header Host              $host;
proxy_set_header X-Real-IP         $remote_addr;
proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
proxy_set_header X-Forwarded-Proto $scheme;
```

少了 `X-Real-IP`，你日志里看到的全是 Nginx 的 IP；少了 `X-Forwarded-Proto`，应用判断不出用户是 HTTP 还是 HTTPS 进来的，生成的回调/跳转链接就会是错的。SSE 流式接口还需要关闭缓冲（`proxy_buffering off;`），否则响应会被攒起来一次性发出。

---

## 12. 建议的练习顺序

按依赖关系排，每一关都要能独立跑通再进下一关：

| 关卡 | 目标 | 验收方式 |
|---|---|---|
| 1 | 本地 `go run .` 跑起来 | `/api/health` 返回 200 |
| 2 | 前端 `npm run build` 后编进二进制 | 访问 `/` 看到前端页面 |
| 3 | 接上 SQLite，建一张表 | 文件里出现 `lab.db` 且表存在 |
| 4 | 实现 CRUD 五个接口 | `curl` 全部通过 |
| 5 | 接上 Postgres，代码零改动 | 只改 `SQL_DSN` 就切库成功 |
| 6 | 学会看容器日志和进容器排查 | 能自己解释“连不上数据库”的报错 |
| 7 | 反代 + 域名 + HTTPS | 浏览器地址栏有小锁标 |
| 8 | 打包部署到服务器 | 关掉本地终端后服务仍在跑 |
| 9 | 加 Redis 做缓存，并验证降级 | 停掉 Redis，服务仍然可用 |
| 10 | 加登录 + 越权检查 | 用户 A 拿不到用户 B 的数据 |

每关做完都问自己三个问题：**它依赖谁？谁依赖它？它挂了会怎样？** 这三个问题就是后端架构思考的起点。
