// Package mailer sends the transactional auth emails (verify, password reset)
// over SMTP — Mailpit in local dev, a real relay in prod. It implements
// authuc.Mailer. When no SMTP host is configured it falls back to logging the
// link so the flow stays testable without a mail server (docs/04-AUTH.md).
package mailer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Config holds the SMTP endpoint and the message envelope.
type Config struct {
	Host      string // empty => dev log-only mode
	Port      int
	Username  string
	Password  string
	From      string
	WebOrigin string // base URL for building the click-through links
}

// Mailer sends mail per Config. Safe for concurrent use.
type Mailer struct {
	cfg    Config
	logger *slog.Logger
}

// New builds a Mailer. A nil logger is tolerated (falls back to slog.Default).
func New(cfg Config, logger *slog.Logger) *Mailer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Mailer{cfg: cfg, logger: logger}
}

// SendEmailVerify emails the 24h verification link (docs/04-AUTH.md §2).
func (m *Mailer) SendEmailVerify(ctx context.Context, to, rawToken string) error {
	return m.sendAction(ctx, to, action{
		Subject: "Verify your Fluxboard email",
		Heading: "Confirm your email address",
		Intro:   "Welcome to Fluxboard. Confirm this is your address to finish setting up your account.",
		Button:  "Verify email",
		Link:    m.link("/verify-email/confirm", rawToken),
		Footer:  "This link expires in 24 hours and works once. If you did not sign up, you can ignore this email.",
	})
}

// SendPasswordReset emails the 1h reset link (docs/04-AUTH.md §2).
func (m *Mailer) SendPasswordReset(ctx context.Context, to, rawToken string) error {
	return m.sendAction(ctx, to, action{
		Subject: "Reset your Fluxboard password",
		Heading: "Reset your password",
		Intro:   "We received a request to set a new password for your Fluxboard account.",
		Button:  "Choose a new password",
		Link:    m.link("/reset-password", rawToken),
		Footer:  "This link expires in 1 hour and works once. If you did not request it, ignore this email; your password stays the same.",
	})
}

// action is a transactional email built around one call-to-action link.
type action struct {
	Subject, Heading, Intro, Button, Link, Footer string
}

// sendAction renders a with a plain-text part (always readable, the link on its
// own line) and an HTML part with a button.
func (m *Mailer) sendAction(ctx context.Context, to string, a action) error {
	text := a.Intro + "\n\n" + a.Button + ":\n" + a.Link + "\n\n" + a.Footer
	h := html.EscapeString
	page := `<!doctype html><html><body style="margin:0;padding:24px;background:#f5f5f4;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;color:#1c1917">` +
		`<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr><td align="center">` +
		`<table role="presentation" width="480" cellpadding="0" cellspacing="0" style="max-width:480px;background:#ffffff;border:1px solid #e7e5e4;border-radius:8px">` +
		`<tr><td style="padding:32px">` +
		`<p style="margin:0 0 24px;font-weight:600;font-size:15px">fluxboard</p>` +
		`<h1 style="margin:0 0 12px;font-size:20px">` + h(a.Heading) + `</h1>` +
		`<p style="margin:0 0 24px;font-size:14px;line-height:1.6;color:#44403c">` + h(a.Intro) + `</p>` +
		`<a href="` + h(a.Link) + `" style="display:inline-block;padding:12px 20px;background:#1c1917;color:#ffffff;text-decoration:none;border-radius:6px;font-size:14px;font-weight:600">` + h(a.Button) + `</a>` +
		`<p style="margin:24px 0 0;font-size:12px;line-height:1.6;color:#78716c">Button not working? Paste this link into your browser:<br>` +
		`<a href="` + h(a.Link) + `" style="color:#44403c;word-break:break-all">` + h(a.Link) + `</a></p>` +
		`<p style="margin:16px 0 0;font-size:12px;line-height:1.6;color:#78716c">` + h(a.Footer) + `</p>` +
		`</td></tr></table></td></tr></table></body></html>`
	return m.deliver(ctx, to, a.Subject, text, page)
}

// SendInvitation emails the org-invite click-through (docs/05-TENANCY-RBAC.md
// §5). The link is path-style ({WebOrigin}/invite/{token}) to match the sitemap.
func (m *Mailer) SendInvitation(ctx context.Context, to, orgName, rawToken string) error {
	base := strings.TrimRight(m.cfg.WebOrigin, "/")
	link := base + "/invite/" + url.PathEscape(rawToken)
	org := orgName
	if org == "" {
		org = "an organization"
	}
	return m.send(ctx, to, "You've been invited to "+org+" on Fluxboard",
		"You have been invited to join "+org+" on Fluxboard:\n\n"+link+
			"\n\nThis invitation expires in 7 days.")
}

// billingTemplates maps an outbox email template name (produced by the webhook
// consumer, docs/06-BILLING.md §4) to its subject and body. The billing link
// points at the org-agnostic billing page; the recipient is always an OWNER.
var billingTemplates = map[string]struct{ subject, body string }{
	"billing.welcome": {
		"Your Fluxboard subscription is active",
		"Thanks for subscribing — your plan is now active.\n\nManage it any time under Settings → Billing:",
	},
	"billing.canceled": {
		"Your Fluxboard subscription has ended",
		"Your subscription has been canceled and the organization is back on the Free plan.\n\nResubscribe under Settings → Billing:",
	},
	"billing.dunning": {
		"Action required: payment failed",
		"The latest payment for your Fluxboard subscription failed. Please update your payment method to keep your plan:\n\n",
	},
	"billing.resolved": {
		"Payment received — subscription restored",
		"Your payment went through and your subscription is active again.\n\n",
	},
}

// SendBilling delivers a billing notification to each recipient. Unknown
// templates error so a bad outbox row surfaces in the dead queue instead of
// silently dropping (09 §2 handler contract).
func (m *Mailer) SendBilling(ctx context.Context, to []string, template string) error {
	t, ok := billingTemplates[template]
	if !ok {
		return fmt.Errorf("mailer: unknown billing template %q", template)
	}
	link := strings.TrimRight(m.cfg.WebOrigin, "/") + "/settings/billing" // sitemap 02 §settings
	for _, rcpt := range to {
		if err := m.send(ctx, rcpt, t.subject, t.body+"\n"+link); err != nil {
			return err
		}
	}
	return nil
}

// SendNotification delivers one rendered in-app notification as an email
// (Phase 5 fan-out, 09 §3). Subject/body are pre-rendered by the producer; a
// footer links to the notifications page. Send-time preference recheck happens
// in the email:send job before this is called.
func (m *Mailer) SendNotification(ctx context.Context, to, subject, body string) error {
	link := strings.TrimRight(m.cfg.WebOrigin, "/") + "/notifications" // sitemap 02 §app
	return m.send(ctx, to, subject, body+"\n\nView it in Fluxboard:\n"+link)
}

// SendIntakeMail delivers a request-inbox mail to a public-form submitter.
// Plain text, no app links: the submitter has no account.
func (m *Mailer) SendIntakeMail(ctx context.Context, to, subject, body string) error {
	return m.send(ctx, to, subject, body)
}

// link builds {WebOrigin}{path}?token=<raw>.
func (m *Mailer) link(path, rawToken string) string {
	base := strings.TrimRight(m.cfg.WebOrigin, "/")
	return base + path + "?token=" + url.QueryEscape(rawToken)
}

// send delivers one plaintext message, or logs it in dev (no SMTP host).
func (m *Mailer) send(ctx context.Context, to, subject, body string) error {
	return m.deliver(ctx, to, subject, body, "")
}

// smtpTimeout bounds the whole SMTP conversation so a stalled relay cannot pin
// the request (register, password reset) that triggered the mail.
const smtpTimeout = 15 * time.Second

// deliver sends a text (and optional HTML) message, or logs it in dev (no SMTP
// host). It mirrors smtp.SendMail (STARTTLS when offered, AUTH when
// configured) over a connection with a deadline.
func (m *Mailer) deliver(ctx context.Context, to, subject, text, htmlBody string) error {
	if strings.ContainsAny(to, "\r\n") {
		return fmt.Errorf("mailer: invalid recipient")
	}
	if m.cfg.Host == "" {
		// Dev mode: no relay configured. Log the body so the link is testable.
		// Never logs token material in prod because prod configures a host.
		m.logger.Info("mailer dev fallback (SMTP not configured)", "to", to, "subject", subject, "body", text)
		return nil
	}
	msg, err := m.buildMessage(to, subject, text, htmlBody)
	if err != nil {
		return err
	}
	if err := m.smtpSend(ctx, to, msg); err != nil {
		return fmt.Errorf("mailer send: %w", err)
	}
	return nil
}

func (m *Mailer) smtpSend(ctx context.Context, to string, msg []byte) error {
	addr := net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))
	d := net.Dialer{Timeout: smtpTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(smtpTimeout))
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if m.cfg.Username != "" {
		if ok, _ := c.Extension("AUTH"); !ok {
			return fmt.Errorf("smtp: server doesn't support AUTH")
		}
		if err := c.Auth(smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(m.cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// buildMessage assembles an RFC 5322 message: text/plain alone, or
// multipart/alternative (text + HTML) when htmlBody is set. Bodies are
// quoted-printable so long HTML lines and non-ASCII text survive any relay; the
// subject is RFC 2047-encoded.
func (m *Mailer) buildMessage(to, subject, text, htmlBody string) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", m.cfg.From)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: %s\r\n", m.messageID())
	b.WriteString("MIME-Version: 1.0\r\n")

	if htmlBody == "" {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		if err := writeQP(&b, text); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}

	mw := multipart.NewWriter(&b)
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", mw.Boundary())
	for _, part := range []struct{ ctype, body string }{
		{"text/plain; charset=UTF-8", text},
		{"text/html; charset=UTF-8", htmlBody},
	} {
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.ctype},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		if err := writeQP(pw, part.body); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func writeQP(w io.Writer, s string) error {
	qp := quotedprintable.NewWriter(w)
	if _, err := qp.Write([]byte(s)); err != nil {
		return err
	}
	return qp.Close()
}

// messageID returns a unique Message-ID on the sender's domain (missing IDs
// raise spam scores).
func (m *Mailer) messageID() string {
	host := "fluxboard.local"
	if at := strings.LastIndexByte(m.cfg.From, '@'); at != -1 {
		host = strings.Trim(m.cfg.From[at+1:], "> ")
	}
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return "<" + hex.EncodeToString(buf) + "@" + host + ">"
}
