package core

import (
	"fmt"
	"time"
)

// DomainStatus 域名状态枚举
type DomainStatus string

const (
	// StatusAvailable 域名可注册
	StatusAvailable DomainStatus = "available"

	// StatusRegistered 域名已注册
	StatusRegistered DomainStatus = "registered"

	// StatusRedemption 域名在赎回期
	StatusRedemption DomainStatus = "redemption"

	// StatusPendingDelete 域名待删除/抢注期
	StatusPendingDelete DomainStatus = "pending_delete"

	// StatusExpired 域名已过期
	StatusExpired DomainStatus = "expired"

	// StatusTransferLocked 域名转移锁定
	StatusTransferLocked DomainStatus = "transfer_locked"

	// StatusHold 域名被Hold
	StatusHold DomainStatus = "hold"

	// StatusUnknown 未知状态
	StatusUnknown DomainStatus = "unknown"

	// StatusError 查询错误
	StatusError DomainStatus = "error"
)

// DomainInfo 域名信息结构
type DomainInfo struct {
	Name         string       `json:"name"`          // 域名名称
	Status       DomainStatus `json:"status"`        // 域名状态
	Registrar    string       `json:"registrar"`     // 注册商
	CreatedDate  *time.Time   `json:"created_date"`  // 创建日期
	ExpiryDate   *time.Time   `json:"expiry_date"`   // 过期日期
	UpdatedDate  *time.Time   `json:"updated_date"`  // 更新日期
	NameServers  []string     `json:"name_servers"`  // 名称服务器
	LastChecked  time.Time    `json:"last_checked"`  // 最后检查时间
	QueryMethod  string       `json:"query_method"`  // 查询方法 (whois/rdap)
	ErrorMessage string       `json:"error_message"` // 错误信息
}

// StatusInfo 状态信息定义
type StatusInfo struct {
	Status       DomainStatus `json:"status"`
	Description  string       `json:"description"`
	Color        string       `json:"color"`         // 前端显示颜色
	Priority     int          `json:"priority"`      // 优先级(用于排序)
	ShouldNotify bool         `json:"should_notify"` // 是否发送通知
}

// GetAllStatusInfo 获取所有状态信息
func GetAllStatusInfo() map[DomainStatus]StatusInfo {
	return map[DomainStatus]StatusInfo{
		StatusAvailable: {
			Status:       StatusAvailable,
			Description:  "域名可注册",
			Color:        "#28a745", // 绿色
			Priority:     1,
			ShouldNotify: true,
		},
		StatusRedemption: {
			Status:       StatusRedemption,
			Description:  "域名在赎回期",
			Color:        "#fd7e14", // 橙色
			Priority:     2,
			ShouldNotify: true,
		},
		StatusPendingDelete: {
			Status:       StatusPendingDelete,
			Description:  "域名待删除/抢注期",
			Color:        "#dc3545", // 红色
			Priority:     3,
			ShouldNotify: true,
		},
		StatusExpired: {
			Status:       StatusExpired,
			Description:  "域名已过期",
			Color:        "#ffc107", // 黄色
			Priority:     4,
			ShouldNotify: true,
		},
		StatusRegistered: {
			Status:       StatusRegistered,
			Description:  "域名已注册",
			Color:        "#6c757d", // 灰色
			Priority:     5,
			ShouldNotify: false,
		},
		StatusTransferLocked: {
			Status:       StatusTransferLocked,
			Description:  "域名转移锁定",
			Color:        "#17a2b8", // 青色
			Priority:     6,
			ShouldNotify: false,
		},
		StatusHold: {
			Status:       StatusHold,
			Description:  "域名被Hold",
			Color:        "#6f42c1", // 紫色
			Priority:     7,
			ShouldNotify: false,
		},
		StatusUnknown: {
			Status:       StatusUnknown,
			Description:  "未知状态",
			Color:        "#343a40", // 深灰色
			Priority:     8,
			ShouldNotify: false,
		},
		StatusError: {
			Status:       StatusError,
			Description:  "查询错误",
			Color:        "#dc3545", // 红色
			Priority:     9,
			ShouldNotify: false,
		},
	}
}

// GetStatusInfo 获取指定状态的信息
func GetStatusInfo(status DomainStatus) StatusInfo {
	statusMap := GetAllStatusInfo()
	if info, exists := statusMap[status]; exists {
		return info
	}
	return statusMap[StatusUnknown]
}

// ShouldNotify 判断状态是否需要发送通知
func (d *DomainInfo) ShouldNotify() bool {
	return GetStatusInfo(d.Status).ShouldNotify
}

// GetDisplayColor 获取状态显示颜色
func (d *DomainInfo) GetDisplayColor() string {
	return GetStatusInfo(d.Status).Color
}

// GetStatusDescription 获取状态描述
func (d *DomainInfo) GetStatusDescription() string {
	return GetStatusInfo(d.Status).Description
}

// IsImportant 判断是否为重要状态（需要特别关注）
func (d *DomainInfo) IsImportant() bool {
	importantStatuses := []DomainStatus{
		StatusAvailable,
		StatusRedemption,
		StatusPendingDelete,
		StatusExpired,
	}

	for _, status := range importantStatuses {
		if d.Status == status {
			return true
		}
	}
	return false
}

// StatusChangeEvent 状态变化事件
type StatusChangeEvent struct {
	Domain    string       `json:"domain"`
	OldStatus DomainStatus `json:"old_status"`
	NewStatus DomainStatus `json:"new_status"`
	Timestamp time.Time    `json:"timestamp"`
	Message   string       `json:"message"`
}

// GetStatusChangeMessage 获取状态变化消息
func GetStatusChangeMessage(domain string, oldStatus, newStatus DomainStatus) string {
	oldInfo := GetStatusInfo(oldStatus)
	newInfo := GetStatusInfo(newStatus)

	return fmt.Sprintf("域名 %s 状态发生变化：从 [%s] 变为 [%s]",
		domain, oldInfo.Description, newInfo.Description)
}

// GetSmartCacheDuration 根据域名状态和过期时间计算智能缓存时间
func (d *DomainInfo) GetSmartCacheDuration() time.Duration {
	// 如果查询失败，使用较短的缓存时间
	if d.Status == StatusError {
		return 10 * time.Minute
	}

	// 根据状态确定缓存策略
	switch d.Status {
	case StatusPendingDelete:
		// 待删除状态：按照检查间隔查询（最高频率）
		return 5 * time.Minute

	case StatusRedemption:
		// 赎回期：1小时查一次
		return 1 * time.Hour

	case StatusAvailable:
		// 可注册状态：频繁检查，因为可能被注册
		return 30 * time.Minute

	case StatusExpired:
		// 已过期：6小时查一次
		return 6 * time.Hour

	case StatusRegistered:
		// 已注册状态：根据过期时间智能调整
		return d.getCacheDurationByExpiryDate()

	default:
		// 其他状态：使用默认缓存时间
		return 2 * time.Hour
	}
}

// getCacheDurationByExpiryDate 根据过期时间计算缓存时间
func (d *DomainInfo) getCacheDurationByExpiryDate() time.Duration {
	// 如果没有过期时间信息，使用默认值
	if d.ExpiryDate == nil {
		return 24 * time.Hour
	}

	now := time.Now()
	daysUntilExpiry := d.ExpiryDate.Sub(now).Hours() / 24

	if daysUntilExpiry <= 0 {
		// 已过期，应该使用过期状态的缓存策略
		return 6 * time.Hour
	} else if daysUntilExpiry <= 3 {
		// 距离过期小于3天：6小时查一次
		return 6 * time.Hour
	} else if daysUntilExpiry <= 30 {
		// 距离过期30天内：12小时查一次
		return 12 * time.Hour
	} else {
		// 距离过期大于30天：24小时查一次
		return 24 * time.Hour
	}
}

// GetCacheKey 获取缓存键
func (d *DomainInfo) GetCacheKey() string {
	return fmt.Sprintf("domain:%s", d.Name)
}
