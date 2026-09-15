package email

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// SMTPConfig configures application email only. TLSMode is starttls (default),
// tls (implicit TLS), or explicit none for a trusted local relay/test fixture.
type SMTPConfig struct {
	Host                              string
	Port                              int
	Username, Password, From, TLSMode string
}

// SMTPClient delivers the existing application templates over SMTP.
type SMTPClient struct {
	config SMTPConfig
	from   *mail.Address
}

func NewSMTPClient(config SMTPConfig) (*SMTPClient, error) {
	if config.Host == "" || strings.ContainsAny(config.Host, "/\r\n ") {
		return nil, fmt.Errorf("SMTP_HOST must be a hostname")
	}
	if config.Port == 0 {
		config.Port = 587
	}
	if config.Port < 1 || config.Port > 65535 {
		return nil, fmt.Errorf("SMTP_PORT is invalid")
	}
	if config.TLSMode == "" {
		config.TLSMode = "starttls"
	}
	if config.TLSMode != "starttls" && config.TLSMode != "tls" && config.TLSMode != "none" {
		return nil, fmt.Errorf("SMTP_TLS_MODE must be starttls, tls or none")
	}
	if (config.Username == "") != (config.Password == "") {
		return nil, fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must both be set or empty")
	}
	if strings.ContainsAny(config.From, "\r\n") {
		return nil, fmt.Errorf("SMTP_FROM is invalid")
	}
	from, err := mail.ParseAddress(config.From)
	if err != nil {
		return nil, fmt.Errorf("SMTP_FROM must be a valid sender address")
	}
	return &SMTPClient{config: config, from: from}, nil
}

func (c *SMTPClient) FromEmail() string { return c.config.From }

func (c *SMTPClient) SendEmail(to, subject, htmlBody, textBody string) error {
	if strings.ContainsAny(to+subject, "\r\n") {
		return fmt.Errorf("invalid email header")
	}
	recipient, err := mail.ParseAddress(to)
	if err != nil {
		return fmt.Errorf("invalid email recipient")
	}
	message, err := smtpMessage(c.from.String(), recipient.String(), subject, htmlBody, textBody)
	if err != nil {
		return err
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	address := net.JoinHostPort(c.config.Host, strconv.Itoa(c.config.Port))
	tlsConfig := &tls.Config{ServerName: c.config.Host, MinVersion: tls.VersionTLS12}
	var conn net.Conn
	if c.config.TLSMode == "tls" {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	} else {
		conn, err = dialer.Dial("tcp", address)
	}
	if err != nil {
		return fmt.Errorf("connect SMTP: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return fmt.Errorf("set SMTP deadline: %w", err)
	}
	client, err := smtp.NewClient(conn, c.config.Host)
	if err != nil {
		return fmt.Errorf("SMTP greeting: %w", err)
	}
	defer client.Close()
	if c.config.TLSMode == "starttls" {
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("SMTP STARTTLS: %w", err)
		}
	}
	if c.config.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.Host)); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}
	if err := client.Mail(c.from.Address); err != nil {
		return fmt.Errorf("SMTP sender rejected: %w", err)
	}
	if err := client.Rcpt(recipient.Address); err != nil {
		return fmt.Errorf("SMTP recipient rejected: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("SMTP delivery rejected: %w", err)
	}
	return client.Quit()
}

func smtpMessage(from, to, subject, htmlBody, textBody string) ([]byte, error) {
	var body bytes.Buffer
	multipartWriter := multipart.NewWriter(&body)
	for _, part := range []struct{ kind, content string }{{"text/plain", textBody}, {"text/html", htmlBody}} {
		header := textproto.MIMEHeader{"Content-Type": {part.kind + "; charset=utf-8"}, "Content-Transfer-Encoding": {"quoted-printable"}}
		writer, err := multipartWriter.CreatePart(header)
		if err != nil {
			return nil, err
		}
		encoded := quotedprintable.NewWriter(writer)
		if _, err := io.WriteString(encoded, part.content); err != nil {
			return nil, err
		}
		if err := encoded.Close(); err != nil {
			return nil, err
		}
	}
	if err := multipartWriter.Close(); err != nil {
		return nil, err
	}
	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n", from, to, mime.QEncoding.Encode("utf-8", subject), time.Now().Format(time.RFC1123Z), multipartWriter.Boundary())
	message.Write(body.Bytes())
	return message.Bytes(), nil
}
