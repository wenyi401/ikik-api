package service

import (
	"fmt"

	"net/smtp"
)

// SendEmailWithConfigAndContentType 允许调用方指定 MIME Content-Type，
// 例如 "text/plain; charset=UTF-8" 用于纯文本邮件。
//
// SendEmailWithConfigAndContentType lets callers pick the MIME content type
// (e.g. "text/plain; charset=UTF-8" for plain-text mails).
func (s *EmailService) SendEmailWithConfigAndContentType(config *SMTPConfig, to, subject, body, contentType string) error {

	to = sanitizeEmailHeader(to)
	subject = sanitizeEmailHeader(subject)
	contentType = sanitizeEmailHeader(contentType)
	if contentType == "" {
		contentType = "text/html; charset=UTF-8"
	}

	from := sanitizeEmailHeader(config.From)
	if config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", sanitizeEmailHeader(config.FromName), sanitizeEmailHeader(config.From))
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: %s\r\n\r\n%s",
		from, to, subject, contentType, body)

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if config.UseTLS {
		return s.sendMailTLS(addr, auth, config.From, to, []byte(msg), config.Host)
	}

	return s.sendMailPlain(addr, auth, config.From, to, []byte(msg), config.Host)
}
