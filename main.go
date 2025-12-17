package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"Puff/auth"
	"Puff/config"
	"Puff/core"
	"Puff/logger"
	"Puff/notification"
	"Puff/storage"
	"Puff/web"
)

// 导出版本号给其他包使用
func GetAppVersion() string {
	return AppVersion
}

var (
	AppName    = "Puff"
	AppVersion = "v0.9.2"
)

func main() {
	fmt.Printf("%s %s\n", AppName, AppVersion)
	fmt.Println("正在启动...")

	// 设置web包的版本号
	web.SetAppVersion(AppVersion)

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("加载配置失败: %v", err)
	}

	// 初始化日志系统
	if err := logger.Init(cfg.Log.Level, cfg.Log.File); err != nil {
		logger.Warn("初始化日志文件失败: %v，将只输出到标准输出", err)
	}
	defer logger.Close()

	logger.Info("日志系统已初始化，级别: %s", cfg.Log.Level)

	// 清理孤立数据（启动时自动清理）
	logger.Info("正在检查并清理孤立数据...")
	if err := storage.CleanOrphanedData(); err != nil {
		logger.Warn("清理孤立数据失败: %v", err)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		logger.Fatal("配置验证失败: %v", err)
	}

	// 创建认证器
	authenticator := auth.NewAuthenticator(cfg.Server.Username, cfg.Server.Password)

	// 创建通知管理器
	notificationMgr := notification.NewNotificationManager()

	// 始终创建邮件通知器（启用状态由配置控制）
	emailNotifier := notification.NewEmailNotifier(cfg.SMTP)
	notificationMgr.AddNotifier(emailNotifier)
	if cfg.SMTP.Enabled {
		logger.Info("邮件通知器已启用")
	} else {
		logger.Info("邮件通知器已创建但未启用")
	}

	// 始终创建Telegram通知器（启用状态由配置控制）
	telegramNotifier := notification.NewTelegramNotifier(cfg.Telegram)
	notificationMgr.AddNotifier(telegramNotifier)
	if cfg.Telegram.Enabled {
		logger.Info("Telegram通知器已启用")
	} else {
		logger.Info("Telegram通知器已创建但未启用")
	}

	// 启动通知管理器
	notificationMgr.Start()

	// 创建域名监控器（传入查询记录函数）
	monitor := core.NewMonitor(cfg, notificationMgr.RecordDomainQuery)

	// 启动通知处理协程
	go handleNotifications(monitor, notificationMgr)

	// 创建Web服务器
	webServer := web.NewServer(cfg, monitor, authenticator, notificationMgr)

	// 启动监控器
	if err := monitor.Start(); err != nil {
		logger.Warn("启动监控器失败: %v", err)
	} else {
		logger.Info("域名监控器已启动")
	}

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动Web服务器
	go func() {
		logger.Info("Web服务器启动在端口 %s", cfg.Server.Port)
		logger.Info("访问地址: http://localhost:%s", cfg.Server.Port)

		if err := webServer.Start(); err != nil {
			logger.Error("Web服务器启动失败: %v", err)
			cancel()
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info("收到信号: %v", sig)
	case <-ctx.Done():
		logger.Info("应用程序上下文已取消")
	}

	// 优雅关闭
	logger.Info("正在关闭应用程序...")

	// 创建关闭超时上下文
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// 关闭各个组件
	go func() {
		// 停止监控器
		monitor.Stop()
		logger.Info("域名监控器已停止")

		// 停止通知管理器
		notificationMgr.Stop()
		logger.Info("通知管理器已停止")

		// 停止Web服务器
		if err := webServer.Stop(); err != nil {
			logger.Warn("停止Web服务器时出错: %v", err)
		} else {
			logger.Info("Web服务器已停止")
		}

		// 清理认证器
		authenticator.CleanupExpiredSessions()
		logger.Info("认证器已清理")

		shutdownCancel()
	}()

	// 等待关闭完成或超时
	<-shutdownCtx.Done()

	if shutdownCtx.Err() == context.DeadlineExceeded {
		logger.Warn("关闭超时，强制退出")
	} else {
		logger.Info("应用程序已优雅关闭")
	}
}

// handleNotifications 处理通知事件
func handleNotifications(monitor *core.Monitor, notificationMgr *notification.NotificationManager) {
	for event := range monitor.GetNotifications() {
		logger.Info("域名状态变化通知: %s %s -> %s",
			event.Domain, event.OldStatus, event.NewStatus)

		// 构建通知事件
		notificationEvent := notification.NotificationEvent{
			Type:      "status_change",
			Domain:    event.Domain,
			Status:    string(event.NewStatus),
			OldStatus: string(event.OldStatus),
			Message:   event.Message,
			Timestamp: event.Timestamp,
		}

		// 发送通知
		notificationMgr.SendNotification(notificationEvent)
	}
}

// 显示帮助信息
func showHelp() {
	fmt.Printf(`%s v%s

使用方法:
  %s [选项]

选项:
  -h, --help     显示帮助信息
  -v, --version  显示版本信息

配置存储:
  所有配置、域名列表、通知设置均存储在 SQLite 数据库中
  数据文件：data/puff.db

更多信息请查看 README.md 文件
`, AppName, AppVersion, os.Args[0])
}

// 显示版本信息
func showVersion() {
	fmt.Printf("%s v%s\n", AppName, AppVersion)
	fmt.Println("构建时间:", getBuildTime())
	fmt.Println("Go版本:", getGoVersion())
}

// 获取构建时间（在实际构建时可以通过ldflags注入）
func getBuildTime() string {
	return "development"
}

// 获取Go版本
func getGoVersion() string {
	return "go1.21+"
}
