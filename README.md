# Conecter

局域网文件直传工具。文件保存在分享者本机，不经过云盘。

## 特性

- 无需注册、无需云盘，在同一局域网内直接分享
- 6 位交换码和分享链接双入口
- 支持单文件下载和多文件 ZIP 下载
- 分享默认 24 小时后失效，过期文件自动清理
- Go 服务流式处理文件，前端保持轻量原生 JavaScript
- 响应式界面，支持桌面和移动端访问

## Windows 快速启动

双击 `start-conecter.bat`。脚本会启动 Go 服务和前端页面，并显示局域网访问地址。

- 本机访问：`http://localhost:5174/`
- 局域网访问：使用启动窗口显示的 LAN 地址
- API 服务：`http://localhost:5173/`

首次启动会自动创建 `uploads` 目录和 `shares.json`。这两个运行时文件不会提交到仓库。

## 从源码运行

环境要求：Go 1.22+、Node.js 20+。

```powershell
npm install
npm run build
npm run build:go
.\start-conecter.bat
```

开发模式可以只启动 Vite：

```powershell
npm run dev
```

Go 服务和前端分别使用 5173、5174 端口。

## 使用流程

1. 发送端拖入一个或多个文件。
2. 将生成的链接或 6 位交换码发给接收端。
3. 接收端输入交换码后，可逐个下载文件；多文件分享可下载 ZIP。
4. 分享者可以在发送端撤销分享。

## API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/info` | 获取局域网 IP |
| `POST` | `/api/shares` | 上传文件并创建分享 |
| `GET` | `/api/shares/{id}` | 获取分享文件列表 |
| `GET` | `/api/shares/{id}/files/{index}` | 下载单个文件 |
| `GET` | `/api/shares/{id}/zip` | 下载全部文件为 ZIP |
| `GET` | `/api/shares/{id}/qr` | 获取分享二维码 |
| `DELETE` | `/api/shares/{id}` | 撤销分享 |

## 限制与安全

- 单次最多 50 个文件。
- 单次请求总大小最多 5 GB。
- 交换码查询有基础限流。
- 仅建议在可信的局域网中使用；当前服务默认使用 HTTP，不适合直接暴露到公网。

## 停止服务

关闭启动窗口不会停止服务。需要在任务管理器中结束 `conecter.exe` 和 Vite 进程，或重启电脑。

## 开发检查

```powershell
go test ./...
go test -race ./...
go vet ./...
npm run build
npm audit --omit=dev
```
