# Rshell-client - Golang 多协议 C2 客户端

**[English](./README.md)** | 简体中文

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
![Language](https://img.shields.io/badge/Language-Go-blue)
![GitHub Stars](https://img.shields.io/github/stars/Rubby2001/Rshell-client?style=social)

> ⚠️ **免责声明**：本项目仅供安全研究人员在**授权范围内**进行渗透测试、红蓝对抗与安全教育使用，禁止用于任何未授权用途。完整声明见 [Rshell 主仓库](https://github.com/Rubby2001/Rshell---A-Cross-Platform-C2#免责声明)。

Rshell-client 是 [Rshell](https://github.com/Rubby2001/Rshell---A-Cross-Platform-C2) C2 框架的 **Golang 客户端**，支持多种协议上线，编译产物作为模板内嵌进服务端，由服务端生成客户端时替换占位符完成配置。

## Rshell 项目矩阵

| 项目 | 说明 |
|---|---|
| [Rshell---A-Cross-Platform-C2](https://github.com/Rubby2001/Rshell---A-Cross-Platform-C2) | C2 服务端（Go） |
| **Rshell-client** | Golang 客户端（本仓库） |
| [Rshell-client-rust](https://github.com/Rubby2001/Rshell-client-rust) | Rust 客户端 |
| [Rshell-web](https://github.com/Rubby2001/Rshell-web) | Web 前端 |

## 核心特性

- **多协议上线**：TCP、WebSocket、KCP、HTTP、OSS 存储桶轮询，均与服务端双加密通道通讯
- **代理转发上线**：`Forward` 支持 TCP / WebSocket 经代理（SOCKS5）转发上线，适配内网出网受限场景
- **内存执行**：内置 BOF（COFF Loader）加载执行能力
- **信息收集**：集成浏览器数据获取（hackbrowserdata）、系统信息、进程信息收集
- **交互式终端**：远程交互式 Shell
- **反沙箱**：支持配置执行密码，未携带正确参数时静默退出
- **模板化配置**：上线地址、公钥、执行密码均为占位符，由服务端生成客户端时替换

## 目录结构

每个协议为独立 Go module，单独编译：

```
├── Reacon_tcp/           # TCP 上线
├── Reacon_websocket/     # WebSocket 上线
├── Reacon_kcp/           # KCP 上线
├── Reacon_http/          # HTTP(S) 上线
├── Reacon_oss/           # 阿里云 OSS 存储桶上线
├── Forward/
│   ├── tcp/              # TCP 经代理转发上线
│   └── websocket/        # WebSocket 经代理转发上线
└── shared/               # 共享库：加密、命令执行、终端、BOF、信息收集等
```

## 编译

依赖 Go（`build.sh` 中默认使用 `go1.20` 工具链，可按需修改），全部 `CGO_ENABLED=0` 交叉编译：

```bash
cd Reacon_tcp
bash build.sh
```

产物输出到各协议目录的 `server/` 下，覆盖常见平台：

- `r_windows_amd64.exe` / `r_windows_386.exe`
- `r_linux_amd64 / 386 / arm / arm64 / loong64 / mips / mipsle / mips64 / mips64le`
- `r_darwin_amd64 / r_darwin_arm64`

### 作为服务端模板使用

将编译产物复制到 Rshell 服务端 `pkg/api/server/<协议>/` 目录（文件名保持 `r_<GOOS>_<ARCH>[.exe]`），重新编译服务端即可内嵌生效。服务端生成客户端时自动替换模板中的占位符：

| 占位符 | 位置 | 含义 |
|---|---|---|
| `HOSTAAA...` | 各协议 `main.go` | 服务端地址 |
| `ServerPublicKeyAAA...` | `shared/config/config.go` | 服务端公钥 |
| `PASSAAA...` | `shared/config/config.go` | 反沙箱执行密码 |

## License

[MIT](./LICENSE) © Rubby2001
