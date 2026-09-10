package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

func documentOutputFits(value any) bool {
	raw, err := json.Marshal(value)
	return err == nil && len([]rune(string(raw))) <= documentOutputBudget
}

func documentCursor(req documentReadRequest, version, tool string, position, character int) string {
	req.Cursor = ""
	raw, _ := json.Marshal(documentReadCursor{Request: req, Version: version, Tool: tool, Position: position, Character: character})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func paginateDocumentItems(response map[string]any, items []any, req documentReadRequest, version, tool string, cursor documentReadCursor, key string) (json.RawMessage, error) {
	if cursor.Position > len(items) || (cursor.Position == len(items) && cursor.Character != 0) {
		return nil, fmt.Errorf("invalid cursor position")
	}
	coversDocument := response["content_complete"] == true && cursor.Position == 0 && cursor.Character == 0
	page := []any{}
	response[key] = page
	for i := cursor.Position; i < len(items); i++ {
		response["complete"] = i+1 == len(items)
		response["content_complete"] = coversDocument && i+1 == len(items)
		response["next_cursor"] = nil
		if i+1 < len(items) {
			response["next_cursor"] = documentCursor(req, version, tool, i+1, 0)
		}
		offset := 0
		if i == cursor.Position {
			offset = cursor.Character
		}
		candidate := append(append([]any{}, page...), items[i])
		response[key] = candidate
		if offset == 0 && documentOutputFits(response) {
			page = candidate
			continue
		}
		// Never split an item just to fill a page. Split only an item that cannot fit alone.
		if len(page) > 0 {
			response[key] = page
			response["complete"] = false
			response["content_complete"] = false
			response["next_cursor"] = documentCursor(req, version, tool, i, offset)
			return marshalBoundedDocument(response)
		}
		raw, err := json.Marshal(items[i])
		if err != nil {
			return nil, err
		}
		runes := []rune(string(raw))
		if offset >= len(runes) {
			return nil, fmt.Errorf("invalid cursor character offset")
		}
		low, high := 0, len(runes)-offset
		for low < high {
			size := (low + high + 1) / 2
			setDocumentFragment(response, key, req, version, tool, i, offset, size, runes, len(items))
			if documentOutputFits(response) {
				low = size
			} else {
				high = size - 1
			}
		}
		if low == 0 {
			return nil, fmt.Errorf("document metadata exceeds response budget; request fewer block_ids")
		}
		setDocumentFragment(response, key, req, version, tool, i, offset, low, runes, len(items))
		return marshalBoundedDocument(response)
	}
	response[key] = page
	response["complete"] = true
	response["content_complete"] = coversDocument
	response["next_cursor"] = nil
	return marshalBoundedDocument(response)
}

func setDocumentFragment(response map[string]any, key string, req documentReadRequest, version, tool string, index, offset, size int, runes []rune, total int) {
	finished := offset+size == len(runes)
	response[key] = []any{map[string]any{
		"fragment_format": "item_json", "item_index": index, "fragment_offset": offset,
		"fragment_complete": finished, "content_fragment": string(runes[offset : offset+size]),
	}}
	response["complete"] = finished && index+1 == total
	response["content_complete"] = false
	response["next_cursor"] = nil
	if !finished {
		response["next_cursor"] = documentCursor(req, version, tool, index, offset+size)
	} else if index+1 < total {
		response["next_cursor"] = documentCursor(req, version, tool, index+1, 0)
	}
}

func marshalBoundedDocument(response map[string]any) (json.RawMessage, error) {
	if complete, ok := response["complete"].(bool); ok && !complete {
		response["content_complete"] = false
	}
	if !documentOutputFits(response) {
		return nil, fmt.Errorf("document metadata exceeds response budget; narrow the selection")
	}
	return json.Marshal(response)
}
