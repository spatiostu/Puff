package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Session 用户会话
type Session struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	LastAccess time.Time `json:"last_access"`
	ExpiresAt  time.Time `json:"expires_at"`
	UserAgent  string    `json:"user_agent,omitempty"`
	IPAddress  string    `json:"ip_address,omitempty"`
	IsActive   bool      `json:"is_active"`
}

// SessionStore 会话存储
type SessionStore struct {
	sessions    map[string]*Session
	mu          sync.RWMutex
	maxAge      time.Duration
	cleanupTick *time.Ticker
}

// NewSessionStore 创建会话存储
func NewSessionStore() *SessionStore {
	store := &SessionStore{
		sessions: make(map[string]*Session),
		maxAge:   24 * time.Hour, // 默认24小时过期
	}

	// 启动清理协程
	store.startCleanup()

	return store
}

// CreateSession 创建新会话
func (s *SessionStore) CreateSession() *Session {
	sessionID := s.generateSessionID()
	now := time.Now()

	session := &Session{
		ID:         sessionID,
		CreatedAt:  now,
		LastAccess: now,
		ExpiresAt:  now.Add(s.maxAge),
		IsActive:   true,
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	return session
}

// GetSession 获取会话
func (s *SessionStore) GetSession(sessionID string) *Session {
	s.mu.RLock()
	session, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists || session.IsExpired() {
		return nil
	}

	return session
}

// DeleteSession 删除会话
func (s *SessionStore) DeleteSession(sessionID string) {
	s.mu.Lock()
	if session, exists := s.sessions[sessionID]; exists {
		session.IsActive = false
		delete(s.sessions, sessionID)
	}
	s.mu.Unlock()
}

// UpdateLastAccess 更新最后访问时间
func (s *Session) UpdateLastAccess() {
	s.LastAccess = time.Now()
	s.ExpiresAt = time.Now().Add(24 * time.Hour) // 延长过期时间
}

// IsExpired 检查会话是否过期
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// GetAge 获取会话年龄
func (s *Session) GetAge() time.Duration {
	return time.Since(s.CreatedAt)
}

// GetIdleTime 获取空闲时间
func (s *Session) GetIdleTime() time.Duration {
	return time.Since(s.LastAccess)
}

// generateSessionID 生成会话ID
func (s *SessionStore) generateSessionID() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// 如果随机数生成失败，使用时间戳作为备选
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(bytes)
}

// GetActiveSessions 获取所有活跃会话
func (s *SessionStore) GetActiveSessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []*Session
	for _, session := range s.sessions {
		if session.IsActive && !session.IsExpired() {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// GetSessionCount 获取活跃会话数量
func (s *SessionStore) GetSessionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, session := range s.sessions {
		if session.IsActive && !session.IsExpired() {
			count++
		}
	}

	return count
}

// CleanupExpiredSessions 清理过期会话
func (s *SessionStore) CleanupExpiredSessions() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleaned := 0
	for id, session := range s.sessions {
		if session.IsExpired() {
			delete(s.sessions, id)
			cleaned++
		}
	}

	return cleaned
}

// ClearAllSessions 清空所有会话
func (s *SessionStore) ClearAllSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, session := range s.sessions {
		session.IsActive = false
	}
	s.sessions = make(map[string]*Session)
}

// SetMaxAge 设置会话最大生存时间
func (s *SessionStore) SetMaxAge(maxAge time.Duration) {
	s.maxAge = maxAge
}

// GetMaxAge 获取会话最大生存时间
func (s *SessionStore) GetMaxAge() time.Duration {
	return s.maxAge
}

// startCleanup 启动清理协程
func (s *SessionStore) startCleanup() {
	s.cleanupTick = time.NewTicker(1 * time.Hour) // 每小时清理一次

	go func() {
		for range s.cleanupTick.C {
			s.CleanupExpiredSessions()
		}
	}()
}

// StopCleanup 停止清理协程
func (s *SessionStore) StopCleanup() {
	if s.cleanupTick != nil {
		s.cleanupTick.Stop()
	}
}

// GetStats 获取会话存储统计信息
func (s *SessionStore) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalSessions := len(s.sessions)
	activeSessions := 0
	expiredSessions := 0

	var oldestSession, newestSession time.Time
	var longestIdle time.Duration

	for _, session := range s.sessions {
		if session.IsExpired() {
			expiredSessions++
		} else if session.IsActive {
			activeSessions++
		}

		// 统计最老和最新的会话
		if oldestSession.IsZero() || session.CreatedAt.Before(oldestSession) {
			oldestSession = session.CreatedAt
		}
		if newestSession.IsZero() || session.CreatedAt.After(newestSession) {
			newestSession = session.CreatedAt
		}

		// 统计最长空闲时间
		idleTime := session.GetIdleTime()
		if idleTime > longestIdle {
			longestIdle = idleTime
		}
	}

	return map[string]interface{}{
		"total_sessions":   totalSessions,
		"active_sessions":  activeSessions,
		"expired_sessions": expiredSessions,
		"max_age":          s.maxAge.String(),
		"oldest_session":   oldestSession,
		"newest_session":   newestSession,
		"longest_idle":     longestIdle.String(),
	}
}

// ExtendSession 延长会话生存时间
func (s *SessionStore) ExtendSession(sessionID string, duration time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("会话不存在")
	}

	if session.IsExpired() {
		return fmt.Errorf("会话已过期")
	}

	session.ExpiresAt = session.ExpiresAt.Add(duration)
	return nil
}

// SetSessionInfo 设置会话信息
func (s *SessionStore) SetSessionInfo(sessionID, userAgent, ipAddress string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("会话不存在")
	}

	session.UserAgent = userAgent
	session.IPAddress = ipAddress

	return nil
}

// IsValidSession 检查会话是否有效
func (s *SessionStore) IsValidSession(sessionToken string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionToken]
	if !exists {
		return false
	}

	if session.IsExpired() {
		// 清理过期会话
		delete(s.sessions, sessionToken)
		return false
	}

	return true
}
