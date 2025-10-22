package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"Puff/auth"
	"Puff/config"
	"Puff/core"
	"Puff/notification"
)

// Server Web服务器
type Server struct {
	config       *config.Config
	monitor      *core.Monitor
	auth         *auth.Authenticator
	notification *notification.NotificationManager
	httpServer   *http.Server
}

// NewServer 创建新的Web服务器
func NewServer(cfg *config.Config, monitor *core.Monitor, authenticator *auth.Authenticator, notificationMgr *notification.NotificationManager) *Server {
	return &Server{
		config:       cfg,
		monitor:      monitor,
		auth:         authenticator,
		notification: notificationMgr,
	}
}

// Start 启动Web服务器
func (s *Server) Start() error {
	// 设置路由
	mux := http.NewServeMux()
	s.setupRoutes(mux)

	// 创建HTTP服务器
	s.httpServer = &http.Server{
		Addr:         ":" + s.config.Server.Port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Web服务器启动在端口 %s", s.config.Server.Port)

	// 启动服务器
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("启动Web服务器失败: %v", err)
	}

	return nil
}

// Stop 停止Web服务器
func (s *Server) Stop() error {
	if s.httpServer != nil {
		// 使用优雅关闭，而不是强制关闭
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			// 如果优雅关闭失败，则强制关闭
			log.Printf("优雅关闭失败，执行强制关闭: %v", err)
			return s.httpServer.Close()
		}
		return nil
	}
	return nil
}

// setupRoutes 设置路由
func (s *Server) setupRoutes(mux *http.ServeMux) {
	// 静态文件服务
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))

	// 主页面
	mux.HandleFunc("/", s.handleIndex)

	// 认证相关
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.handleLogout)

	// API接口
	mux.HandleFunc("/api/domains", s.withAuth(s.handleDomains))
	mux.HandleFunc("/api/domain/", s.withAuth(s.handleDomainDetail))
	mux.HandleFunc("/api/domain/check/", s.withAuth(s.handleDomainCheck))
	mux.HandleFunc("/api/domain/add", s.withAuth(s.handleDomainAdd))
	mux.HandleFunc("/api/domain/batch-add", s.withAuth(s.handleDomainBatchAdd))
	mux.HandleFunc("/api/domain/remove/", s.withAuth(s.handleDomainRemove))
	mux.HandleFunc("/api/stats", s.withAuth(s.handleStats))
	mux.HandleFunc("/api/monitor/start", s.withAuth(s.handleMonitorStart))
	mux.HandleFunc("/api/monitor/stop", s.withAuth(s.handleMonitorStop))
	mux.HandleFunc("/api/monitor/reload", s.withAuth(s.handleMonitorReload))
	mux.HandleFunc("/api/notification/test", s.withAuth(s.handleNotificationTest))
	mux.HandleFunc("/api/change-password", s.withAuth(s.handleChangePassword))
	mux.HandleFunc("/api/update-username", s.withAuth(s.handleUpdateUsername))
	mux.HandleFunc("/api/settings/smtp", s.withAuth(s.handleSmtpSettings))
	mux.HandleFunc("/api/settings/telegram", s.withAuth(s.handleTelegramSettings))
	mux.HandleFunc("/api/settings", s.withAuth(s.handleGetSettings))
	mux.HandleFunc("/api/test/email", s.withAuth(s.handleTestEmail))
	mux.HandleFunc("/api/test/telegram", s.withAuth(s.handleTestTelegram))

	// 健康检查
	mux.HandleFunc("/health", s.handleHealth)
}

// withAuth 认证中间件
func (s *Server) withAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 检查是否需要认证
		if !s.auth.RequireAuth() {
			handler(w, r)
			return
		}

		// 从Cookie获取会话ID
		cookie, err := r.Cookie("session_id")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 验证会话
		if err := s.auth.AuthMiddleware(cookie.Value); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		handler(w, r)
	}
}

// enableCORS 启用CORS
func (s *Server) enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

// writeJSON 写入JSON响应
func (s *Server) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	s.enableCORS(w)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// writeError 写入错误响应
func (s *Server) writeError(w http.ResponseWriter, message string, code int) {
	s.enableCORS(w)
	http.Error(w, message, code)
}

// handleIndex 主页面处理器
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// 检查是否需要认证
	if s.auth.RequireAuth() {
		// 检查是否已登录
		if cookie, err := r.Cookie("session_id"); err != nil || s.auth.AuthMiddleware(cookie.Value) != nil {
			// 未登录，重定向到登录页面
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	}

	// 服务主页面
	http.ServeFile(w, r, "web/static/index.html")
}

// handleHealth 健康检查处理器
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now(),
		"monitor":   s.monitor.IsRunning(),
		"domains":   len(s.monitor.GetDomains()),
	}

	s.writeJSON(w, health)
}
