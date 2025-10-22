package core

import (
	"fmt"
	"log"
	"sync"
	"time"

	"Puff/config"
	"Puff/storage"
)

// Monitor 域名监控器
type Monitor struct {
	checker       *DomainChecker
	config        *config.Config
	domains       []string
	isRunning     bool
	stopCh        chan struct{}
	mu            sync.RWMutex
	notifications chan StatusChangeEvent
	lastStatus    map[string]DomainStatus // 记录最后一次状态，用于变更检测
	lastResults   []*DomainInfo           // 缓存最后一次查询结果
	lastStatsTime time.Time               // 最后一次统计时间
	statsCache    map[string]interface{}  // 统计缓存
	startTime     time.Time               // 启动时间
	cache         *storage.Cache          // 域名查询结果缓存
}

// NewMonitor 创建新的监控器
func NewMonitor(cfg *config.Config) *Monitor {
	return &Monitor{
		checker:       NewDomainChecker(cfg),
		config:        cfg,
		stopCh:        make(chan struct{}),
		notifications: make(chan StatusChangeEvent, 1000),
		lastStatus:    make(map[string]DomainStatus),
		lastResults:   make([]*DomainInfo, 0),
		statsCache:    make(map[string]interface{}),
		startTime:     time.Now(),
		cache:         storage.NewCache(cfg.Monitor.CacheDuration),
	}
}

// LoadDomains 加载域名列表
func (m *Monitor) LoadDomains() error {
	domains, err := config.LoadDomains("domains.yml")
	if err != nil {
		return fmt.Errorf("加载域名列表失败: %v", err)
	}

	// 验证域名格式
	validDomains := make([]string, 0, len(domains))
	for _, domain := range domains {
		if err := m.checker.ValidateDomain(domain); err != nil {
			log.Printf("警告: 跳过无效域名 %s: %v", domain, err)
			continue
		}
		validDomains = append(validDomains, domain)
	}

	m.mu.Lock()
	m.domains = validDomains
	// 清除缓存，因为域名列表已更改
	m.cache.Clear()
	m.mu.Unlock()

	log.Printf("加载了 %d 个有效域名", len(validDomains))
	return nil
}

// Start 启动监控
func (m *Monitor) Start() error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return fmt.Errorf("监控器已在运行")
	}

	if len(m.domains) == 0 {
		m.mu.Unlock()
		return fmt.Errorf("没有要监控的域名")
	}

	m.isRunning = true
	m.mu.Unlock()

	log.Printf("启动域名监控，间隔: %v, 域名数量: %d", m.config.Monitor.CheckInterval, len(m.domains))

	// 启动监控协程
	go m.monitorLoop()

	return nil
}

// Stop 停止监控
func (m *Monitor) Stop() {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return
	}

	m.isRunning = false
	m.mu.Unlock()

	close(m.stopCh)
	log.Println("域名监控已停止")
}

// IsRunning 检查是否正在运行
func (m *Monitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

// GetDomains 获取域名列表
func (m *Monitor) GetDomains() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domains := make([]string, len(m.domains))
	copy(domains, m.domains)
	return domains
}

// GetDomainInfo 获取域名信息（优化版本，使用智能缓存）
func (m *Monitor) GetDomainInfo(domain string) (*DomainInfo, error) {
	// 先尝试从缓存获取
	cacheKey := fmt.Sprintf("domain:%s", domain)
	if cachedInfo := m.cache.Get(cacheKey); cachedInfo != nil {
		if domainInfo, ok := cachedInfo.(*DomainInfo); ok {
			log.Printf("从缓存获取域名 %s 的信息", domain)
			return domainInfo, nil
		}
	}

	// 缓存中没有，进行即时查询
	log.Printf("缓存未命中，查询域名 %s", domain)
	info := m.checker.CheckDomain(domain)

	// 使用智能缓存策略存储结果
	if info.Status != StatusError {
		cacheDuration := info.GetSmartCacheDuration()
		m.cache.SetWithDuration(cacheKey, info, cacheDuration)
		log.Printf("域名 %s 查询完成，状态: %s, 缓存时间: %v", domain, info.Status, cacheDuration)
	} else {
		// 错误状态使用较短的缓存时间
		m.cache.SetWithDuration(cacheKey, info, 10*time.Minute)
		log.Printf("域名 %s 查询失败，错误: %s", domain, info.ErrorMessage)
	}

	return info, nil
}

// GetAllDomainInfo 获取所有域名信息（优化版本，使用缓存）
func (m *Monitor) GetAllDomainInfo() []*DomainInfo {
	m.mu.RLock()
	domains := make([]string, len(m.domains))
	copy(domains, m.domains)

	// 如果有缓存且不超过5分钟，直接返回缓存
	if len(m.lastResults) > 0 && time.Since(m.lastStatsTime) < 5*time.Minute {
		defer m.mu.RUnlock()
		return m.lastResults
	}
	m.mu.RUnlock()

	// 否则重新查询
	results := m.checker.CheckDomains(domains)

	// 更新缓存
	m.mu.Lock()
	m.lastResults = results
	m.lastStatsTime = time.Now()
	m.mu.Unlock()

	return results
}

// ForceCheck 强制检查指定域名
func (m *Monitor) ForceCheck(domain string) (*DomainInfo, error) {
	// 强制检查时清除该域名的缓存
	cacheKey := fmt.Sprintf("domain:%s", domain)
	m.cache.Delete(cacheKey)

	log.Printf("强制检查域名 %s", domain)
	info := m.checker.CheckDomain(domain)

	// 使用智能缓存策略存储新结果
	if info.Status != StatusError {
		cacheDuration := info.GetSmartCacheDuration()
		m.cache.SetWithDuration(cacheKey, info, cacheDuration)
		log.Printf("域名 %s 强制检查完成，状态: %s, 缓存时间: %v", domain, info.Status, cacheDuration)
	} else {
		m.cache.SetWithDuration(cacheKey, info, 10*time.Minute)
		log.Printf("域名 %s 强制检查失败，错误: %s", domain, info.ErrorMessage)
	}

	// 检查状态是否发生变化
	m.mu.Lock()
	lastStatus, exists := m.lastStatus[domain]
	if exists && lastStatus != info.Status {
		// 状态发生变化，发送通知
		event := StatusChangeEvent{
			Domain:    domain,
			OldStatus: lastStatus,
			NewStatus: info.Status,
			Timestamp: time.Now(),
			Message:   GetStatusChangeMessage(domain, lastStatus, info.Status),
		}

		select {
		case m.notifications <- event:
			log.Printf("域名 %s 状态变化通知已发送: %s -> %s", domain, lastStatus, info.Status)
		default:
			log.Printf("通知队列已满，丢弃域名 %s 的状态变化通知", domain)
		}
	}

	// 更新最后状态
	if info.Status != StatusError {
		m.lastStatus[domain] = info.Status
	}
	m.mu.Unlock()

	return info, nil
}

// GetNotifications 获取通知通道
func (m *Monitor) GetNotifications() <-chan StatusChangeEvent {
	return m.notifications
}

// monitorLoop 监控循环
func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(m.config.Monitor.CheckInterval)
	defer ticker.Stop()

	// 立即执行一次检查
	m.checkAllDomains()

	for {
		select {
		case <-ticker.C:
			m.checkAllDomains()
		case <-m.stopCh:
			return
		}
	}
}

// checkAllDomains 检查所有域名
func (m *Monitor) checkAllDomains() {
	start := time.Now()

	m.mu.RLock()
	domains := make([]string, len(m.domains))
	copy(domains, m.domains)
	m.mu.RUnlock()

	if len(domains) == 0 {
		return
	}

	log.Printf("开始检查 %d 个域名...", len(domains))

	// 批量检查域名
	results := m.checker.CheckDomains(domains)

	// 处理结果
	changedCount := 0
	errorCount := 0

	for i, info := range results {
		domain := domains[i]

		if info.Status == StatusError {
			errorCount++
			log.Printf("检查域名 %s 失败: %s", domain, info.ErrorMessage)
			continue
		}

		// 检查状态是否发生变化
		m.mu.Lock()
		lastStatus, exists := m.lastStatus[domain]
		if exists && lastStatus != info.Status {
			changedCount++

			// 发送状态变化通知
			event := StatusChangeEvent{
				Domain:    domain,
				OldStatus: lastStatus,
				NewStatus: info.Status,
				Timestamp: time.Now(),
				Message:   GetStatusChangeMessage(domain, lastStatus, info.Status),
			}

			// 使用非阻塞发送，避免goroutine泄漏
			select {
			case m.notifications <- event:
				log.Printf("域名 %s 状态变化: %s -> %s", domain, lastStatus, info.Status)
			default:
				log.Printf("通知队列已满，丢弃域名 %s 的状态变化通知", domain)
				// 可以考虑增加队列容量或实现持久化存储
			}
		}

		// 更新最后状态和缓存
		if info.Status != StatusError {
			m.lastStatus[domain] = info.Status
			// 使用智能缓存策略更新缓存
			cacheKey := fmt.Sprintf("domain:%s", domain)
			cacheDuration := info.GetSmartCacheDuration()
			m.cache.SetWithDuration(cacheKey, info, cacheDuration)
		}
		m.mu.Unlock()
	}

	// 更新缓存的结果
	m.mu.Lock()
	m.lastResults = results
	m.lastStatsTime = time.Now()
	m.mu.Unlock()

	duration := time.Since(start)
	log.Printf("检查完成，耗时: %v, 状态变化: %d, 错误: %d", duration, changedCount, errorCount)
}

// GetStats 获取监控统计信息（优化版本，使用缓存）
func (m *Monitor) GetStats() map[string]interface{} {
	m.mu.RLock()
	domainCount := len(m.domains)
	isRunning := m.isRunning

	// 检查是否有缓存且不超过1分钟
	if len(m.statsCache) > 0 && time.Since(m.lastStatsTime) < 1*time.Minute {
		// 更新动态数据
		m.statsCache["domain_count"] = domainCount
		m.statsCache["is_running"] = isRunning
		m.statsCache["uptime"] = time.Since(m.startTime).String()
		defer m.mu.RUnlock()
		return m.statsCache
	}
	m.mu.RUnlock()

	// 重新生成统计信息
	stats := map[string]interface{}{
		"domain_count":     domainCount,
		"is_running":       isRunning,
		"check_interval":   m.config.Monitor.CheckInterval.String(),
		"concurrent_limit": m.config.Monitor.ConcurrentLimit,
		"tracked_domains":  len(m.lastStatus),
		"uptime":           time.Since(m.startTime).String(),
	}

	// 统计各状态的域名数量（使用缓存的结果）
	statusCounts := make(map[DomainStatus]int)

	m.mu.RLock()
	if len(m.lastResults) > 0 {
		for _, info := range m.lastResults {
			statusCounts[info.Status]++
		}
	}
	m.mu.RUnlock()

	stats["status_counts"] = statusCounts

	// 添加缓存统计信息
	stats["cache_stats"] = m.cache.GetStats()

	// 更新缓存
	m.mu.Lock()
	m.statsCache = stats
	m.mu.Unlock()

	return stats
}

// GetChecker 获取域名检查器
func (m *Monitor) GetChecker() *DomainChecker {
	return m.checker
}

// ClearCache 清除缓存
func (m *Monitor) ClearCache() {
	m.cache.Clear()
	log.Println("域名查询缓存已清除")
}

// GetCacheStats 获取缓存统计信息
func (m *Monitor) GetCacheStats() map[string]interface{} {
	stats := m.cache.GetStats()

	// 添加智能缓存策略说明
	stats["smart_cache_policy"] = map[string]interface{}{
		"pending_delete":          "5分钟（最高优先级）",
		"redemption":              "1小时",
		"available":               "30分钟",
		"expired":                 "6小时",
		"registered_0_3_days":     "6小时（距离过期≤3天）",
		"registered_4_30_days":    "12小时（距离过期4-30天）",
		"registered_30_plus_days": "24小时（距离过期>30天）",
		"error":                   "10分钟",
		"other":                   "2小时",
	}

	return stats
}
