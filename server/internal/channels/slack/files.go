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

// ErrFileTooLarge also covers understated sizes on the download stream.
var ErrFileTooLarge = errors.New("Slack attachment exceeds the download limit")

// DownloadFile resolves metadata that Slack Connect can omit from the event.
func (c *Client) DownloadFile(ctx context.Context, token, id string, limit int64) (File, []byte, error) {
	var info struct {
		File struct {
			File
			Download string `json:"url_private_download"`
			Private  string `json:"url_private"`
		} `json:"file"`
	}
	if err := c.Get(ctx, token, "files.info", url.Values{"file": {id}}, &info); err != nil {
		return File{}, nil, err
	}
	file := info.File.File
	if file.ID != id || id == "" {
		return File{}, nil, errors.New("Slack returned a different file")
	}
	if file.Size > limit {
		return file, nil, ErrFileTooLarge
	}
	address := info.File.Download
	if address == "" {
		address = info.File.Private
	}
	if !slackFileURL(address) {
		return file, nil, errors.New("Slack file is unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return file, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return file, nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return file, nil, errors.New("Slack file download failed")
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if int64(len(data)) > limit {
		return file, nil, ErrFileTooLarge
	}
	return file, data, err
}

// Upload finalizes the uploaded bytes in the original thread.
func (c *Client) Upload(ctx context.Context, token, channel, root, name string, data []byte) (string, error) {
	var ticket struct {
		ID  string `json:"file_id"`
		URL string `json:"upload_url"`
	}
	form := url.Values{"filename": {name}, "length": {strconv.Itoa(len(data))}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"files.getUploadURLExternal", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := c.do(request, &ticket); err != nil {
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
