package notification

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"Puff/config"
)

// EmailNotifier 邮件通知器
type EmailNotifier struct {
	config  config.SMTPConfig
	enabled bool
}

// NewEmailNotifier 创建邮件通知器
func NewEmailNotifier(cfg config.SMTPConfig) *EmailNotifier {
	return &EmailNotifier{
		config:  cfg,
		enabled: cfg.Enabled,
	}
}

// SendMessage 发送邮件
func (e *EmailNotifier) SendMessage(subject, message string) error {
	if !e.enabled {
		return fmt.Errorf("邮件通知未启用")
	}

	// 验证配置
	if err := e.validateConfig(); err != nil {
		return fmt.Errorf("邮件配置无效: %v", err)
	}

	// 构建邮件内容
	body := e.buildEmailBody(subject, message)

	// 发送邮件
	return e.sendEmail(subject, body)
}

// IsEnabled 检查是否启用
func (e *EmailNotifier) IsEnabled() bool {
	return e.enabled && e.config.Enabled
}

// GetType 获取通知器类型
func (e *EmailNotifier) GetType() string {
	return "email"
}

// Test 测试邮件连接
func (e *EmailNotifier) Test() error {
	if !e.enabled {
		return fmt.Errorf("邮件通知未启用")
	}

	// 验证配置
	if err := e.validateConfig(); err != nil {
		return err
	}

	// 发送测试邮件
	subject := "[域名监控] 测试邮件"
	message := "这是一封测试邮件，用于验证邮件通知功能是否正常工作。\n\n如果您收到这封邮件，说明邮件通知配置正确。"

	return e.SendMessage(subject, message)
}

// validateConfig 验证配置
func (e *EmailNotifier) validateConfig() error {
	if e.config.Host == "" {
		return fmt.Errorf("SMTP服务器地址不能为空")
	}

	if e.config.Port <= 0 || e.config.Port > 65535 {
		return fmt.Errorf("SMTP端口无效: %d", e.config.Port)
	}

	if e.config.User == "" {
		return fmt.Errorf("SMTP用户名不能为空")
	}

	if e.config.Password == "" {
		return fmt.Errorf("SMTP密码不能为空")
	}

	if e.config.From == "" {
		return fmt.Errorf("发件人地址不能为空")
	}

	if e.config.To == "" {
		return fmt.Errorf("收件人地址不能为空")
	}

	return nil
}

// buildEmailBody 构建邮件内容
func (e *EmailNotifier) buildEmailBody(subject, message string) string {
	var body strings.Builder

	// 邮件头
	body.WriteString(fmt.Sprintf("From: %s\r\n", e.config.From))
	body.WriteString(fmt.Sprintf("To: %s\r\n", e.config.To))
	body.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	body.WriteString("MIME-Version: 1.0\r\n")
	body.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	body.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	body.WriteString("\r\n")

	// 邮件正文
	body.WriteString(message)

	return body.String()
}

// sendEmail 发送邮件
func (e *EmailNotifier) sendEmail(subject, body string) error {
	// 构建服务器地址
	addr := fmt.Sprintf("%s:%d", e.config.Host, e.config.Port)

	// 创建认证
	auth := smtp.PlainAuth("", e.config.User, e.config.Password, e.config.Host)

	// 解析收件人地址
	recipients := e.parseRecipients(e.config.To)

	// 判断是否使用TLS
	if e.config.Port == 465 {
		// 使用SSL/TLS连接
		return e.sendWithTLS(addr, auth, recipients, body)
	} else {
		// 使用STARTTLS连接
		return e.sendWithSTARTTLS(addr, auth, recipients, body)
	}
}

// sendWithTLS 使用TLS发送邮件
func (e *EmailNotifier) sendWithTLS(addr string, auth smtp.Auth, recipients []string, body string) error {
	// 创建TLS连接
	tlsConfig := &tls.Config{
		ServerName: e.config.Host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS连接失败: %v", err)
	}
	defer conn.Close()

	// 创建SMTP客户端
	client, err := smtp.NewClient(conn, e.config.Host)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %v", err)
	}
	defer client.Close()

	// 认证
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP认证失败: %v", err)
	}

	// 设置发件人
	if err := client.Mail(e.config.From); err != nil {
		return fmt.Errorf("设置发件人失败: %v", err)
	}

	// 设置收件人
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("设置收件人失败: %v", err)
		}
	}

	// 发送邮件内容
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("获取数据写入器失败: %v", err)
	}

	_, err = w.Write([]byte(body))
	if err != nil {
		return fmt.Errorf("写入邮件内容失败: %v", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("关闭数据写入器失败: %v", err)
	}

	return client.Quit()
}

// sendWithSTARTTLS 使用STARTTLS发送邮件
func (e *EmailNotifier) sendWithSTARTTLS(addr string, auth smtp.Auth, recipients []string, body string) error {
	return smtp.SendMail(addr, auth, e.config.From, recipients, []byte(body))
}

// parseRecipients 解析收件人地址
func (e *EmailNotifier) parseRecipients(to string) []string {
	// 支持多个收件人，用逗号分隔
	recipients := strings.Split(to, ",")

	// 清理空格
	for i, recipient := range recipients {
		recipients[i] = strings.TrimSpace(recipient)
	}

	return recipients
}

// SetEnabled 设置启用状态
func (e *EmailNotifier) SetEnabled(enabled bool) {
	e.enabled = enabled
}

// UpdateConfig 更新配置
func (e *EmailNotifier) UpdateConfig(cfg config.SMTPConfig) {
	e.config = cfg
	e.enabled = cfg.Enabled
}
