package slack

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func slackFileURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" && strings.EqualFold(u.Hostname(), "files.slack.com")
}

// Download resolves private files through Slack and only sends credentials to its file host.
func (c *Client) Download(ctx context.Context, token, id string, limit int64) ([]byte, error) {
	var info struct {
		File struct {
			Download string `json:"url_private_download"`
		}
	}
	if err := c.Get(ctx, token, "files.info", url.Values{"file": {id}}, &info); err != nil {
		return nil, err
	}
	if !slackFileURL(info.File.Download) {
		return nil, errors.New("Slack file is unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.File.Download, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("Slack file download failed")
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if int64(len(data)) > limit {
		return nil, errors.New("Slack attachment exceeds size limit")
	}
	return data, err
}

// Upload finalizes the uploaded bytes in the original thread.
func (c *Client) Upload(ctx context.Context, token, channel, root, name string, data []byte) (string, error) {
	var ticket struct {
		ID  string `json:"file_id"`
		URL string `json:"upload_url"`
	}
	if err := c.Get(ctx, token, "files.getUploadURLExternal", url.Values{"filename": {name}, "length": {strconv.Itoa(len(data))}}, &ticket); err != nil {
		return "", err
	}
	if ticket.ID == "" || !slackFileURL(ticket.URL) {
		return "", errors.New("invalid Slack upload destination")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ticket.URL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		return "", errors.New("Slack file upload failed")
	}
	err = c.Call(ctx, token, "files.completeUploadExternal", map[string]any{"channel_id": channel, "thread_ts": root, "files": []map[string]string{{"id": ticket.ID, "title": name}}}, nil)
	return ticket.ID, err
}
