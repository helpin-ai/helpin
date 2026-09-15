package email

// appMessageSender is the narrow delivery contract shared by app templates.
type appMessageSender interface {
	SendEmail(to, subject, htmlBody, textBody string) error
}

// AppSender serves authentication, invitations and application notifications.
// Support reply headers, attachments and inbound processing remain on Postmark.
type AppSender interface {
	appMessageSender
	SendVerificationEmail(to, fullName, verificationURL string) error
	SendPasswordResetEmail(to, fullName, resetURL string) error
	SendInviteEmail(to, inviterName, workspaceName, joinURL string) error
	FromEmail() string
}

func (c *Client) SendVerificationEmail(to, name, url string) error {
	return sendVerificationEmail(c, to, name, url)
}
func (c *Client) SendPasswordResetEmail(to, name, url string) error {
	return sendPasswordResetEmail(c, to, name, url)
}
func (c *Client) SendInviteEmail(to, inviter, workspace, url string) error {
	return sendInviteEmail(c, to, inviter, workspace, url)
}

func (c *SMTPClient) SendVerificationEmail(to, name, url string) error {
	return sendVerificationEmail(c, to, name, url)
}
func (c *SMTPClient) SendPasswordResetEmail(to, name, url string) error {
	return sendPasswordResetEmail(c, to, name, url)
}
func (c *SMTPClient) SendInviteEmail(to, inviter, workspace, url string) error {
	return sendInviteEmail(c, to, inviter, workspace, url)
}

// NewAppSender selects SMTP when explicitly configured, otherwise optional
// Postmark. A disabled sender returns a nil interface, never a typed nil.
func NewAppSender(smtpConfig SMTPConfig, postmarkToken, postmarkFrom string) (AppSender, error) {
	if smtpConfig.Host != "" {
		return NewSMTPClient(smtpConfig)
	}
	if postmarkToken == "" {
		return nil, nil
	}
	return NewClient(postmarkToken, postmarkFrom), nil
}
