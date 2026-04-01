package worker

import (
	"errors"
	"testing"
)

func TestCodexNeedsManagedChatGPTFallback(t *testing.T) {
	t.Run("matches unsupported device code variant error", func(t *testing.T) {
		err := errors.New("account/login/start failed (-32602): unknown variant `chatgptDeviceCode`, expected one of `apiKey`, `chatgpt`, `chatgptAuthTokens`")
		if !codexNeedsManagedChatGPTFallback(err) {
			t.Fatalf("expected fallback for unsupported device-code login variant")
		}
	})

	t.Run("does not match unrelated error", func(t *testing.T) {
		err := errors.New("account/login/start failed (-32000): network down")
		if codexNeedsManagedChatGPTFallback(err) {
			t.Fatalf("did not expect fallback for unrelated login error")
		}
	})
}
