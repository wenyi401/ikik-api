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

	client, err := s.connectSMTP(config)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err = client.Mail(sanitizeEmailHeader(config.From)); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err = w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("write msg: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("close writer: %w", err)
	}
	_ = client.Quit()
	return nil
}
