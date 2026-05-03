package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// DomainClient is a lightweight Postmark Account API client for sender domains.
type DomainClient struct {
	accountToken string
	httpClient   *http.Client
}

type PostmarkDomain struct {
	ID                         int    `json:"ID"`
	Name                       string `json:"Name"`
	ReturnPathDomain           string `json:"ReturnPathDomain"`
	ReturnPathDomainCNAMEValue string `json:"ReturnPathDomainCNAMEValue"`
	ReturnPathDomainVerified   bool   `json:"ReturnPathDomainVerified"`
	DKIMHost                   string `json:"DKIMHost"`
	DKIMTextValue              string `json:"DKIMTextValue"`
	DKIMPendingHost            string `json:"DKIMPendingHost"`
	DKIMPendingTextValue       string `json:"DKIMPendingTextValue"`
	DKIMVerified               bool   `json:"DKIMVerified"`
	DKIMUpdateStatus           string `json:"DKIMUpdateStatus"`
}

type postmarkCreateDomainRequest struct {
	Name             string `json:"Name"`
	ReturnPathDomain string `json:"ReturnPathDomain,omitempty"`
}

// NewDomainClient creates an Account API client. It returns nil when the token
// is empty so callers can treat custom domains as disabled.
func NewDomainClient(accountToken string) *DomainClient {
	accountToken = strings.TrimSpace(accountToken)
	if accountToken == "" {
		return nil
	}
	return &DomainClient{accountToken: accountToken, httpClient: &http.Client{}}
}

func (c *DomainClient) SetHTTPClient(httpClient *http.Client) {
	if c == nil || httpClient == nil {
		return
	}
	c.httpClient = httpClient
}

func (c *DomainClient) CreateDomain(name, returnPathDomain string) (*PostmarkDomain, error) {
	payload := postmarkCreateDomainRequest{
		Name:             strings.TrimSpace(strings.ToLower(name)),
		ReturnPathDomain: strings.TrimSpace(strings.ToLower(returnPathDomain)),
	}
	var domain PostmarkDomain
	if err := c.do("POST", "https://api.postmarkapp.com/domains", payload, &domain); err != nil {
		return nil, err
	}
	return &domain, nil
}

func (c *DomainClient) VerifyDKIM(domainID int) (*PostmarkDomain, error) {
	var domain PostmarkDomain
	if err := c.do("PUT", fmt.Sprintf("https://api.postmarkapp.com/domains/%d/verifyDkim", domainID), nil, &domain); err != nil {
		return nil, err
	}
	return &domain, nil
}

func (c *DomainClient) VerifyReturnPath(domainID int) (*PostmarkDomain, error) {
	var domain PostmarkDomain
	if err := c.do("PUT", fmt.Sprintf("https://api.postmarkapp.com/domains/%d/verifyReturnPath", domainID), nil, &domain); err != nil {
		return nil, err
	}
	return &domain, nil
}

func (c *DomainClient) do(method, url string, payload any, out any) error {
	if c == nil {
		return fmt.Errorf("postmark account client not configured")
	}

	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal postmark domain request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("create postmark domain request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Postmark-Account-Token", c.accountToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("postmark domain request: %w", err)
	}
	defer resp.Body.Close()

	var decoded struct {
		ErrorCode int    `json:"ErrorCode"`
		Message   string `json:"Message"`
	}
	if resp.StatusCode >= 400 {
		_ = json.NewDecoder(resp.Body).Decode(&decoded)
		return &PostmarkAPIError{StatusCode: resp.StatusCode, ErrorCode: decoded.ErrorCode, Message: decoded.Message}
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode postmark domain response: %w", err)
	}
	return nil
}
