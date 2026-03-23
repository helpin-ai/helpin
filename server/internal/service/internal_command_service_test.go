package service

import "testing"

func TestWriteDocumentContentCommandSupportsDocumentTarget(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("docs.write_document_content")
	if !ok {
		t.Fatal("expected docs.write_document_content definition")
	}

	for _, targetType := range def.SupportedTargetTypes {
		if targetType == "document" {
			return
		}
	}

	t.Fatalf("expected docs.write_document_content to support target type document, got %#v", def.SupportedTargetTypes)
}
