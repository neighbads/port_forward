# Port Forward Tool — Design Document

## Overview

Go 语言端口转发工具，支持 TCP/UDP 协议。Windows 提供系统托盘 + Fyne GUI，Linux 仅命令行。Windows 加 `-noui` 参数时也走命令行模式。

## Project Structure

```
port_forward/
├── main.go                 # 入口，判断 GUI/CLI 模式
├── go.mod
├── core/
│   ├── forwarder/
│   │   ├── tcp.go          # TCP 转发
│   │   ├── udp.go          # UDP 转发
│   │   └── manager.go      # 转发规则管理器
│   ├── config/
│   │   └── config.go       # YAML 配置读写
│   └── logger/
│       └── logger.go       # 分级日志，按规则独立缓冲
├── cli/
│   └── cli.go              # 命令行参数解析与执行
├── gui/
│   ├── tray.go             # 系统托盘（构建标签 windows）
│   ├── window.go           # 主窗口列表
│   ├── logview.go          # 日志查看窗口
│   └── icons/              # 红/绿图标资源
└── config.yaml             # 默认配置示例
```

## Core Forwarding Engine (core/forwarder/)

### TCP 转发 (tcp.go)

- `net.Listen` 监听本地端口
- 接受连接后 `net.Dial` 连接远端（受 `connect_timeout` 控制）
- 双向 `io.Copy` 管道转发数据
- 空闲超时（`idle_timeout`）：无数据传输超过阈值则双向关闭
- 每个连接独立 goroutine，通过 context 控制生命周期

### UDP 转发 (udp.go)

- `net.ListenPacket` 监听本地端口
- 收到数据包后维护 client→remote session map
- 双向转发，每个 session 有独立超时（`session_timeout`）
- 超时后清理 session entry，释放资源

### Manager (manager.go)

- 管理所有转发规则的生命周期：启用/禁用/添加/删除
- 每条规则有独立的 goroutine 组和 context 取消控制
- 线程安全，支持运行时动态操作

## Timeout Configuration

### 全局默认 + 规则级覆盖

```yaml
defaults:
  tcp_connect_timeout: 10s
  tcp_idle_timeout: 300s
  udp_session_timeout: 60s

rules:
  - protocol: tcp
    local: "127.0.0.1:1234"
    remote: "0.0.0.0:5678"
    enabled: true
    tcp_connect_timeout: 5s
    tcp_idle_timeout: 0s       # 0 = 不超时
  - protocol: udp
    local: "127.0.0.1:9000"
    remote: "192.168.1.100:9000"
    enabled: false
    udp_session_timeout: 120s
log_level: info
```

### 超时异常处理

| 场景 | 处理 |
|------|------|
| TCP 连接远端超时 | Warn 日志，关闭客户端连接 |
| TCP 空闲超时 | Info 日志，双向关闭连接 |
| UDP 会话超时 | Info 日志，清理 session entry |
| 远端不可达 | Error 日志，不影响其他连接/规则 |

## Logger (core/logger/)

- 4 个级别：Debug, Info, Warn, Error
- 每个转发规则一个环形缓冲区（最近 1000 条）
- 全局日志级别，运行时可通过托盘菜单或 CLI 调整
- GUI 日志窗口从缓冲区实时读取
- 全局日志汇总所有规则的日志

## GUI (Windows Only, Fyne)

### 系统托盘

- **启动**：无窗口，只显示托盘图标（红色 = 停止状态）
- **左键单击**：启动所有 enabled 规则，图标变绿；再点击停止所有，变红
- **双击**：打开主窗口
- **右键菜单**：
  - 查看日志（全局日志窗口）
  - 日志级别 → Debug / Info / Warn / Error
  - 退出

### 主窗口

- 列表显示所有转发规则
- 列字段：序号、协议(TCP/UDP)、本地地址(IP:PORT)、远程地址(IP:PORT)、操作按钮
- 操作按钮：启用/禁用（文字自动切换）、删除、日志
- 最后一行：灰色待添加行，输入内容后添加为新规则
- 日志按钮：弹出该规则的日志窗口

### 图标

- 红色风格图标：服务停止状态
- 绿色风格图标：服务运行状态

## CLI

跨平台通用，Windows `-noui` 时也使用此模式。

```
port_forward -noui                    # Windows 无 UI 模式
port_forward add -p tcp -l 127.0.0.1:1234 -r 0.0.0.0:5678 --connect-timeout 5s --idle-timeout 0s
port_forward add -p udp -l 127.0.0.1:9000 -r 192.168.1.100:9000 --session-timeout 120s
port_forward remove -id 1
port_forward list
port_forward start                    # 启动所有 enabled
port_forward stop
port_forward log -id 1
port_forward set-loglevel debug
```

## Build

- `gui/` 下文件使用 `//go:build windows` 构建标签
- Linux 编译时自动排除 GUI 代码
- Windows 编译时 `-ldflags "-H windowsgui"` 隐藏控制台窗口

## Dependencies

- `fyne.io/fyne/v2` — Windows GUI 框架
- `gopkg.in/yaml.v3` — YAML 配置
- Go 标准库 — net, io, context, sync, flag
