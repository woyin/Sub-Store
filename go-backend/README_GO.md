# Sub-Store Go Backend

Go re-implementation of Sub-Store backend with 100% functional equivalence.

## Features

- RESTful API compatible with original Sub-Store
- Proxy parsing: SS, SSR, VMess, VLESS, Trojan, Hysteria, Hysteria2, TUIC, AnyTLS, WireGuard, SOCKS5, HTTP
- Platform output: Clash, ClashMeta, Surge, Loon, QX, Stash, Shadowrocket, Surfboard, sing-box, V2Ray
- File-based data persistence
- Gist sync/backup
- Cron jobs for artifact sync
- CORS support
- IPv6 support
- Docker support
- GitHub Actions CI/CD

## Build

```bash
cd go-backend
go build -o sub-store .
```

## Run

```bash
./sub-store
```

Environment variables:
- `SUB_STORE_BACKEND_API_PORT` - Server port (default: 3000)
- `SUB_STORE_BACKEND_API_HOST` - Server host (default: ::)
- `SUB_STORE_DATA_BASE_PATH` - Data directory (default: ./data)
- `SUB_STORE_BACKEND_SYNC_CRON` - Artifact sync cron expression

## Docker

```bash
docker build -t sub-store-go ./go-backend
docker run -p 3000:3000 -v ./data:/app/data sub-store-go
```

## License

MIT
