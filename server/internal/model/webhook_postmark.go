package model

// PostmarkAddress captures an email/name pair from the inbound webhook payload.
type PostmarkAddress struct {
	Email string `json:"Email"`
	Name  string `json:"Name"`
}

// PostmarkHeader captures an inbound RFC header forwarded by Postmark.
type PostmarkHeader struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

// PostmarkInboundPayload is the payload sent by Postmark inbound webhooks.
type PostmarkInboundPayload struct {
	From              string            `json:"From"`
	FromFull          PostmarkAddress   `json:"FromFull"`
	To                string            `json:"To"`
	ToFull            []PostmarkAddress `json:"ToFull"`
	Subject           string            `json:"Subject"`
	MessageID         string            `json:"MessageID"`
	MailboxHash       string            `json:"MailboxHash"`
	TextBody          string            `json:"TextBody"`
	HtmlBody          string            `json:"HtmlBody"`
	StrippedTextReply string            `json:"StrippedTextReply"`
	Date              string            `json:"Date"`
	Headers           []PostmarkHeader  `json:"Headers"`
}

// PostmarkOpenPayload is the payload sent by Postmark open tracking webhooks.
type PostmarkOpenPayload struct {
	RecordType    string `json:"RecordType"`
	MessageID     string `json:"MessageID"`
	MessageStream string `json:"MessageStream"`
	Recipient     string `json:"Recipient"`
	FirstOpen     bool   `json:"FirstOpen"`
	ReceivedAt    string `json:"ReceivedAt"`
}
