// Package mail sends the server's few transactional e-mails (invites,
// notifications) over plain SMTP — the stdlib client, STARTTLS when the
// server offers it, PLAIN auth when configured. Not configured = a no-op
// sender that logs what it would have sent.
package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/gopherex/xlog"
)

// Config is the `mail` section. Empty host = disabled.
type Config struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port" schemapb:"default=587;gte=0;lte=65535"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty" schemapb:"secret=true"`
	// From is the sender address, e.g. "Stroppy Cloud <no-reply@stroppy.io>".
	From string `json:"from" schemapb:"default=Stroppy Cloud <no-reply@stroppy.io>"`
	// TLS: "starttls" (default), "tls" (implicit), "none".
	TLS     string        `json:"tls" schemapb:"default=starttls;in=[\"starttls\",\"tls\",\"none\"]"`
	Timeout time.Duration `json:"timeout" schemapb:"default=10s"`
}

// Enabled reports a configured relay.
func (c *Config) Enabled() bool { return c.Host != "" }

// Message is one e-mail.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Sender delivers messages.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// New builds the sender for the config.
func New(cfg *Config, log *xlog.Logger) Sender {
	if !cfg.Enabled() {
		return nopSender{log: log}
	}
	return &smtpSender{cfg: cfg}
}

type nopSender struct{ log *xlog.Logger }

func (n nopSender) Send(ctx context.Context, m Message) error {
	n.log.Ctx().Info(ctx, "mail: relay not configured, dropping", xlog.String("to", m.To), xlog.String("subject", m.Subject))
	return nil
}

type smtpSender struct{ cfg *Config }

func (s *smtpSender) Send(ctx context.Context, m Message) error {
	from, err := mail.ParseAddress(s.cfg.From)
	if err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return fmt.Errorf("mail: to: %w", err)
	}
	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprint(s.cfg.Port))
	client, err := s.dial(ctx, addr)
	if err != nil {
		return fmt.Errorf("mail: dial %s: %w", addr, err)
	}
	defer client.Close()
	if s.cfg.TLS == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("mail: starttls: %w", err)
			}
		}
	}
	if s.cfg.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("mail: rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	if _, err := w.Write(render(from, to, m)); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: send: %w", err)
	}
	return client.Quit()
}

func (s *smtpSender) dial(ctx context.Context, addr string) (*smtp.Client, error) {
	d := net.Dialer{Timeout: s.cfg.Timeout}
	if s.cfg.TLS == "tls" {
		td := tls.Dialer{NetDialer: &d, Config: &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}}
		conn, err := td.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, s.cfg.Host)
	}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return smtp.NewClient(conn, s.cfg.Host)
}

func render(from, to *mail.Address, m Message) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from.String())
	fmt.Fprintf(&b, "To: %s\r\n", to.String())
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", m.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(strings.ReplaceAll(m.Text, "\n", "\r\n"))
	return []byte(b.String())
}
