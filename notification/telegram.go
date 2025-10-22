package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"Puff/config"
)

// TelegramNotifier Telegram通知器
type TelegramNotifier struct {
	config     config.TelegramConfig
	httpClient *http.Client
	enabled    bool
}

// TelegramMessage Telegram消息结构
type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// TelegramResponse Telegram API响应
type TelegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	ErrorCode   int    `json:"error_code,omitempty"`
}

// NewTelegramNotifier 创建Telegram通知器
func NewTelegramNotifier(cfg config.TelegramConfig) *TelegramNotifier {
	return &TelegramNotifier{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		enabled: cfg.Enabled,
	}
}

// SendMessage 发送Telegram消息
func (t *TelegramNotifier) SendMessage(subject, message string) error {
	if !t.enabled {
		return fmt.Errorf("Telegram通知未启用")
	}

	// 验证配置
	if err := t.validateConfig(); err != nil {
		return fmt.Errorf("Telegram配置无效: %v", err)
	}

	// 格式化消息
	formattedMessage := t.formatMessage(subject, message)

	// 发送消息
	return t.sendToTelegram(formattedMessage)
}

// IsEnabled 检查是否启用
func (t *TelegramNotifier) IsEnabled() bool {
	return t.enabled && t.config.Enabled
}

// GetType 获取通知器类型
func (t *TelegramNotifier) GetType() string {
	return "telegram"
}

// Test 测试Telegram连接
func (t *TelegramNotifier) Test() error {
	if !t.enabled {
		return fmt.Errorf("Telegram通知未启用")
	}

	// 验证配置
	if err := t.validateConfig(); err != nil {
		return err
	}

	// 发送测试消息
	testMessage := `🧪 *域名监控系统测试*

这是一条测试消息，用于验证Telegram通知功能是否正常工作。

时间: ` + time.Now().Format("2006-01-02 15:04:05") + `

如果您收到这条消息，说明Telegram通知配置正确。`

	return t.sendToTelegram(testMessage)
}

// validateConfig 验证配置
func (t *TelegramNotifier) validateConfig() error {
	if t.config.BotToken == "" {
		return fmt.Errorf("Telegram Bot Token不能为空")
	}

	if t.config.ChatID == "" {
		return fmt.Errorf("Telegram Chat ID不能为空")
	}

	// 验证Chat ID格式
	if _, err := strconv.ParseInt(t.config.ChatID, 10, 64); err != nil {
		// Chat ID可能是用户名格式(@username)
		if !strings.HasPrefix(t.config.ChatID, "@") {
			return fmt.Errorf("Telegram Chat ID格式无效: %s", t.config.ChatID)
		}
	}

	return nil
}

// formatMessage 格式化Telegram消息
func (t *TelegramNotifier) formatMessage(subject, message string) string {
	var formatted strings.Builder

	// 使用Markdown格式
	formatted.WriteString(fmt.Sprintf("*%s*\n\n", t.escapeMarkdown(subject)))

	// 处理消息内容
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			formatted.WriteString("\n")
			continue
		}

		// 特殊格式处理
		if strings.Contains(line, "域名:") {
			formatted.WriteString(fmt.Sprintf("*%s*\n", t.escapeMarkdown(line)))
		} else if strings.Contains(line, "时间:") {
			formatted.WriteString(fmt.Sprintf("时间: %s\n", t.escapeMarkdown(line)))
		} else if strings.Contains(line, "状态:") {
			formatted.WriteString(fmt.Sprintf("状态: %s\n", t.escapeMarkdown(line)))
		} else if strings.Contains(line, "状态变化:") {
			formatted.WriteString(fmt.Sprintf("变化: *%s*\n", t.escapeMarkdown(line)))
		} else if strings.Contains(line, "错误信息:") {
			formatted.WriteString(fmt.Sprintf("错误: `%s`\n", t.escapeMarkdown(line)))
		} else if strings.HasPrefix(line, "---") {
			formatted.WriteString("━━━━━━━━━━━━━━━━━━━━\n")
		} else {
			formatted.WriteString(fmt.Sprintf("%s\n", t.escapeMarkdown(line)))
		}
	}

	return formatted.String()
}

// escapeMarkdown 转义Markdown特殊字符
func (t *TelegramNotifier) escapeMarkdown(text string) string {
	// Telegram Markdown V2需要转义的字符
	specialChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}

	for _, char := range specialChars {
		text = strings.ReplaceAll(text, char, "\\"+char)
	}

	return text
}

// sendToTelegram 发送消息到Telegram
func (t *TelegramNotifier) sendToTelegram(message string) error {
	// 构建API URL
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.config.BotToken)

	// 构建消息
	telegramMsg := TelegramMessage{
		ChatID:    t.config.ChatID,
		Text:      message,
		ParseMode: "MarkdownV2",
	}

	// 编码为JSON
	jsonData, err := json.Marshal(telegramMsg)
	if err != nil {
		return fmt.Errorf("编码JSON失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var telegramResp TelegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	if !telegramResp.OK {
		return fmt.Errorf("Telegram API错误 [%d]: %s", telegramResp.ErrorCode, telegramResp.Description)
	}

	return nil
}

// GetBotInfo 获取Bot信息
func (t *TelegramNotifier) GetBotInfo() (map[string]interface{}, error) {
	if !t.enabled {
		return nil, fmt.Errorf("Telegram通知未启用")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", t.config.BotToken)

	resp, err := t.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("获取Bot信息失败: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析Bot信息失败: %v", err)
	}

	return result, nil
}

// GetChatInfo 获取聊天信息
func (t *TelegramNotifier) GetChatInfo() (map[string]interface{}, error) {
	if !t.enabled {
		return nil, fmt.Errorf("Telegram通知未启用")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getChat?chat_id=%s", t.config.BotToken, t.config.ChatID)

	resp, err := t.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("获取聊天信息失败: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析聊天信息失败: %v", err)
	}

	return result, nil
}

// SetEnabled 设置启用状态
func (t *TelegramNotifier) SetEnabled(enabled bool) {
	t.enabled = enabled
}

// UpdateConfig 更新配置
func (t *TelegramNotifier) UpdateConfig(cfg config.TelegramConfig) {
	t.config = cfg
	t.enabled = cfg.Enabled
}
