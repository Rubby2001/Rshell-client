# Rshell-client - Multi-Protocol C2 Client in Golang

English | **[简体中文](./README_zh-CN.md)**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
![Language](https://img.shields.io/badge/Language-Go-blue)
![GitHub Stars](https://img.shields.io/github/stars/Rubby2001/Rshell-client?style=social)

> ⚠️ **Disclaimer**: This project is intended solely for security research, **authorized** penetration testing, red/blue teaming and education. Do not use it for any unauthorized purpose. See the full disclaimer in the [main repository](https://github.com/Rubby2001/Rshell---A-Cross-Platform-C2#disclaimer).

Rshell-client is the **Golang client** of the [Rshell](https://github.com/Rubby2001/Rshell---A-Cross-Platform-C2) C2 framework. It supports multiple protocols; build outputs serve as templates embedded into the server, which patches the placeholders when generating a client.

## Rshell Project Matrix

| Project | Description |
|---|---|
| [Rshell---A-Cross-Platform-C2](https://github.com/Rubby2001/Rshell---A-Cross-Platform-C2) | C2 server (Go) |
| **Rshell-client** | Golang client (this repo) |
| [Rshell-client-rust](https://github.com/Rubby2001/Rshell-client-rust) | Rust client |
| [Rshell-web](https://github.com/Rubby2001/Rshell-web) | Web frontend |

## Core Features

- **Multi-protocol callbacks**: TCP, WebSocket, KCP, HTTP and OSS bucket polling — all over the server's double-encrypted channel
- **Proxied egress**: `Forward/` tunnels TCP / WebSocket connections through a SOCKS5 proxy for restricted networks
- **In-memory execution**: built-in BOF (COFF loader) support
- **Information gathering**: browser data extraction (hackbrowserdata), system and process information collection
- **Interactive terminal**: remote interactive shell
- **Anti-sandbox**: optional execution password — the binary silently exits without the correct argument
- **Templated configuration**: server address, public key and execution password are placeholders replaced by the server at generation time

## Repository Layout

Each protocol is a standalone Go module, built separately:

```
├── Reacon_tcp/           # TCP transport
├── Reacon_websocket/     # WebSocket transport
├── Reacon_kcp/           # KCP transport
├── Reacon_http/          # HTTP(S) transport
├── Reacon_oss/           # Alibaba Cloud OSS transport
├── Forward/
│   ├── tcp/              # TCP through a proxy
│   └── websocket/        # WebSocket through a proxy
└── shared/               # shared library: crypto, command execution, terminal, BOF, info gathering
```

## Building

Requires Go (the `build.sh` scripts default to the `go1.20` toolchain; adjust as needed). All builds are `CGO_ENABLED=0` cross-compilations:

```bash
cd Reacon_tcp
bash build.sh
```

Artifacts are output into each protocol's `server/` directory, covering common platforms:

- `r_windows_amd64.exe` / `r_windows_386.exe`
- `r_linux_amd64 / 386 / arm / arm64 / loong64 / mips / mipsle / mips64 / mips64le`
- `r_darwin_amd64 / r_darwin_arm64`

### Using outputs as server templates

Copy the build outputs into the Rshell server's `pkg/api/server/<protocol>/` directory (keep the `r_<GOOS>_<ARCH>[.exe]` naming), then rebuild the server to embed them. Placeholders are replaced automatically when the server generates a client:

| Placeholder | Location | Meaning |
|---|---|---|
| `HOSTAAA...` | each protocol's `main.go` | Server address |
| `ServerPublicKeyAAA...` | `shared/config/config.go` | Server public key |
| `PASSAAA...` | `shared/config/config.go` | Anti-sandbox execution password |

## License

[MIT](./LICENSE) © Rubby2001
