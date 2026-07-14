# Sub-Store Go Backend

[Sub-Store](https://github.com/sub-store-org/Sub-Store) 后端的 Go 重写版本，力求与原 Node.js 版本功能等效。

## 特性

### 协议解析（5 种格式，15+ 协议）

- **URI 方案**: ss://, ssr://, vmess://, vless://, trojan://, hysteria://, hysteria2://, tuic://, wireguard://, anytls://, socks5://, http://, snell://, ssh://
- **Clash YAML**: 完整 Clash/mihomo 代理配置解析
- **Surge 行格式**: 18 种代理类型
- **Loon 行格式**: shadowsocks, vmess, vless, trojan, hysteria2, anytls, wireguard 等
- **QX 行格式**: shadowsocks (SS/SSR 自动识别), vmess, vless, trojan, http, socks5, anytls

### 平台输出（14 种）

Clash, ClashMeta (mihomo), Surge, Loon, Quantumult X, sing-box, V2Ray, URI, Shadowrocket, Stash, Surfboard, Egern, SurgeMac, JSON

### 处理器（16 种）

| 类型 | 名称 |
|------|------|
| Filter | Useless Filter, Regex Filter, Type Filter, Conditional Filter, Region Filter, Script Filter |
| Operator | Quick Setting, Flag, Sort, Regex Sort, Regex Rename, Regex Delete, Handle Duplicate, Resolve Domain, Script Operator |

### 核心功能

- **订阅处理**: URL 获取 → 解析 → 处理器流水线 → 平台输出
- **组合订阅**: 多个订阅聚合，支持 subscriptionTags 自动匹配
- **合并来源**: localFirst / remoteFirst 模式合并本地与远程内容
- **流量信息透传**: subscription-userinfo / profile-web-page-url / plan-name 响应头
- **UA 透传**: passThroughUA 将客户端 User-Agent 转发到远程订阅
- **文件处理**: URL 获取、mergeSources、缓存、代理处理
- **Share Token 认证**: 时间过期 + 次数限制，自动消费计数
- **存储导入/导出**: 与原项目兼容的 JSON 格式
- **Nezha 监控 API**: server/details 和 monitor 端点
- **定时任务**: Sync / Produce / Download / MMDB 更新 cron

## 架构

```
go-backend/
├── cmd/sub-store/        # 入口
├── internal/
│   ├── app/              # 应用核心 (日志、推送、同步)
│   ├── cache/            # 内存 TTL 缓存
│   ├── config/           # 环境变量配置
│   ├── flowutil/         # 流量信息获取与归一化
│   ├── handler/          # HTTP 路由与处理器
│   ├── middleware/        # CORS、日志、Token 认证
│   ├── model/            # 数据模型 (Subscription, Collection, Proxy 等)
│   ├── normalizer/       # 代理字段归一化
│   ├── parser/           # 多格式解析器 (URI, Clash, Surge, Loon, QX)
│   ├── processor/        # 过滤器与操作器
│   ├── producer/         # 多平台输出器
│   ├── ruleutil/         # 规则解析与输出
│   └── store/            # 文件 JSON 持久化
└── Dockerfile
```

## 快速开始

### 本地构建

```bash
cd go-backend
go build -o sub-store ./cmd/sub-store/
./sub-store
```

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `SUB_STORE_BACKEND_API_HOST` | 监听地址 | `::` |
| `SUB_STORE_BACKEND_API_PORT` | 监听端口 | `3000` |
| `SUB_STORE_DATA_BASE_PATH` | 数据目录 | `.` |
| `SUB_STORE_BACKEND_MERGE` | 在根路径提供 UI | `false` |
| `SUB_STORE_BACKEND_PATH` | 私有后端路径；非空时隐藏无前缀 API | 空 |
| `SUB_STORE_BACKEND_SYNC_CRON` | 同步 cron 表达式 | 空 |
| `SUB_STORE_BACKEND_PRODUCE_CRON` | 生成 cron 表达式 | 空 |
| `SUB_STORE_BACKEND_DOWNLOAD_CRON` | 预取 cron 表达式 | 空 |
| `SUB_STORE_CORS_ALLOWED_ORIGINS` | CORS 允许的来源 | `*` |
| `SUB_STORE_BACKEND_DEFAULT_PROXY` | 默认代理 | 空 |
| `SUB_STORE_BACKEND_DEFAULT_USER_AGENT` | 默认 UA | 空 |

### Docker

```bash
docker build -t sub-store-go ./go-backend
docker run -d -p 3000:3000 -v sub-store-data:/app/data -e SUB_STORE_BACKEND_MERGE=true -e SUB_STORE_BACKEND_PATH=your-private-path sub-store-go
```

### GHCR 镜像

```bash
docker pull ghcr.io/woyin/sub-store:latest
docker run -d -p 3000:3000 -v sub-store-data:/app/data -e SUB_STORE_BACKEND_MERGE=true -e SUB_STORE_BACKEND_PATH=your-private-path ghcr.io/woyin/sub-store:latest
```

### Fly.io 私有后端路径

不要把后端路径写入 `fly.toml`。使用 Fly Secret：

```bash
flyctl secrets set SUB_STORE_BACKEND_PATH=your-private-path --app your-app
flyctl deploy --config fly.toml
```

UI 中输入相同路径。此路径用于隐藏后端入口，不替代强认证。

## API 端点

### 订阅

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/subs` | 获取所有订阅 |
| POST | `/api/subs` | 创建订阅 |
| GET | `/api/sub/:name` | 获取单个订阅 |
| PATCH | `/api/sub/:name` | 更新订阅 |
| DELETE | `/api/sub/:name` | 删除订阅 |
| GET | `/api/sub/flow/:name` | 获取流量信息 |
| GET | `/download/:name` | 下载订阅 (自动检测平台) |
| GET | `/download/:name/:target` | 下载订阅 (指定平台) |

### 组合订阅

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/collections` | 获取所有组合 |
| POST | `/api/collections` | 创建组合 |
| GET | `/api/collection/:name` | 获取单个组合 |
| PATCH | `/api/collection/:name` | 更新组合 |
| DELETE | `/api/collection/:name` | 删除组合 |
| GET | `/download/collection/:name` | 下载组合 |

### 文件

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/files` | 获取所有文件 |
| POST | `/api/files` | 创建文件 |
| GET | `/api/file/:name` | 下载文件 (支持 URL 获取 + mergeSources) |
| PATCH | `/api/file/:name` | 更新文件 |
| DELETE | `/api/file/:name` | 删除文件 |

### Share 路由 (需 Token)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/share/sub/:name` | 分享订阅 |
| GET | `/share/col/:name` | 分享组合 |
| GET | `/share/file/:name` | 分享文件 |

### 其他

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/settings` | 获取设置 |
| PATCH | `/api/settings` | 更新设置 |
| GET | `/api/storage` | 导出存储 |
| POST | `/api/storage` | 导入存储 |
| GET | `/api/utils/env` | 环境信息 |
| GET | `/api/utils/backup` | Gist 备份 |
| GET | `/download/:name/api/v1/server/details` | Nezha 服务器详情 |

## 许可证

[AGPL-3.0](../LICENSE) — 与原项目 [Sub-Store](https://github.com/sub-store-org/Sub-Store) 保持一致。

## 致谢

- [Sub-Store](https://github.com/sub-store-org/Sub-Store) — 原项目
- [Gin](https://github.com/gin-gonic/gin) — HTTP 框架
- [goja](https://github.com/dop251/goja) — JavaScript 运行时 (Script Filter/Operator)
- [robfig/cron](https://github.com/robfig/cron) — 定时任务
