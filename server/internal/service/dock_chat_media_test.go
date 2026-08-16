package service

import "testing"

func TestValidateDockChatMediaSignature(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
		wantErr     bool
	}{
		{name: "png", contentType: "image/png", body: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00}},
		{name: "mp4", contentType: "video/mp4", body: []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}},
		{name: "mismatched png", contentType: "image/png", body: []byte("not an image"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDockChatMediaSignature(tt.contentType, tt.body)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDockChatMediaSignature() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
