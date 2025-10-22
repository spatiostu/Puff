# 域名监控程序 (Domain Monitor)

一个使用Go语言开发的高性能域名监控系统，支持通过WHOIS和RDAP协议实时监控域名状态变化。

## 🚀 功能特性

### 核心功能
- **多协议支持**: 同时支持WHOIS和RDAP协议查询
- **实时监控**: 定时检查域名状态（可注册、赎回期、待删除等）
- **状态追踪**: 自动检测域名状态变化并记录
- **高性能**: 多线程并发处理，支持几千个域名流畅查询

### 通知系统
- **邮箱通知**: 支持SMTP邮件推送
- **Telegram通知**: 支持Telegram Bot消息推送
- **状态变化通知**: 域名状态改变时自动通知

### 用户界面
- **简洁WebUI**: 响应式Web界面，支持移动端
- **实时状态**: 域名状态实时展示
- **历史记录**: 状态变化历史追踪

### 数据存储
- **无数据库设计**: 使用文件存储，降低部署复杂度
- **YAML配置**: 域名列表使用YAML格式存储
- **环境变量**: 敏感配置存储在.env文件

### 安全认证
- **单用户登录**: 简单的密码认证机制
- **会话管理**: 安全的用户会话控制

## 📁 项目结构

```
domain-monitor/
├── README.md           # 项目说明文档
├── main.go            # 主程序入口
├── go.mod             # Go模块依赖
├── go.sum             # 依赖版本锁定
├── .env.example       # 环境变量模板
├── domains.yml        # 域名列表配置
├── config/            # 配置管理
│   ├── config.go      # 配置结构和加载
│   └── servers.go     # 内置WHOIS/RDAP服务器地址
├── core/              # 核心功能模块
│   ├── whois.go       # WHOIS查询实现
│   ├── rdap.go        # RDAP查询实现
│   ├── domain.go      # 域名状态解析
│   └── monitor.go     # 监控调度器
├── notification/      # 通知系统
│   ├── email.go       # 邮件通知
│   ├── telegram.go    # Telegram通知
│   └── notifier.go    # 通知接口定义
├── auth/              # 认证模块
│   ├── auth.go        # 用户认证
│   └── session.go     # 会话管理
├── web/               # Web界面
│   ├── server.go      # HTTP服务器
│   ├── handlers.go    # 请求处理器
│   └── static/        # 静态资源
│       ├── index.html # 主页面
│       ├── style.css  # 样式文件
│       └── script.js  # JavaScript脚本
└── storage/           # 数据存储
    ├── file.go        # 文件操作
    └── cache.go       # 内存缓存
```

## ⚙️ 配置说明

### 环境变量 (.env)
```env
# 服务器配置
PORT=8080
PASSWORD=your_secure_password

# 邮件通知配置
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your_email@gmail.com
SMTP_PASS=your_app_password
SMTP_FROM=your_email@gmail.com
SMTP_TO=notify@example.com

# Telegram通知配置
TELEGRAM_BOT_TOKEN=your_bot_token
TELEGRAM_CHAT_ID=your_chat_id

# 监控配置
CHECK_INTERVAL=300  # 检查间隔(秒)
CONCURRENT_LIMIT=50 # 并发查询限制
```

### 域名列表 (domains.yml)
```yaml
domains:
  - name: "example.com"
    enabled: true
    notify: true
  - name: "test.net"
    enabled: true
    notify: false
  - name: "sample.org"
    enabled: false
    notify: true
```

## 🔧 安装和使用

### 安装依赖
```bash
go mod init domain-monitor
go mod tidy
```

### 配置环境
1. 复制环境变量模板：
```bash
cp .env.example .env
```

2. 编辑`.env`文件，填入您的配置信息

3. 编辑`domains.yml`文件，添加要监控的域名

### 启动程序
```bash
go run main.go
```

### 访问界面
打开浏览器访问: `http://localhost:8080`

## 📊 域名状态说明

| 状态 | 描述 | 通知触发 |
|------|------|----------|
| 可注册 | 域名未被注册，可以立即注册 | ✅ |
| 已注册 | 域名已被他人注册 | ❌ |
| 赎回期 | 域名在赎回期内，可以赎回 | ✅ |
| 待删除 | 域名即将删除，进入抢注阶段 | ✅ |
| 查询失败 | 无法获取域名状态信息 | ❌ |

## 🔍 技术实现

### WHOIS查询
- 内置常用TLD的WHOIS服务器地址
- 支持自定义WHOIS服务器
- 智能解析WHOIS响应内容

### RDAP查询
- 支持RDAP协议查询
- 结构化数据解析
- 更准确的状态判断

### 性能优化
- 使用goroutine池进行并发查询
- 智能重试机制
- **多层缓存系统**: 
  - 域名查询结果缓存
  - 统计信息缓存
  - LRU淘汰策略
- 限流控制避免被封IP
- **智能分页**: 优化大量域名的显示性能
- **缓存管理**: 自动清理过期数据，支持手动清除

## 🚨 注意事项

1. **查询频率**: 建议设置合理的检查间隔，避免过于频繁的查询
2. **并发限制**: 根据服务器性能调整并发查询数量
3. **服务器选择**: 某些WHOIS服务器可能有查询限制
4. **通知配置**: 确保邮件和Telegram配置正确

## 📝 更新日志

### v1.0.1 (性能优化版本)
- 🚀 **性能优化**
  - 添加智能缓存系统，减少重复查询
  - 实现LRU缓存淘汰策略
  - 优化统计信息获取，避免频繁的实时查询
  - 添加缓存命中率统计

- 🐛 **Bug修复**
  - 修复总域名数显示错误（只显示当前页域名数的问题）
  - 修复统计数据不准确的问题
  - 改进前端数据加载逻辑

- 💡 **功能增强**
  - 添加缓存统计信息展示
  - 优化分页性能
  - 改进域名查询缓存策略
  - 增强错误处理和日志记录

### v1.0.0 (初始版本)
- 基础域名监控功能
- WHOIS和RDAP查询支持
- 邮件和Telegram通知
- 简洁的Web界面
- 多线程并发处理

## 🤝 贡献指南

欢迎提交Issue和Pull Request来改进这个项目！

## 📄 许可证

MIT License