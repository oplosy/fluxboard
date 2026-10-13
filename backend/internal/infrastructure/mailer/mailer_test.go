package mailer

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
)

func TestBuildMessageMultipartKeepsLinkIntact(t *testing.T) {
	m := New(Config{From: "no-reply@fluxboard.io", WebOrigin: "https://app.fluxboard.io"}, nil)
	link := m.link("/reset-password", "tok_with-long+chars/=")
	raw, err := m.buildMessage("user@example.com", "Şifrenizi sıfırlayın", "Open "+link, `<a href="`+link+`">reset</a>`+strings.Repeat("x", 400))
	if err != nil {
		t.Fatal(err)
	}
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("not a valid RFC 5322 message: %v", err)
	}
	if subj, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject")); subj != "Şifrenizi sıfırlayın" {
		t.Fatalf("subject = %q", subj)
	}
	if msg.Header.Get("Date") == "" || !strings.HasSuffix(msg.Header.Get("Message-ID"), "@fluxboard.io>") {
		t.Fatalf("missing Date/Message-ID: %v", msg.Header)
	}
	mt, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/alternative" {
		t.Fatalf("content-type = %q (%v)", mt, err)
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])
	var parts []string
	for {
		p, err := mr.NextPart() // decodes quoted-printable transparently
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(p)
		parts = append(parts, string(b))
	}
	if len(parts) != 2 || !strings.Contains(parts[0], link) || !strings.Contains(parts[1], link) {
		t.Fatalf("link must survive in both parts; got %d parts", len(parts))
	}
}

func TestDeliverRejectsHeaderInjection(t *testing.T) {
	m := New(Config{Host: "127.0.0.1", Port: 1, From: "a@b.io"}, nil)
	if err := m.send(t.Context(), "x@y.io\r\nBcc: z@w.io", "s", "b"); err == nil {
		t.Fatal("a recipient with CR/LF must be refused before any SMTP traffic")
	}
}
