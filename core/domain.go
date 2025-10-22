package core

import (
	"fmt"
	"strings"
	"time"

	"Puff/config"
)

// DomainChecker 域名检查器
type DomainChecker struct {
	whoisClient *WhoisClient
	rdapClient  *RDAPClient
	config      *config.Config
}

// NewDomainChecker 创建新的域名检查器
func NewDomainChecker(cfg *config.Config) *DomainChecker {
	return &DomainChecker{
		whoisClient: NewWhoisClient(cfg.Monitor.Timeout),
		rdapClient:  NewRDAPClient(cfg.Monitor.Timeout),
		config:      cfg,
	}
}

// CheckDomain 检查单个域名
func (d *DomainChecker) CheckDomain(domain string) *DomainInfo {
	domain = strings.ToLower(strings.TrimSpace(domain))

	// 获取TLD
	tld := d.extractTLD(domain)
	if tld == "" {
		return &DomainInfo{
			Name:         domain,
			Status:       StatusError,
			ErrorMessage: "无法提取域名TLD",
			LastChecked:  time.Now(),
		}
	}

	// 首先尝试RDAP查询
	if rdapInfo := d.tryRDAPQuery(domain, tld); rdapInfo != nil && rdapInfo.Status != StatusError {
		return rdapInfo
	}

	// RDAP失败，尝试WHOIS查询
	if whoisInfo := d.tryWhoisQuery(domain, tld); whoisInfo != nil {
		return whoisInfo
	}

	// 都失败了
	return &DomainInfo{
		Name:         domain,
		Status:       StatusError,
		ErrorMessage: "WHOIS和RDAP查询都失败",
		LastChecked:  time.Now(),
	}
}

// CheckDomains 批量检查域名
func (d *DomainChecker) CheckDomains(domains []string) []*DomainInfo {
	results := make([]*DomainInfo, len(domains))

	// 创建工作通道
	type job struct {
		index  int
		domain string
	}

	jobs := make(chan job, len(domains))
	results_chan := make(chan struct {
		index int
		info  *DomainInfo
	}, len(domains))

	// 启动工作协程
	workerCount := d.config.Monitor.ConcurrentLimit
	if workerCount > len(domains) {
		workerCount = len(domains)
	}

	for i := 0; i < workerCount; i++ {
		go func() {
			for j := range jobs {
				info := d.CheckDomain(j.domain)
				results_chan <- struct {
					index int
					info  *DomainInfo
				}{j.index, info}
			}
		}()
	}

	// 发送任务
	for i, domain := range domains {
		jobs <- job{i, domain}
	}
	close(jobs)

	// 收集结果
	for i := 0; i < len(domains); i++ {
		result := <-results_chan
		results[result.index] = result.info
	}

	return results
}

// ValidateDomain 验证域名格式
func (dc *DomainChecker) ValidateDomain(domain string) error {
	domain = strings.TrimSpace(domain)

	if domain == "" {
		return fmt.Errorf("域名不能为空")
	}

	// 基本长度检查
	if len(domain) > 253 {
		return fmt.Errorf("域名长度不能超过253个字符")
	}

	// 检查是否包含无效字符
	for _, char := range domain {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '.' || char == '-') {
			return fmt.Errorf("域名包含无效字符: %c", char)
		}
	}

	// 分割域名部分
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return fmt.Errorf("域名必须包含至少一个点")
	}

	// 检查每个部分
	for i, part := range parts {
		if part == "" {
			return fmt.Errorf("域名部分不能为空")
		}

		if len(part) > 63 {
			return fmt.Errorf("域名部分长度不能超过63个字符")
		}

		// 不能以连字符开始或结束
		if strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return fmt.Errorf("域名部分不能以连字符开始或结束: %s", part)
		}

		// 最后一部分（TLD）不能全是数字
		if i == len(parts)-1 {
			allDigits := true
			for _, char := range part {
				if char < '0' || char > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return fmt.Errorf("顶级域名不能全是数字")
			}
		}
	}

	return nil
}

// extractTLD 提取顶级域名
func (d *DomainChecker) extractTLD(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

// tryRDAPQuery 尝试RDAP查询
func (d *DomainChecker) tryRDAPQuery(domain, tld string) *DomainInfo {
	server, exists := config.GetRDAPServerByTLD(tld)
	if !exists {
		return &DomainInfo{
			Name:         domain,
			Status:       StatusError,
			ErrorMessage: fmt.Sprintf("不支持的TLD: %s (RDAP)", tld),
			LastChecked:  time.Now(),
		}
	}

	rdapResp, err := d.rdapClient.QueryRDAP(domain, server.Server)
	if err != nil {
		return &DomainInfo{
			Name:         domain,
			Status:       StatusError,
			ErrorMessage: fmt.Sprintf("RDAP查询失败: %v", err),
			LastChecked:  time.Now(),
		}
	}

	return d.rdapClient.ParseRDAPResponse(domain, rdapResp)
}

// tryWhoisQuery 尝试WHOIS查询
func (d *DomainChecker) tryWhoisQuery(domain, tld string) *DomainInfo {
	server, exists := config.GetWhoisServerByTLD(tld)
	if !exists {
		return &DomainInfo{
			Name:         domain,
			Status:       StatusError,
			ErrorMessage: fmt.Sprintf("不支持的TLD: %s (WHOIS)", tld),
			LastChecked:  time.Now(),
		}
	}

	response, err := d.whoisClient.QueryWhois(domain, server.Server, server.Port)
	if err != nil {
		return &DomainInfo{
			Name:         domain,
			Status:       StatusError,
			ErrorMessage: fmt.Sprintf("WHOIS查询失败: %v", err),
			LastChecked:  time.Now(),
		}
	}

	return d.whoisClient.ParseWhoisResponse(domain, response)
}

// GetSupportedTLDs 获取支持的TLD列表
func (d *DomainChecker) GetSupportedTLDs() []string {
	return config.GetSupportedTLDs()
}
