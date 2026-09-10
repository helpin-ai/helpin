package service

import (
	"encoding/json"

	"github.com/helpin-ai/helpin/server/internal/tiptap"
)

func changedDocumentBlocks(before, after json.RawMessage) ([]map[string]any, []string) {
	oldNodes := documentNodeMap(before)
	var doc struct {
		Content []map[string]any `json:"content"`
	}
	_ = json.Unmarshal(after, &doc)
	changed := []map[string]any{}
	for _, node := range doc.Content {
		id := aggregateBlockID(node)
		raw, _ := json.Marshal(node)
		previous, exists := oldNodes[id]
		if !exists || tiptap.DocumentVersion(previous) != tiptap.DocumentVersion(raw) {
			changed = append(changed, map[string]any{"id": id, "type": node["type"], "content_text": truncateCommandBarText(extractEmbeddableBlockText(raw), 140)})
		}
		delete(oldNodes, id)
	}
	deleted := []string{}
	// Keep deletion receipts in original document order.
	_ = json.Unmarshal(before, &doc)
	for _, node := range doc.Content {
		if _, ok := oldNodes[aggregateBlockID(node)]; ok {
			deleted = append(deleted, aggregateBlockID(node))
		}
	}
	return changed, deleted
}

func documentNodeMap(raw json.RawMessage) map[string]json.RawMessage {
	var doc struct {
		Content []json.RawMessage `json:"content"`
	}
	_ = json.Unmarshal(raw, &doc)
	result := map[string]json.RawMessage{}
	for _, rawNode := range doc.Content {
		var node map[string]any
		if json.Unmarshal(rawNode, &node) == nil {
			result[aggregateBlockID(node)] = rawNode
		}
	}
	return result
}

func boundDocumentEditResult(response map[string]any) (json.RawMessage, error) {
	// The mutation has already committed. Never return a response-size error that
	// might cause an agent to repeat it. Mark oversized receipts explicitly.
	response["receipt_complete"] = true
	changed := response["changed_blocks"].([]map[string]any)
	deleted := response["deleted_block_ids"].([]string)
	response["changed_blocks_total"] = len(changed)
	response["deleted_blocks_total"] = len(deleted)
	for !documentOutputFits(response) {
		response["receipt_complete"] = false
		if len(changed) > 0 {
			changed = changed[:len(changed)/2]
			response["changed_blocks"] = changed
		} else if len(deleted) > 0 {
			deleted = deleted[:len(deleted)/2]
			response["deleted_block_ids"] = deleted
		} else {
			break
		}
	}
	return json.Marshal(response)
}
