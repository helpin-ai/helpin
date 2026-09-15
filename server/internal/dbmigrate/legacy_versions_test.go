package dbmigrate

import "testing"

func TestLegacyVersionMoves(t *testing.T) {
	core, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	byVersion := make(map[string]Migration)
	for _, migration := range core {
		byVersion[migration.Version] = migration
	}
	versions := map[string]string{correctedPipelineVersion: originalPipelineVersion}
	for current, legacy := range legacyNativeVersions {
		versions[current] = legacy
	}
	for current, legacy := range versions {
		t.Run(current, func(t *testing.T) {
			native, crm := byVersion[current], byVersion[legacy]
			if native.Checksum == "" || crm.Checksum == "" || native.Checksum == crm.Checksum {
				t.Fatal("missing or indistinguishable migration pair")
			}
			for _, tc := range []struct {
				name      string
				applied   map[string]appliedMigration
				wantMoves int
				wantError bool
			}{
				{"fresh", map[string]appliedMigration{}, 0, false},
				{"develop", map[string]appliedMigration{legacy: {Checksum: crm.Checksum}}, 0, false},
				{"native", map[string]appliedMigration{legacy: {Checksum: native.Checksum}}, 1, false},
				{"unknown checksum", map[string]appliedMigration{legacy: {Checksum: "corrupt"}}, 0, false},
				{"already merged", map[string]appliedMigration{legacy: {Checksum: crm.Checksum}, current: {Checksum: native.Checksum}}, 0, false},
				{"occupied destination", map[string]appliedMigration{legacy: {Checksum: native.Checksum}, current: {Checksum: native.Checksum}}, 0, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					moves, err := legacyVersionMoves(core, tc.applied)
					if (err != nil) != tc.wantError || len(moves) != tc.wantMoves {
						t.Fatalf("moves=%v err=%v", moves, err)
					}
				})
			}
		})
	}
}
