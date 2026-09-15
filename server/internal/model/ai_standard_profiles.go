package model

import (
	"crypto/md5"
	"fmt"
)

// These stable database identities are shared with migration 202609140011.
// The hash identifies a standard row; it is not used for credentials or security.
func StandardAIProfileID(workspace, tier string) string {
	return standardAIID("helpin-standard-ai-profile|" + workspace + "|" + tier)
}

func StandardAIConnectionID(workspace, provider string) string {
	return standardAIID("helpin-standard-ai-connection|" + workspace + "|" + provider)
}

func standardAIID(identity string) string {
	sum := md5.Sum([]byte(identity))
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[:4], sum[4:6], sum[6:8], sum[8:10], sum[10:])
}
