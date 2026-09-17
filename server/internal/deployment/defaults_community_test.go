//go:build !ee

package deployment

import "testing"

func TestCommunityHasNoHostedServiceDefaults(t *testing.T) {
	if DefaultReplyDomain != "" || DefaultRouteDomain != "" || DefaultWidgetOrigin != "" || DefaultSDKLoaderURL != "" {
		t.Fatal("Community must use operator-owned service addresses")
	}
}
