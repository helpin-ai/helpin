package email

import "errors"

// ErrAppEmailNotConfigured is returned when a message is sent while no
// application mail sender is configured.
var ErrAppEmailNotConfigured = errors.New("application email is not configured")

// DynamicAppSender delivers through whichever sender resolve returns at send
// time, so settings saved in the app apply without a restart. resolve returns
// nil when no sender is configured.
type DynamicAppSender struct {
	resolve func() AppSender
}

var _ AppSender = (*DynamicAppSender)(nil)

// NewDynamicAppSender returns a sender that resolves its delivery sender on
// every message.
func NewDynamicAppSender(resolve func() AppSender) *DynamicAppSender {
	return &DynamicAppSender{resolve: resolve}
}

// Configured reports whether a delivery sender is currently available.
func (d *DynamicAppSender) Configured() bool { return d.current() != nil }

// FromEmail returns the current sender address, or "" when not configured.
func (d *DynamicAppSender) FromEmail() string {
	if sender := d.current(); sender != nil {
		return sender.FromEmail()
	}
	return ""
}

// SendEmail delivers one message through the current sender.
func (d *DynamicAppSender) SendEmail(to, subject, htmlBody, textBody string) error {
	sender := d.current()
	if sender == nil {
		return ErrAppEmailNotConfigured
	}
	return sender.SendEmail(to, subject, htmlBody, textBody)
}

// SendVerificationEmail renders and sends the email verification message.
func (d *DynamicAppSender) SendVerificationEmail(to, name, url string) error {
	return sendVerificationEmail(d, to, name, url)
}

// SendPasswordResetEmail renders and sends the password reset message.
func (d *DynamicAppSender) SendPasswordResetEmail(to, name, url string) error {
	return sendPasswordResetEmail(d, to, name, url)
}

// SendInviteEmail renders and sends the workspace invitation message.
func (d *DynamicAppSender) SendInviteEmail(to, inviter, workspace, url string) error {
	return sendInviteEmail(d, to, inviter, workspace, url)
}

func (d *DynamicAppSender) current() AppSender {
	if d == nil || d.resolve == nil {
		return nil
	}
	return d.resolve()
}

// Configured reports whether sender can deliver mail: false for nil and for a
// dynamic sender without a current configuration.
func Configured(sender AppSender) bool {
	if sender == nil {
		return false
	}
	if dynamic, ok := sender.(interface{ Configured() bool }); ok {
		return dynamic.Configured()
	}
	return true
}
