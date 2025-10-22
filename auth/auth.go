package auth

import (
	"crypto/subtle"
	"fmt"
	"sync"
)

// Authenticator 认证器
type Authenticator struct {
	username     string
	password     string
	sessionStore *SessionStore
	mu           sync.RWMutex
}

// NewAuthenticator 创建认证器
func NewAuthenticator(username, password string) *Authenticator {
	return &Authenticator{
		username:     username,
		password:     password,
		sessionStore: NewSessionStore(),
	}
}

// Login 用户登录
func (a *Authenticator) Login(username, password string) (*Session, error) {
	if !a.validateCredentials(username, password) {
		return nil, fmt.Errorf("用户名或密码错误")
	}

	// 创建会话
	session := a.sessionStore.CreateSession()
	return session, nil
}

// Logout 用户登出
func (a *Authenticator) Logout(sessionID string) error {
	a.sessionStore.DeleteSession(sessionID)
	return nil
}

// ValidateSession 验证会话
func (a *Authenticator) ValidateSession(sessionID string) (*Session, error) {
	session := a.sessionStore.GetSession(sessionID)
	if session == nil {
		return nil, fmt.Errorf("会话不存在或已过期")
	}

	if session.IsExpired() {
		a.sessionStore.DeleteSession(sessionID)
		return nil, fmt.Errorf("会话已过期")
	}

	// 更新最后访问时间
	session.UpdateLastAccess()
	return session, nil
}

// validateCredentials 验证用户名和密码
func (a *Authenticator) validateCredentials(username, password string) bool {
	if a.username == "" || a.password == "" {
		return false
	}

	// 直接比较用户名和密码（使用常量时间比较防止时序攻击）
	usernameMatch := subtle.ConstantTimeCompare([]byte(a.username), []byte(username)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(a.password), []byte(password)) == 1

	return usernameMatch && passwordMatch
}

// ChangePassword 更改密码
func (a *Authenticator) ChangePassword(username, oldPassword, newPassword string) error {
	if !a.validateCredentials(username, oldPassword) {
		return fmt.Errorf("用户名或原密码错误")
	}

	if newPassword == "" {
		return fmt.Errorf("新密码不能为空")
	}

	if len(newPassword) < 6 {
		return fmt.Errorf("新密码长度不能少于6位")
	}

	a.password = newPassword

	// 清除所有现有会话，强制重新登录
	a.sessionStore.ClearAllSessions()

	return nil
}

// GetActiveSessions 获取活跃会话列表
func (a *Authenticator) GetActiveSessions() []*Session {
	return a.sessionStore.GetActiveSessions()
}

// GetSessionCount 获取活跃会话数量
func (a *Authenticator) GetSessionCount() int {
	return a.sessionStore.GetSessionCount()
}

// IsValidSession 检查会话是否有效
func (a *Authenticator) IsValidSession(sessionToken string) bool {
	return a.sessionStore.IsValidSession(sessionToken)
}

// ValidatePassword 验证密码
func (a *Authenticator) ValidatePassword(password string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	passwordMatch := subtle.ConstantTimeCompare([]byte(a.password), []byte(password)) == 1
	return passwordMatch
}

// CleanupExpiredSessions 清理过期会话
func (a *Authenticator) CleanupExpiredSessions() int {
	return a.sessionStore.CleanupExpiredSessions()
}

// GetStats 获取认证统计信息
func (a *Authenticator) GetStats() map[string]interface{} {
	stats := a.sessionStore.GetStats()
	stats["has_credentials"] = a.username != "" && a.password != ""
	stats["username"] = a.username
	return stats
}

// AuthMiddleware 认证中间件
func (a *Authenticator) AuthMiddleware(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("未提供会话ID")
	}

	_, err := a.ValidateSession(sessionID)
	return err
}

// IsPasswordSet 检查是否设置了密码
func (a *Authenticator) IsPasswordSet() bool {
	return a.username != "" && a.password != ""
}

// RequireAuth 检查是否需要认证
func (a *Authenticator) RequireAuth() bool {
	return a.IsPasswordSet()
}

// UpdatePassword 更新密码
func (a *Authenticator) UpdatePassword(newPassword string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(newPassword) < 6 {
		return fmt.Errorf("新密码长度不能少于6位")
	}

	a.password = newPassword

	// 清除所有现有会话，强制重新登录
	a.sessionStore.ClearAllSessions()

	return nil
}

// UpdateUsername 更新用户名
func (a *Authenticator) UpdateUsername(newUsername string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(newUsername) < 3 {
		return fmt.Errorf("用户名长度不能少于3位")
	}

	a.username = newUsername
	return nil
}
