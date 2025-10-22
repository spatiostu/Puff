package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RDAPClient RDAP查询客户端
type RDAPClient struct {
	httpClient *http.Client
}

// NewRDAPClient 创建新的RDAP客户端
func NewRDAPClient(timeout time.Duration) *RDAPClient {
	return &RDAPClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// RDAPResponse RDAP响应结构
type RDAPResponse struct {
	ObjectClassName string           `json:"objectClassName"`
	Handle          string           `json:"handle"`
	LDHName         string           `json:"ldhName"`
	Status          []string         `json:"status"`
	Entities        []RDAPEntity     `json:"entities"`
	Events          []RDAPEvent      `json:"events"`
	NameServers     []RDAPNameServer `json:"nameservers"`
	ErrorCode       int              `json:"errorCode,omitempty"`
	Title           string           `json:"title,omitempty"`
	Description     []string         `json:"description,omitempty"`
}

// RDAPEntity RDAP实体结构
type RDAPEntity struct {
	ObjectClassName string        `json:"objectClassName"`
	Handle          string        `json:"handle"`
	Roles           []string      `json:"roles"`
	VCardArray      []interface{} `json:"vcardArray,omitempty"`
}

// RDAPEvent RDAP事件结构
type RDAPEvent struct {
	EventAction string    `json:"eventAction"`
	EventDate   time.Time `json:"eventDate"`
}

// RDAPNameServer RDAP名称服务器结构
type RDAPNameServer struct {
	ObjectClassName string `json:"objectClassName"`
	LDHName         string `json:"ldhName"`
	IPAddresses     struct {
		V4 []string `json:"v4,omitempty"`
		V6 []string `json:"v6,omitempty"`
	} `json:"ipAddresses,omitempty"`
}

// QueryRDAP 执行RDAP查询
func (r *RDAPClient) QueryRDAP(domain, serverURL string) (*RDAPResponse, error) {
	// 构建查询URL
	url := strings.TrimSuffix(serverURL, "/") + "/domain/" + domain

	// 创建HTTP请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Accept", "application/rdap+json")
	req.Header.Set("User-Agent", "Puff-Domain-Monitor/1.0")

	// 执行请求
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("执行HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %v", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode == 404 {
		// 404通常表示域名不存在
		return &RDAPResponse{
			ErrorCode:   404,
			Title:       "Not Found",
			Description: []string{"Domain not found"},
		}, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 解析JSON响应
	var rdapResp RDAPResponse
	if err := json.Unmarshal(body, &rdapResp); err != nil {
		return nil, fmt.Errorf("解析JSON响应失败: %v", err)
	}

	return &rdapResp, nil
}

// ParseRDAPResponse 解析RDAP响应
func (r *RDAPClient) ParseRDAPResponse(domain string, rdapResp *RDAPResponse) *DomainInfo {
	info := &DomainInfo{
		Name:        domain,
		LastChecked: time.Now(),
		QueryMethod: "rdap",
	}

	// 检查是否为错误响应
	if rdapResp.ErrorCode == 404 {
		info.Status = StatusAvailable
		return info
	}

	// 解析域名状态
	info.Status = r.parseRDAPStatus(rdapResp.Status)

	// 解析注册商
	info.Registrar = r.parseRDAPRegistrar(rdapResp.Entities)

	// 解析事件日期
	r.parseRDAPEvents(rdapResp.Events, info)

	// 解析名称服务器
	info.NameServers = r.parseRDAPNameServers(rdapResp.NameServers)

	return info
}

// parseRDAPStatus 解析RDAP状态
func (r *RDAPClient) parseRDAPStatus(statuses []string) DomainStatus {
	if len(statuses) == 0 {
		return StatusUnknown
	}

	// 将状态转换为小写进行匹配
	statusMap := make(map[string]bool)
	for _, status := range statuses {
		statusMap[strings.ToLower(status)] = true
	}

	// 检查重要状态（优先级从高到低）
	if statusMap["redemption period"] || statusMap["redemptionperiod"] {
		return StatusRedemption
	}

	if statusMap["pending delete"] || statusMap["pendingdelete"] {
		return StatusPendingDelete
	}

	if statusMap["expired"] {
		return StatusExpired
	}

	if statusMap["client hold"] || statusMap["server hold"] ||
		statusMap["registrar hold"] || statusMap["registry hold"] {
		return StatusHold
	}

	if statusMap["client transfer prohibited"] || statusMap["server transfer prohibited"] ||
		statusMap["clienttransferprohibited"] || statusMap["servertransferprohibited"] {
		return StatusTransferLocked
	}

	// 检查活跃状态
	if statusMap["ok"] || statusMap["active"] || statusMap["client update prohibited"] ||
		statusMap["server update prohibited"] || statusMap["client delete prohibited"] ||
		statusMap["server delete prohibited"] {
		return StatusRegistered
	}

	return StatusRegistered // 默认为已注册
}

// parseRDAPRegistrar 解析注册商信息
func (r *RDAPClient) parseRDAPRegistrar(entities []RDAPEntity) string {
	for _, entity := range entities {
		// 寻找registrar角色
		for _, role := range entity.Roles {
			if strings.ToLower(role) == "registrar" {
				// 尝试从vCard中提取组织名称
				if orgName := r.extractOrgFromVCard(entity.VCardArray); orgName != "" {
					return orgName
				}
				// 如果没有vCard信息，返回handle
				return entity.Handle
			}
		}
	}
	return ""
}

// extractOrgFromVCard 从vCard中提取组织名称
func (r *RDAPClient) extractOrgFromVCard(vcard []interface{}) string {
	if len(vcard) < 2 {
		return ""
	}

	// vCard数组的第二个元素包含属性
	if properties, ok := vcard[1].([]interface{}); ok {
		for _, prop := range properties {
			if propArray, ok := prop.([]interface{}); ok && len(propArray) >= 4 {
				// 检查是否为组织属性
				if propName, ok := propArray[0].(string); ok && strings.ToLower(propName) == "org" {
					if propValue, ok := propArray[3].(string); ok {
						return propValue
					}
				}
				// 检查fn（全名）属性
				if propName, ok := propArray[0].(string); ok && strings.ToLower(propName) == "fn" {
					if propValue, ok := propArray[3].(string); ok {
						return propValue
					}
				}
			}
		}
	}

	return ""
}

// parseRDAPEvents 解析RDAP事件
func (r *RDAPClient) parseRDAPEvents(events []RDAPEvent, info *DomainInfo) {
	for _, event := range events {
		switch strings.ToLower(event.EventAction) {
		case "registration":
			info.CreatedDate = &event.EventDate
		case "expiration":
			info.ExpiryDate = &event.EventDate
		case "last changed", "last update of rdap database":
			info.UpdatedDate = &event.EventDate
		}
	}
}

// parseRDAPNameServers 解析RDAP名称服务器
func (r *RDAPClient) parseRDAPNameServers(nameservers []RDAPNameServer) []string {
	var result []string
	seen := make(map[string]bool)

	for _, ns := range nameservers {
		if ns.LDHName != "" {
			lowerName := strings.ToLower(ns.LDHName)
			if !seen[lowerName] {
				result = append(result, lowerName)
				seen[lowerName] = true
			}
		}
	}

	return result
}
