# Puff

![GitHub repo size](https://img.shields.io/github/repo-size/spatiostu/puff)
![GitHub Repo stars](https://img.shields.io/github/stars/spatiostu/puff)
![GitHub all releases](https://img.shields.io/github/downloads/spatiostu/puff/total)

开源、快速、便捷、基于Go的域名监控程序。

原理：通过 Whois 通过字段进行判断域名状态。

**现版本是 V1 版本的 Beta，如碰到问题，请及时反馈，谢谢！**

**当前版本由于完全重构，故无法直接迁移。**

![image.png](https://s2.loli.net/2025/12/17/CzWOGqwaMxbiBhr.png)

## 功能特性

### 核心功能
- **多协议支持**: 同时支持WHOIS和RDAP协议查询
- **实时监控**: 定时检查域名状态（可注册、赎回期、待删除等）
- **状态追踪**: 自动检测域名状态变化并记录
- **高性能**: 多线程并发处理，支持几千个域名流畅查询

### 通知系统
- **邮箱通知**: 支持SMTP邮件推送，扁平化简洁的HTML邮件模板
- **Telegram通知**: 支持Telegram Bot消息推送，简洁的文本格式
- **智能通知聚合**: 10秒窗口内的多个状态变化合并发送
- **自适应发送**: 8秒内无新查询时立即发送通知，无需等待
- **状态变化通知**: 仅在域名状态变化时发送通知
- **首次查询保护**: 首次查询的域名不发送通知，避免噪音
- **通知历史记录**: 数据库记录所有通知历史，避免重复通知
- **重启后智能判断**: 程序重启后，已通知过的状态不会重复通知

### 用户界面
- **简洁WebUI**: 响应式Web界面，支持移动端
- **实时状态**: 域名状态实时展示
- **历史记录**: 状态变化历史追踪

### 数据存储
- **SQLite持久化**: 所有配置、域名列表、通知开关、查询结果均写入 `data/puff.db`
- **内置默认值**: 首次启动自动写入默认账号、端口、通知配置
- **可视化配置**: 通过 Web 界面保存 SMTP/Telegram/账号/监控参数等设置
- **无缓存设计**: 所有数据直接从SQLite读写，保证数据一致性

# 部署 Puff

目前支持三种部署方式，编译部署、手动部署、Docker 部署。

## 编译部署

### 环境要求

- Go 版本 >=1.23.4

### 克隆仓库

``` shell
git clone https://github.com/spatiostu/puff.git
```

### 构建程序

``` shell
go build -o Puff.exe main.go
```

### 运行

```
./Puff.exe
```



## 手动部署

### 下载 Puff

打开 [Puff Release](https://github.com/spatiostu/Puff/releases) 下载对应的平台以及系统的文件。

如果最新的包没有您对应的二进制文件，可以提交 [issues](https://github.com/spatiostu/Puff/issues) ，或可以选择自己编译安装。

其中：

armv6 对应 arm 架构32位版本，arm64 对应 arm 架构64位版本。

x86 对应 x86 平台32位版本，x86_64 对应  x86 平台64位版本。

克隆模板文件以及静态资源文件。

### 手动运行

#### Linux / MacOS

``` shell
# 解压下载后的文件，请求改为您下载的文件名
tar -zxvf filename.tar.gz

# 授予执行权限
chmod +x Puff

./Puff
```

#### Windows

双击运行即可。

### 持久化运行

#### Linux

使用编辑器编辑 ``` /usr/lib/systemd/system/puff.service``` 添加如下内容：

``` shell
[Unit]
Description=puff
After=network.target
 
[Service]
Type=simple
WorkingDirectory=puff_path
ExecStart=puff_path/Puff
Restart=on-failure
 
[Install]
WantedBy=multi-user.target
```

保存后，使用 ```systemctl deamon-reload``` 重载配置。具体使用命令如下：

- 启动: `systemctl start puff`
- 关闭: `systemctl stop puff`
- 配置开机自启: `systemctl enable puff`
- 取消开机自启: `systemctl disable puff`
- 状态: `systemctl status puff`
- 重启: `systemctl restart puff`

### 更新版本

如果有新版本更新，下载新版本，将旧版本的文件删除重新运行即可。

## Docker 部署

首先请确保您正确的安装并配置了 Docker 以及 Docker Compose

### Docker CLI

``` shell
docker run -d --restart=unless-stopped -v ./data/puff:/app/data -p 8080:8080 --name="Puff" spatiostu/puff:latest
```

### Docker Compose

在空目录中创建 docker-compose.yaml 文件，将下列内容保存。

``` dockerfile
services:
  Puff:
    image: spatiostu/puff:latest
    container_name: Puff
    volumes:
      - ./data/puff:/app/data
    restart: unless-stopped
    ports:
      - 8080:8080
```

保存后，使用 ``` docker compose up -d``` 创建并启动容器。

### Docker 容器更新

#### CLI

```shell
#查看容器ID
docker ps -a

#停止容器
docker stop ID

#删除容器
docker rm ID

#获取新镜像
docker pull spatiostu/puff:latest

# 输入安装命令
docker run -d --restart=unless-stopped -v ./data/puff:/app/data -p 8080:8080 --name="Puff" spatiostu/puff:latest
```

#### Docker Compose

``` shell
#获取新镜像
docker pull spatiostu/puff:latest

#创建并启动容器
docker compose up -d
```

## 访问

此时打开 `IP:8080` 即可打开站点。默认账号 `puff`密码均为 `puff123`。

## 星标趋势

[![Star History Chart](https://api.star-history.com/svg?repos=spatiostu/Puff&type=date&legend=top-left)](https://www.star-history.com/#spatiostu/Puff&type=date&legend=top-left)

MIT License
