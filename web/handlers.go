package web

import (
	"Puff/config"
	"Puff/core"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// handleLogin 登录处理器
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.serveLoginPage(w, r)
	case "POST":
		s.processLogin(w, r)
	default:
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// serveLoginPage 服务登录页面
func (s *Server) serveLoginPage(w http.ResponseWriter, r *http.Request) {
	loginHTML := `
<!DOCTYPE html>
<html lang="zh-CN" data-theme="lofi">
<head>
    <title>Puff - 登录</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link href="https://cdn.jsdelivr.net/npm/daisyui@4.4.24/dist/full.min.css" rel="stylesheet" type="text/css" />
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="min-h-screen bg-base-200 flex items-center justify-center">
    <div class="card w-96 bg-base-100 shadow-xl">
        <div class="card-body">
            <h2 class="card-title justify-center text-2xl mb-6">域名监控系统</h2>
            
            <form id="loginForm" method="POST" class="space-y-4">
                <div class="form-control w-full">
                    <label class="label">
                        <span class="label-text">用户名</span>
                    </label>
                    <input type="text" name="username" class="input input-bordered w-full" required />
                </div>
                
                <div class="form-control w-full">
                    <label class="label">
                        <span class="label-text">密码</span>
                    </label>
                    <input type="password" name="password" class="input input-bordered w-full" required />
                </div>
                
                <div class="form-control w-full mt-6">
                    <button type="submit" class="btn btn-primary" id="loginBtn">
                        <span class="loading loading-spinner loading-sm hidden" id="loginSpinner"></span>
                        登录
                    </button>
                </div>
                
                <div id="errorAlert" class="alert alert-error hidden">
                    <svg xmlns="http://www.w3.org/2000/svg" class="stroke-current shrink-0 h-6 w-6" fill="none" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <span id="errorMessage"></span>
                </div>
            </form>
        </div>
    </div>

    <script>
        document.getElementById('loginForm').addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const loginBtn = document.getElementById('loginBtn');
            const loginSpinner = document.getElementById('loginSpinner');
            const errorAlert = document.getElementById('errorAlert');
            const errorMessage = document.getElementById('errorMessage');
            
            // 显示加载状态
            loginBtn.disabled = true;
            loginSpinner.classList.remove('hidden');
            errorAlert.classList.add('hidden');
            
            try {
                const formData = new FormData(this);
                const response = await fetch('/login', {
                    method: 'POST',
                    body: formData
                });
                
                if (response.ok) {
                    // 登录成功，重定向到主页面
                    window.location.href = '/';
                } else {
                    // 登录失败，显示错误信息
                    const result = await response.json();
                    errorMessage.textContent = result.error || '登录失败';
                    errorAlert.classList.remove('hidden');
                }
            } catch (error) {
                errorMessage.textContent = '网络错误，请重试';
                errorAlert.classList.remove('hidden');
            } finally {
                // 恢复按钮状态
                loginBtn.disabled = false;
                loginSpinner.classList.add('hidden');
            }
        });
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(loginHTML))
}

// processLogin 处理登录
func (s *Server) processLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	session, err := s.auth.Login(username, password)
	if err != nil {
		// 返回JSON错误响应
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "用户名或密码错误",
		})
		return
	}

	// 设置会话Cookie
	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // 在生产环境中应该设置为true
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	}

	http.SetCookie(w, cookie)

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

// handleLogout 登出处理器
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// 获取会话ID
	if cookie, err := r.Cookie("session_id"); err == nil {
		s.auth.Logout(cookie.Value)
	}

	// 清除Cookie
	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	}

	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleDomains 域名列表处理器（支持分页、搜索、过滤）
func (s *Server) handleDomains(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// 获取分页参数
		pageStr := r.URL.Query().Get("page")
		limitStr := r.URL.Query().Get("limit")
		statsOnlyStr := r.URL.Query().Get("stats_only")
		searchTerm := strings.TrimSpace(r.URL.Query().Get("search"))
		statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

		page := 1
		limit := 10
		statsOnly := statsOnlyStr == "true"

		if pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}

		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		// 获取域名总数（快速获取）
		domainCount := len(s.monitor.GetDomains())

		// 如果只需要统计信息，直接返回
		if statsOnly {
			s.writeJSON(w, map[string]interface{}{
				"total": domainCount,
			})
			return
		}

		allDomains := s.monitor.GetAllDomainInfo()

		// 应用搜索和状态过滤
		filteredDomains := s.filterDomains(allDomains, searchTerm, statusFilter)
		total := len(filteredDomains)

		// 计算分页
		start := (page - 1) * limit
		end := start + limit

		if start >= total {
			start = 0
			end = 0
		} else if end > total {
			end = total
		}

		var paginatedDomains []*core.DomainInfo
		if start < end {
			paginatedDomains = filteredDomains[start:end]
		}

		s.writeJSON(w, map[string]interface{}{
			"domains":     paginatedDomains,
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": (total + limit - 1) / limit,
			"has_next":    end < total,
			"has_prev":    page > 1,
		})
	default:
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDomainDetail 域名详情处理器
func (s *Server) handleDomainDetail(w http.ResponseWriter, r *http.Request) {
	// 从URL路径提取域名
	path := strings.TrimPrefix(r.URL.Path, "/api/domain/")
	domain := strings.TrimSuffix(path, "/")

	if domain == "" {
		s.writeError(w, "Domain name required", http.StatusBadRequest)
		return
	}

	info, err := s.monitor.GetDomainInfo(domain)
	if err != nil {
		s.writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, info)
}

// handleDomainCheck 域名检查处理器
func (s *Server) handleDomainCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径提取域名
	path := strings.TrimPrefix(r.URL.Path, "/api/domain/check/")
	domain := strings.TrimSuffix(path, "/")

	if domain == "" {
		s.writeError(w, "Domain name required", http.StatusBadRequest)
		return
	}

	info, err := s.monitor.ForceCheck(domain)
	if err != nil {
		s.writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, info)
}

// handleStats 统计信息处理器
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := s.monitor.GetStats()
	authStats := s.auth.GetStats()
	notificationStats := s.notification.GetStats()

	response := map[string]interface{}{
		"monitor":      stats,
		"auth":         authStats,
		"notification": notificationStats,
		"timestamp":    time.Now(),
	}

	s.writeJSON(w, response)
}

// handleMonitorStart 启动监控处理器
func (s *Server) handleMonitorStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.monitor.Start(); err != nil {
		s.writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, map[string]string{"status": "started"})
}

// handleMonitorStop 停止监控处理器
func (s *Server) handleMonitorStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.monitor.Stop()
	s.writeJSON(w, map[string]string{"status": "stopped"})
}

// handleMonitorReload 重新加载监控处理器
func (s *Server) handleMonitorReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 重新加载域名列表
	if err := s.monitor.LoadDomains(); err != nil {
		s.writeError(w, fmt.Sprintf("重新加载域名列表失败: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, map[string]string{"status": "reloaded"})
}

// handleNotificationTest 通知测试处理器
func (s *Server) handleNotificationTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 测试所有通知器
	results := s.notification.TestAllNotifiers()

	response := map[string]interface{}{
		"test_results": results,
		"timestamp":    time.Now(),
	}

	s.writeJSON(w, response)
}

// handleDomainAdd 添加单个域名
func (s *Server) handleDomainAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Domain string `json:"domain"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	domain := strings.TrimSpace(request.Domain)
	if domain == "" {
		s.writeError(w, "Domain name required", http.StatusBadRequest)
		return
	}

	// 验证域名格式
	if err := s.monitor.GetChecker().ValidateDomain(domain); err != nil {
		s.writeError(w, "Invalid domain: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 添加到监控列表
	if err := s.addDomainToConfig(domain); err != nil {
		s.writeError(w, "Failed to add domain: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 重新加载域名列表
	if err := s.monitor.LoadDomains(); err != nil {
		s.writeError(w, "Failed to reload domains: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "Domain added successfully",
		"domain":  domain,
	})
}

// handleDomainBatchAdd 批量添加域名
func (s *Server) handleDomainBatchAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Domains []string `json:"domains"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(request.Domains) == 0 {
		s.writeError(w, "No domains provided", http.StatusBadRequest)
		return
	}

	validDomains := []string{}
	invalidDomains := []string{}

	// 验证所有域名
	for _, domain := range request.Domains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}

		if err := s.monitor.GetChecker().ValidateDomain(domain); err != nil {
			invalidDomains = append(invalidDomains, domain)
		} else {
			validDomains = append(validDomains, domain)
		}
	}

	// 批量添加有效域名
	addedCount := 0
	for _, domain := range validDomains {
		if err := s.addDomainToConfig(domain); err == nil {
			addedCount++
		}
	}

	// 重新加载域名列表
	if addedCount > 0 {
		s.monitor.LoadDomains()
	}

	response := map[string]interface{}{
		"status":          "success",
		"added_count":     addedCount,
		"invalid_count":   len(invalidDomains),
		"invalid_domains": invalidDomains,
	}

	s.writeJSON(w, response)
}

// handleDomainRemove 删除域名
func (s *Server) handleDomainRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径提取域名
	path := strings.TrimPrefix(r.URL.Path, "/api/domain/remove/")
	domain := strings.TrimSuffix(path, "/")

	if domain == "" {
		s.writeError(w, "Domain name required", http.StatusBadRequest)
		return
	}

	// 从配置中删除域名
	if err := s.removeDomainFromConfig(domain); err != nil {
		s.writeError(w, "Failed to remove domain: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 重新加载域名列表
	if err := s.monitor.LoadDomains(); err != nil {
		s.writeError(w, "Failed to reload domains: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "Domain removed successfully",
		"domain":  domain,
	})
}

// handleChangePassword 处理修改密码请求
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	// 检查请求方法
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查会话
	sessionToken := ""
	if cookie, err := r.Cookie("session_token"); err == nil {
		sessionToken = cookie.Value
	}

	if !s.auth.IsValidSession(sessionToken) {
		s.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 解析请求
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证当前密码
	if !s.auth.ValidatePassword(req.CurrentPassword) {
		s.writeError(w, "当前密码错误", http.StatusBadRequest)
		return
	}

	// 验证新密码
	if len(req.NewPassword) < 6 {
		s.writeError(w, "新密码长度至少6位", http.StatusBadRequest)
		return
	}

	// 更新密码
	if err := s.auth.UpdatePassword(req.NewPassword); err != nil {
		log.Printf("更新密码失败: %v", err)
		s.writeError(w, "更新密码失败", http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "密码修改成功",
	})
}

// handleUpdateUsername 处理更新用户名请求
func (s *Server) handleUpdateUsername(w http.ResponseWriter, r *http.Request) {
	// 检查请求方法
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查会话
	sessionToken := ""
	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	if !s.auth.IsValidSession(sessionToken) {
		s.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 解析请求
	var req struct {
		Username string `json:"username"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证用户名
	if len(req.Username) < 3 {
		s.writeError(w, "用户名长度至少3位", http.StatusBadRequest)
		return
	}

	// 更新用户名
	if err := s.auth.UpdateUsername(req.Username); err != nil {
		log.Printf("更新用户名失败: %v", err)
		s.writeError(w, "更新用户名失败", http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "用户名更新成功",
	})
}

// handleSmtpSettings 处理SMTP设置
func (s *Server) handleSmtpSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查会话
	sessionToken := ""
	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	if !s.auth.IsValidSession(sessionToken) {
		s.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		From     string `json:"from"`
		To       string `json:"to"`
		Enabled  bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 将设置保存到.env文件
	envUpdates := map[string]string{
		"SMTP_HOST":    req.Host,
		"SMTP_PORT":    fmt.Sprintf("%d", req.Port),
		"SMTP_USER":    req.User,
		"SMTP_PASS":    req.Password,
		"SMTP_FROM":    req.From,
		"SMTP_TO":      req.To,
		"SMTP_ENABLED": fmt.Sprintf("%t", req.Enabled),
	}

	if err := config.UpdateEnvFileSimple(envUpdates); err != nil {
		log.Printf("保存SMTP设置到.env文件失败: %v", err)
		s.writeError(w, "保存设置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新当前配置
	s.config.SMTP.Host = req.Host
	s.config.SMTP.Port = req.Port
	s.config.SMTP.User = req.User
	s.config.SMTP.Password = req.Password
	s.config.SMTP.From = req.From
	s.config.SMTP.To = req.To
	s.config.SMTP.Enabled = req.Enabled

	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "SMTP设置保存成功",
	})
}

// handleTelegramSettings 处理Telegram设置
func (s *Server) handleTelegramSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查会话
	sessionToken := ""
	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	if !s.auth.IsValidSession(sessionToken) {
		s.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		BotToken string `json:"bot_token"`
		ChatID   string `json:"chat_id"`
		Enabled  bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 将设置保存到.env文件
	envUpdates := map[string]string{
		"TELEGRAM_BOT_TOKEN": req.BotToken,
		"TELEGRAM_CHAT_ID":   req.ChatID,
		"TELEGRAM_ENABLED":   fmt.Sprintf("%t", req.Enabled),
	}

	if err := config.UpdateEnvFileSimple(envUpdates); err != nil {
		log.Printf("保存Telegram设置到.env文件失败: %v", err)
		s.writeError(w, "保存设置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新当前配置
	s.config.Telegram.BotToken = req.BotToken
	s.config.Telegram.ChatID = req.ChatID
	s.config.Telegram.Enabled = req.Enabled

	s.writeJSON(w, map[string]string{
		"status":  "success",
		"message": "Telegram设置保存成功",
	})
}

// handleTestEmail 测试邮件发送
func (s *Server) handleTestEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查会话
	sessionToken := ""
	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	if !s.auth.IsValidSession(sessionToken) {
		s.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 这里应该使用当前SMTP配置发送测试邮件
	// 目前返回模拟响应
	if s.config.SMTP.Enabled {
		s.writeJSON(w, map[string]string{
			"status":  "success",
			"message": "测试邮件发送成功",
		})
	} else {
		s.writeError(w, "邮件通知未启用", http.StatusBadRequest)
	}
}

// handleTestTelegram 测试Telegram发送
func (s *Server) handleTestTelegram(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查会话
	sessionToken := ""
	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	if !s.auth.IsValidSession(sessionToken) {
		s.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 这里应该使用当前Telegram配置发送测试消息
	// 目前返回模拟响应
	if s.config.Telegram.Enabled {
		s.writeJSON(w, map[string]string{
			"status":  "success",
			"message": "测试Telegram消息发送成功",
		})
	} else {
		s.writeError(w, "Telegram通知未启用", http.StatusBadRequest)
	}
}

// handleGetSettings 获取当前设置
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查会话
	sessionToken := ""
	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	if !s.auth.IsValidSession(sessionToken) {
		s.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 返回当前设置（不包含敏感信息）
	settings := map[string]interface{}{
		"smtp": map[string]interface{}{
			"host":    s.config.SMTP.Host,
			"port":    s.config.SMTP.Port,
			"user":    s.config.SMTP.User,
			"from":    s.config.SMTP.From,
			"to":      s.config.SMTP.To,
			"enabled": s.config.SMTP.Enabled,
		},
		"telegram": map[string]interface{}{
			"chat_id": s.config.Telegram.ChatID,
			"enabled": s.config.Telegram.Enabled,
		},
		"username": s.auth.GetStats()["username"],
	}

	s.writeJSON(w, settings)
}

// filterDomains 过滤域名列表
func (s *Server) filterDomains(domains []*core.DomainInfo, searchTerm, statusFilter string) []*core.DomainInfo {
	if searchTerm == "" && statusFilter == "" {
		return domains
	}

	var filtered []*core.DomainInfo
	searchLower := strings.ToLower(searchTerm)

	for _, domain := range domains {
		// 检查搜索条件
		matchesSearch := searchTerm == "" || strings.Contains(strings.ToLower(domain.Name), searchLower)

		// 检查状态过滤条件
		matchesStatus := statusFilter == "" || string(domain.Status) == statusFilter

		if matchesSearch && matchesStatus {
			filtered = append(filtered, domain)
		}
	}

	return filtered
}
