package main

import "testing"

func TestEventClickHouseOptionsEnablePartitionedFinalIndexes(t *testing.T) {
	options, err := eventClickHouseOptions("clickhouse://localhost:9000/usermaven")
	if err != nil {
		t.Fatalf("parse ClickHouse options: %v", err)
	}
	for _, setting := range []string{
		"do_not_merge_across_partitions_select_final",
		"use_skip_indexes_if_final_exact_mode",
	} {
		if got := options.Settings[setting]; got != 1 {
			t.Errorf("%s = %#v, want 1", setting, got)
		}
	}
}
