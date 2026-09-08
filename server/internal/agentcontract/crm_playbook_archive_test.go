package agentcontract

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

func TestCRMPlaybookArchivePreservesCapturedResources(t *testing.T) {
	for _, journey := range []string{"buying_intent", "sales_handoff", "renewal_recovery"} {
		specialization, err := CaptureCRMPlaybookSpecialization(journey)
		if err != nil {
			t.Fatal(err)
		}
		for _, skill := range specialization.Skills {
			archive, checksum, err := BuildCRMPlaybookSkillArchive(skill)
			if err != nil {
				t.Fatal(err)
			}
			again, againChecksum, err := BuildCRMPlaybookSkillArchive(skill)
			if err != nil || !bytes.Equal(archive, again) || checksum != againChecksum || len(checksum) != 64 {
				t.Fatal("archive is not deterministic")
			}
			reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
			if err != nil {
				t.Fatal(err)
			}
			if len(reader.File) != len(skill.Files) {
				t.Fatal("captured references were dropped")
			}
			for i, file := range reader.File {
				stream, err := file.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(stream)
				if err != nil {
					t.Fatal(err)
				}
				if err := stream.Close(); err != nil {
					t.Fatal(err)
				}
				if file.Name != skill.Key+"/"+skill.Files[i].Path || !bytes.Equal(data, skill.Files[i].Data) {
					t.Fatalf("resource changed: %s", file.Name)
				}
			}
			skill.Key = "../escape"
			if _, _, err := BuildCRMPlaybookSkillArchive(skill); err == nil {
				t.Fatal("unsafe archive root allowed")
			}
		}
	}
}
