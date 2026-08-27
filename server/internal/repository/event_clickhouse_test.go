package repository

import "testing"

func TestParseEventRetentionDDL(t *testing.T) {
	withoutTTL := parseEventRetentionDDL("CREATE TABLE helpin.events (_timestamp DateTime64(3)) ENGINE = MergeTree ORDER BY _timestamp")
	if withoutTTL.TTLConfigured || withoutTTL.TTLClause != "" {
		t.Fatalf("unexpected retention policy: %#v", withoutTTL)
	}
	withTTL := parseEventRetentionDDL("CREATE TABLE helpin.events (_timestamp DateTime64(3)) ENGINE = MergeTree ORDER BY _timestamp TTL _timestamp + INTERVAL 180 DAY DELETE SETTINGS index_granularity=8192")
	if !withTTL.TTLConfigured || withTTL.TTLClause != "_timestamp + INTERVAL 180 DAY DELETE" {
		t.Fatalf("retention policy: %#v", withTTL)
	}
}
