package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIImagesGenerateSendsJSONAndDecodesImage(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nfake")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" || r.Header.Get("Authorization") != "Bearer key" {
			t.Fatalf("unexpected request %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != OpenAIImageModelFast || body["prompt"] != "a diagram" || body["size"] != "1536x1024" || body["n"] != float64(1) {
			t.Fatalf("body %v", body)
		}
		if _, ok := body["background"]; ok {
			t.Fatal("empty fields must be omitted")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":          []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString(png)}},
			"output_format": "png", "size": "1536x1024", "quality": "medium",
			"usage": map[string]any{"input_tokens": 12, "output_tokens": 1056, "total_tokens": 1068},
		})
	}))
	defer server.Close()

	client := NewOpenAIImagesClient("key", server.URL+"/v1", server.Client())
	result, err := client.Generate(context.Background(), ImageGenerateRequest{Model: OpenAIImageModelFast, Prompt: "a diagram", Size: "1536x1024"})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Data) != string(png) || result.OutputFormat != "png" || result.Usage.OutputTokens != 1056 {
		t.Fatalf("result %+v", result)
	}
}

func TestOpenAIImagesEditSendsMultipartImagesAndMask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("model") != OpenAIImageModelQuality || r.FormValue("prompt") != "blur emails" || r.FormValue("quality") != "high" {
			t.Fatalf("fields %v", r.MultipartForm.Value)
		}
		images := r.MultipartForm.File["image[]"]
		if len(images) != 2 || images[0].Header.Get("Content-Type") != "image/png" || images[1].Filename != "b.jpg" {
			t.Fatalf("images %+v", images)
		}
		mask := r.MultipartForm.File["mask"]
		if len(mask) != 1 {
			t.Fatal("mask missing")
		}
		file, _ := images[0].Open()
		data, _ := io.ReadAll(file)
		if string(data) != "A" {
			t.Fatalf("image bytes %q", data)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString([]byte("out"))}}})
	}))
	defer server.Close()

	client := NewOpenAIImagesClient("key", server.URL+"/v1", server.Client())
	result, err := client.Edit(context.Background(), ImageEditRequest{
		ImageGenerateRequest: ImageGenerateRequest{Model: OpenAIImageModelQuality, Prompt: "blur emails", Quality: "high"},
		Images: []ImageInput{
			{FileName: "a.png", ContentType: "image/png", Data: []byte("A")},
			{FileName: "b.jpg", ContentType: "image/jpeg", Data: []byte("B")},
		},
		Mask: &ImageInput{FileName: "mask.png", ContentType: "image/png", Data: []byte("M")},
	})
	if err != nil || string(result.Data) != "out" {
		t.Fatalf("result %+v err %v", result, err)
	}
}

func TestOpenAIImagesErrorsAreTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Your request was rejected by the safety system.","code":"moderation_blocked"}}`))
	}))
	defer server.Close()

	client := NewOpenAIImagesClient("key", server.URL, server.Client())
	_, err := client.Generate(context.Background(), ImageGenerateRequest{Model: OpenAIImageModelFast, Prompt: "x"})
	var apiErr *ImageAPIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 || apiErr.Code != "moderation_blocked" || !strings.Contains(err.Error(), "safety system") {
		t.Fatalf("err %v", err)
	}
	if _, err := client.Edit(context.Background(), ImageEditRequest{}); err == nil {
		t.Fatal("edit without images must fail before calling the API")
	}
	if _, err := NewOpenAIImagesClient("", server.URL, nil).Generate(context.Background(), ImageGenerateRequest{}); err == nil {
		t.Fatal("missing key must fail")
	}
}
