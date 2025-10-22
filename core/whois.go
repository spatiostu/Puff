package core

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// WhoisClient WHOIS查询客户端
type WhoisClient struct {
	timeout time.Duration
}

// NewWhoisClient 创建新的WHOIS客户端
func NewWhoisClient(timeout time.Duration) *WhoisClient {
	return &WhoisClient{
		timeout: timeout,
	}
}

// QueryWhois 执行WHOIS查询
func (w *WhoisClient) QueryWhois(domain, server string, port int) (string, error) {
	address := fmt.Sprintf("%s:%d", server, port)

	// 建立连接
	conn, err := net.DialTimeout("tcp", address, w.timeout)
	if err != nil {
		return "", fmt.Errorf("连接WHOIS服务器失败: %v", err)
	}
	defer conn.Close()

	// 设置读写超时
	conn.SetDeadline(time.Now().Add(w.timeout))

	// 发送查询请求
	query := domain + "\r\n"
	_, err = conn.Write([]byte(query))
	if err != nil {
		return "", fmt.Errorf("发送查询请求失败: %v", err)
	}

	// 读取响应
	response := make([]byte, 0, 4096)
	buffer := make([]byte, 1024)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if n == 0 {
				break // 连接关闭
			}
			return "", fmt.Errorf("读取响应失败: %v", err)
		}
		response = append(response, buffer[:n]...)

		// 防止响应过大
		if len(response) > 100*1024 { // 100KB限制
			break
		}
	}

	return string(response), nil
}

// ParseWhoisResponse 解析WHOIS响应
func (w *WhoisClient) ParseWhoisResponse(domain, response string) *DomainInfo {
	info := &DomainInfo{
		Name:        domain,
		LastChecked: time.Now(),
		QueryMethod: "whois",
	}

	// 转换为小写便于匹配
	lowerResponse := strings.ToLower(response)

	// 检查域名状态
	info.Status = w.parseStatus(lowerResponse)

	// 解析注册商
	info.Registrar = w.parseRegistrar(response)

	// 解析日期
	info.CreatedDate = w.parseDate(response, []string{"creation date", "created", "registered"})
	info.ExpiryDate = w.parseDate(response, []string{"expiry date", "expires", "expiration date", "registry expiry date"})
	info.UpdatedDate = w.parseDate(response, []string{"updated date", "last updated", "modified"})

	// 解析名称服务器
	info.NameServers = w.parseNameServers(response)

	return info
}

// parseStatus 解析域名状态
func (w *WhoisClient) parseStatus(response string) DomainStatus {
	// 检查是否可注册（常见的"未找到"消息）
	availablePatterns := []string{
		"no matching record",
		"not found",
		"no data found",
		"not exist",
		"available",
		"no entries found",
		"status: free",
		"状态: 未注册",
		"未注册",
		"not registered",
	}

	for _, pattern := range availablePatterns {
		if strings.Contains(response, pattern) {
			return StatusAvailable
		}
	}

	// 检查赎回期状态
	redemptionPatterns := []string{
		"redemption",
		"pending restore",
		"redemptionperiod",
		"pending delete restorable",
	}

	for _, pattern := range redemptionPatterns {
		if strings.Contains(response, pattern) {
			return StatusRedemption
		}
	}

	// 检查待删除状态
	pendingDeletePatterns := []string{
		"pending delete",
		"pendingdelete",
		"to be released",
	}

	for _, pattern := range pendingDeletePatterns {
		if strings.Contains(response, pattern) {
			return StatusPendingDelete
		}
	}

	// 检查过期状态
	expiredPatterns := []string{
		"expired",
		"expiry date",
		"expires:",
	}

	for _, pattern := range expiredPatterns {
		if strings.Contains(response, pattern) {
			// 进一步检查是否真的过期
			if w.isExpired(response) {
				return StatusExpired
			}
		}
	}

	// 检查Hold状态
	holdPatterns := []string{
		"client hold",
		"server hold",
		"registrar hold",
		"registry hold",
	}

	for _, pattern := range holdPatterns {
		if strings.Contains(response, pattern) {
			return StatusHold
		}
	}

	// 检查转移锁定
	transferLockPatterns := []string{
		"transfer prohibited",
		"clienttransferprohibited",
		"servertransferprohibited",
	}

	for _, pattern := range transferLockPatterns {
		if strings.Contains(response, pattern) {
			return StatusTransferLocked
		}
	}

	// 如果包含注册信息，则认为已注册
	registeredPatterns := []string{
		"registrar:",
		"registrant:",
		"domain status:",
		"status:",
		"creation date:",
		"expiry date:",
		"name server",
	}

	for _, pattern := range registeredPatterns {
		if strings.Contains(response, pattern) {
			return StatusRegistered
		}
	}

	return StatusUnknown
}

// parseRegistrar 解析注册商
func (w *WhoisClient) parseRegistrar(response string) string {
	patterns := []string{
		`(?i)registrar:\s*(.+)`,
		`(?i)registrar organization:\s*(.+)`,
		`(?i)sponsoring registrar:\s*(.+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(response)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// parseDate 解析日期
func (w *WhoisClient) parseDate(response string, keywords []string) *time.Time {
	for _, keyword := range keywords {
		pattern := fmt.Sprintf(`(?i)%s:\s*([^\r\n]+)`, regexp.QuoteMeta(keyword))
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(response)

		if len(matches) > 1 {
			dateStr := strings.TrimSpace(matches[1])
			if date := w.parseDateTime(dateStr); date != nil {
				return date
			}
		}
	}

	return nil
}

// parseDateTime 解析日期时间字符串
func (w *WhoisClient) parseDateTime(dateStr string) *time.Time {
	// 常见的日期格式
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02",
		"02-Jan-2006",
		"January 02 2006",
		"Jan 02 2006",
		"2006/01/02",
		"02/01/2006",
		"01/02/2006",
	}

	// 清理日期字符串
	dateStr = strings.TrimSpace(dateStr)
	dateStr = regexp.MustCompile(`\s+`).ReplaceAllString(dateStr, " ")

	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return &date
		}
	}

	return nil
}

// parseNameServers 解析名称服务器
func (w *WhoisClient) parseNameServers(response string) []string {
	patterns := []string{
		`(?i)name server:\s*([^\r\n]+)`,
		`(?i)nameserver:\s*([^\r\n]+)`,
		`(?i)nserver:\s*([^\r\n]+)`,
		`(?i)dns:\s*([^\r\n]+)`,
	}

	var nameServers []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(response, -1)

		for _, match := range matches {
			if len(match) > 1 {
				ns := strings.TrimSpace(strings.ToLower(match[1]))
				if ns != "" && !seen[ns] {
					nameServers = append(nameServers, ns)
					seen[ns] = true
				}
			}
		}
	}

	return nameServers
}

// isExpired 检查域名是否已过期
func (w *WhoisClient) isExpired(response string) bool {
	// 查找过期日期
	expiryPatterns := []string{
		`(?i)expiry date:\s*([^\r\n]+)`,
		`(?i)expires:\s*([^\r\n]+)`,
		`(?i)expiration date:\s*([^\r\n]+)`,
		`(?i)registry expiry date:\s*([^\r\n]+)`,
	}

	for _, pattern := range expiryPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(response)

		if len(matches) > 1 {
			dateStr := strings.TrimSpace(matches[1])
			if date := w.parseDateTime(dateStr); date != nil {
				// 检查是否已过期
				return date.Before(time.Now())
			}
		}
	}

	return false
}
