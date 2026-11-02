package model

import "testing"

func TestSupportChannelRegistryKeysAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, spec := range SupportChannels() {
		if spec.Key == "" || seen[spec.Key] {
			t.Fatalf("blank or duplicate channel key %q", spec.Key)
		}
		seen[spec.Key] = true
		if got, ok := LookupSupportChannel(spec.Key); !ok || got != spec {
			t.Errorf("lookup %q = %+v, %v", spec.Key, got, ok)
		}
	}
}

func TestLookupSupportChannelIsExact(t *testing.T) {
	for _, key := range []string{"", "Widget", " email", "intercom", "chat"} {
		if _, ok := LookupSupportChannel(key); ok {
			t.Errorf("%q must not resolve", key)
		}
	}
}

func TestSupportChannelsReturnsCopy(t *testing.T) {
	SupportChannels()[0].Key = "changed"
	if SupportChannels()[0].Key != SupportChannelWidget {
		t.Fatal("callers must not mutate the registry")
	}
}
