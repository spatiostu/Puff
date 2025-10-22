package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"Puff/auth"
	"Puff/config"
	"Puff/core"
	"Puff/notification"
	"Puff/web"
)

const (
	AppName    = "Puff"
	AppVersion = "1.0.0"
)

func main() {
	fmt.Printf("%s v%s\n", AppName, AppVersion)
	fmt.Println("正在启动...")

	// 确保.env文件存在
	if err := config.CreateDefaultEnvFile(); err != nil {
		log.Printf("创建默认.env文件失败: %v", err)
	}

	// 加载.env文件
	if err := config.LoadEnvFile(); err != nil {
		log.Printf("加载.env文件失败: %v", err)
	}

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置验证失败: %v", err)
	}

	// 创建域名监控器
	monitor := core.NewMonitor(cfg)

	// 加载域名列表
	if err := monitor.LoadDomains(); err != nil {
		log.Fatalf("加载域名列表失败: %v", err)
	}

	// 创建认证器
	authenticator := auth.NewAuthenticator(cfg.Server.Username, cfg.Server.Password)

	// 创建通知管理器
	notificationMgr := notification.NewNotificationManager()

	// 添加邮件通知器
	if cfg.SMTP.Enabled {
		emailNotifier := notification.NewEmailNotifier(cfg.SMTP)
		notificationMgr.AddNotifier(emailNotifier)
		log.Println("邮件通知器已启用")
	}

	// 添加Telegram通知器
	if cfg.Telegram.Enabled {
		telegramNotifier := notification.NewTelegramNotifier(cfg.Telegram)
		notificationMgr.AddNotifier(telegramNotifier)
		log.Println("Telegram通知器已启用")
	}

	// 启动通知管理器
	notificationMgr.Start()

	// 启动通知处理协程
	go handleNotifications(monitor, notificationMgr)

	// 创建Web服务器
	webServer := web.NewServer(cfg, monitor, authenticator, notificationMgr)

	// 启动监控器
	if err := monitor.Start(); err != nil {
		log.Printf("警告: 启动监控器失败: %v", err)
	} else {
		log.Println("域名监控器已启动")
	}

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动Web服务器
	go func() {
		log.Printf("Web服务器启动在端口 %s", cfg.Server.Port)
		log.Printf("访问地址: http://localhost:%s", cfg.Server.Port)

		if err := webServer.Start(); err != nil {
			log.Printf("Web服务器启动失败: %v", err)
			cancel()
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		log.Printf("收到信号: %v", sig)
	case <-ctx.Done():
		log.Println("应用程序上下文已取消")
	}

	// 优雅关闭
	log.Println("正在关闭应用程序...")

	// 创建关闭超时上下文
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// 关闭各个组件
	go func() {
		// 停止监控器
		monitor.Stop()
		log.Println("域名监控器已停止")

		// 停止通知管理器
		notificationMgr.Stop()
		log.Println("通知管理器已停止")

		// 停止Web服务器
		if err := webServer.Stop(); err != nil {
			log.Printf("警告: 停止Web服务器时出错: %v", err)
		} else {
			log.Println("Web服务器已停止")
		}

		// 清理认证器
		authenticator.CleanupExpiredSessions()
		log.Println("认证器已清理")

		shutdownCancel()
	}()

	// 等待关闭完成或超时
	<-shutdownCtx.Done()

	if shutdownCtx.Err() == context.DeadlineExceeded {
		log.Println("警告: 关闭超时，强制退出")
	} else {
		log.Println("应用程序已优雅关闭")
	}
}

// handleNotifications 处理通知事件
func handleNotifications(monitor *core.Monitor, notificationMgr *notification.NotificationManager) {
	for event := range monitor.GetNotifications() {
		log.Printf("📧 域名状态变化通知: %s %s -> %s",
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

// 初始化日志
func init() {
	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 可以在这里添加日志文件输出
	// 例如：log.SetOutput(logFile)
}

// 显示帮助信息
func showHelp() {
	fmt.Printf(`%s v%s

使用方法:
  %s [选项]

选项:
  -h, --help     显示帮助信息
  -v, --version  显示版本信息

环境变量:
  请参考 .env.example 文件配置环境变量

配置文件:
  domains.yml    域名列表配置文件
  .env          环境变量配置文件

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
