package email

import (
	"bufio"
	"fmt"
	"net"
	"net/mail"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"
)

func smtpFixture(t *testing.T, reject bool) (SMTPConfig, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	messages := make(chan string, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					t.Error(err)
					return
				}
				respond := func(line string) bool { _, err := fmt.Fprint(conn, line+"\r\n"); return err == nil }
				if !respond("220 localhost test SMTP") {
					return
				}
				reader := textproto.NewReader(bufio.NewReader(conn))
				for {
					line, err := reader.ReadLine()
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"), strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
						if !respond("250 OK") {
							return
						}
					case line == "DATA":
						if !respond("354 send message") {
							return
						}
						data, err := reader.ReadDotBytes()
						if err != nil {
							return
						}
						messages <- string(data)
						if reject {
							if !respond("550 delivery refused") {
								return
							}
						} else {
							if !respond("250 accepted") {
								return
							}
						}
					case line == "QUIT":
						respond("221 bye")
						return
					default:
						if !respond("502 not supported") {
							return
						}
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
		<-done
	})
	host, portRaw, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil {
		t.Fatal(err)
	}
	return SMTPConfig{Host: host, Port: port, From: "Helpin <helpin@example.test>", TLSMode: "none"}, messages
}

func TestSMTPApplicationMail(t *testing.T) {
	config, messages := smtpFixture(t, false)
	client, err := NewAppSender(config, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, send := range []func() error{
		func() error {
			return client.SendVerificationEmail("user@example.test", "Person", "https://inbox.example.test/verify-email?token=test")
		},
		func() error {
			return client.SendPasswordResetEmail("user@example.test", "Person", "https://inbox.example.test/reset-password?token=test")
		},
		func() error {
			return client.SendInviteEmail("user@example.test", "Inviter", "Workspace", "https://inbox.example.test/join/test")
		},
		func() error {
			return client.SendEmail("user@example.test", "Notification", "<p>Reply received</p>", "Reply received")
		},
	} {
		if err := send(); err != nil {
			t.Fatal(err)
		}
		message := <-messages
		parsed, err := mail.ReadMessage(strings.NewReader(message))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(parsed.Header.Get("Content-Type"), "multipart/alternative") {
			t.Fatal("missing plain/HTML alternatives")
		}
		if parsed.Header.Get("To") != "<user@example.test>" {
			t.Fatalf("recipient = %q", parsed.Header.Get("To"))
		}
	}
}

func TestSMTPFailureAndHeaderInjection(t *testing.T) {
	config, _ := smtpFixture(t, true)
	client, err := NewSMTPClient(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SendEmail("user@example.test", "Subject", "body", "body"); err == nil {
		t.Fatal("delivery rejection lost")
	}
	if err := client.SendEmail("user@example.test", "Subject\r\nBcc: other@example.test", "body", "body"); err == nil {
		t.Fatal("header injection accepted")
	}
	config.TLSMode = "starttls"
	client, err = NewSMTPClient(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SendEmail("user@example.test", "Subject", "body", "body"); err == nil {
		t.Fatal("STARTTLS silently downgraded")
	}
}

func TestDisabledAppSenderIsNil(t *testing.T) {
	sender, err := NewAppSender(SMTPConfig{}, "", "")
	if sender != nil || err != nil {
		t.Fatalf("sender=%v err=%v", sender, err)
	}
}

func TestSMTPRejectsCredentialsWithoutTLS(t *testing.T) {
	_, err := NewSMTPClient(SMTPConfig{Host: "relay.lan", TLSMode: "none", Username: "user", Password: "fixture", From: "support@example.test"})
	if err == nil || !strings.Contains(err.Error(), "use starttls or tls") {
		t.Fatalf("expected actionable configuration error, got %v", err)
	}
}
