package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newDockMediaDownloadTestClient(t testing.TB, handler http.Handler) *http.Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	client := newDockChatExternalMediaClient()
	transport := client.Transport.(*http.Transport)
	transport.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	t.Cleanup(client.CloseIdleConnections)
	return client
}

func dockDownloadPNG(marker string) []byte {
	return append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, []byte(marker)...)
}

func dockDownloadAttachments(count int) []dockChatMediaAttachment {
	attachments := make([]dockChatMediaAttachment, count)
	for i := range attachments {
		attachments[i] = dockChatMediaAttachment{ID: fmt.Sprint(i), Source: "hosted_link", URL: fmt.Sprintf("https://example.com/%d.png", i)}
	}
	return attachments
}

func TestDockChatImageDownloadsOverlapWithinLimit(t *testing.T) {
	var active, peak atomic.Int32
	gate := make(chan struct{})
	var release sync.Once
	defer release.Do(func() { close(gate) })
	client := newDockMediaDownloadTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
		}
		if current == 4 {
			release.Do(func() { close(gate) })
		}
		select {
		case <-gate:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(dockDownloadPNG(r.URL.Path))
	}))
	s := &DockChatService{externalMediaClient: client}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := s.hydrateDockChatExternalImages(ctx, dockDownloadAttachments(9))
	if err != nil {
		t.Fatal(err)
	}
	if peak.Load() != 4 || len(result) != 9 {
		t.Fatalf("peak downloads/results = %d/%d, want 4/9", peak.Load(), len(result))
	}
}

func TestDockChatImageDownloadsPreserveOrderAndSkipFailures(t *testing.T) {
	secondDone := make(chan struct{})
	client := newDockMediaDownloadTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/0.png":
			select {
			case <-secondDone:
			case <-r.Context().Done():
				return
			}
		case "/1.png":
			close(secondDone)
		case "/2.png":
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(dockDownloadPNG(r.URL.Path))
	}))
	s := &DockChatService{externalMediaClient: client}
	attachments := dockDownloadAttachments(4)
	direct := dockChatMediaAttachment{ID: "direct", Source: "support_conversation", FileType: "image/png", URL: "https://storage.example.com/direct.png"}
	attachments = append(attachments[:1], append([]dockChatMediaAttachment{direct}, attachments[1:]...)...)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := s.hydrateDockChatExternalImages(ctx, attachments)
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{"0", "direct", "1", "3"}
	if len(result) != len(wantIDs) {
		t.Fatalf("results = %d, want %d", len(result), len(wantIDs))
	}
	for i, id := range wantIDs {
		if result[i].ID != id {
			t.Fatalf("result %d ID = %q, want %q", i, result[i].ID, id)
		}
		if id == "direct" {
			if result[i] != direct {
				t.Fatal("direct attachment changed")
			}
			continue
		}
		wantURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(dockDownloadPNG("/"+id+".png"))
		if result[i].URL != wantURL || result[i].FileType != "image/png" {
			t.Fatalf("result %d image content changed", i)
		}
	}
}

func TestDockChatImageDownloadsReuseConnectionsAcrossTurns(t *testing.T) {
	addresses := map[string]bool{}
	var mu sync.Mutex
	client := newDockMediaDownloadTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		addresses[r.RemoteAddr] = true
		mu.Unlock()
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(dockDownloadPNG(r.URL.Path))
	}))
	s := &DockChatService{externalMediaClient: client}
	for range 3 {
		if _, err := s.hydrateDockChatExternalImages(context.Background(), dockDownloadAttachments(1)); err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(addresses) != 1 {
		t.Fatalf("TCP connections = %d, want one reused connection", len(addresses))
	}
}

func TestDockChatImageDownloadsCancelPendingWork(t *testing.T) {
	started := make(chan struct{}, 1)
	var requests atomic.Int32
	client := newDockMediaDownloadTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		select {
		case started <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	s := &DockChatService{externalMediaClient: client}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := s.hydrateDockChatExternalImages(ctx, dockDownloadAttachments(12))
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("download did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("downloads did not stop after cancellation")
	}
	if requests.Load() > 4 {
		t.Fatalf("started %d requests after cancellation, want at most four", requests.Load())
	}
}

func BenchmarkDockChatImageDownloads(b *testing.B) {
	client := newDockMediaDownloadTestClient(b, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(25 * time.Millisecond)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(dockDownloadPNG(r.URL.Path))
	}))
	s := &DockChatService{externalMediaClient: client}
	b.ResetTimer()
	for range b.N {
		if _, err := s.hydrateDockChatExternalImages(context.Background(), dockDownloadAttachments(8)); err != nil {
			b.Fatal(err)
		}
	}
}
