package repository

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Keep foreign keys in the audit values and human-readable snapshots in metadata.
// Missing legacy snapshots are resolved in batches, scoped to the event workspace.
func (r *PMActivityRepository) enrichTaskValueLabels(ctx context.Context, rows []model.PMActivityLog) {
	type groupKey struct{ workspace, kind string }
	groups := map[groupKey]map[string]bool{}
	kindFor := func(field string) string {
		switch field {
		case "label", "label_id":
			return "label"
		case "owner", "follower", "owner_id", "owner_member_id", "requester_id", "requester_member_id":
			return "member"
		}
		return ""
	}
	metadata := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		if row.EntityType != "task" || row.FieldName == nil {
			continue
		}
		kind := kindFor(*row.FieldName)
		if kind == "" {
			continue
		}
		meta := map[string]interface{}{}
		if len(row.Metadata) > 0 {
			_ = json.Unmarshal(row.Metadata, &meta)
		}
		if meta == nil {
			meta = map[string]interface{}{}
		}
		metadata[i] = meta
		key := groupKey{row.WorkspaceID, kind}
		for name, value := range map[string]*string{"old_label": row.OldValue, "new_label": row.NewValue} {
			if existing, ok := meta[name].(string); ok && strings.TrimSpace(existing) != "" {
				continue
			}
			if value == nil || strings.TrimSpace(*value) == "" {
				continue
			}
			if groups[key] == nil {
				groups[key] = map[string]bool{}
			}
			groups[key][*value] = true
		}
	}
	labels := map[groupKey]map[string]string{}
	for key, values := range groups {
		ids := make([]string, 0, len(values))
		for id := range values {
			ids = append(ids, id)
		}
		found := map[string]string{}
		if key.kind == "label" {
			var records []struct{ ID, Name string }
			if err := r.db.WithContext(ctx).Table("pm_labels").Select("id, name").Where("workspace_id = ? AND id IN ?", key.workspace, ids).Scan(&records).Error; err == nil {
				for _, record := range records {
					found[record.ID] = record.Name
				}
			}
		} else {
			var records []struct {
				ID                 string
				UserID             *string
				DisplayName, Email string
			}
			if err := r.db.WithContext(ctx).Table("workspace_members").Select("id, user_id, display_name, email").Where("workspace_id = ? AND (id IN ? OR user_id IN ?)", key.workspace, ids, ids).Scan(&records).Error; err == nil {
				for _, record := range records {
					name := strings.TrimSpace(record.DisplayName)
					if name == "" {
						name = record.Email
					}
					found[record.ID] = name
					if record.UserID != nil {
						found[*record.UserID] = name
					}
				}
			}
		}
		labels[key] = found
	}
	for i := range rows {
		meta := metadata[i]
		if meta == nil {
			continue
		}
		key := groupKey{rows[i].WorkspaceID, kindFor(*rows[i].FieldName)}
		for name, value := range map[string]*string{"old_label": rows[i].OldValue, "new_label": rows[i].NewValue} {
			if existing, ok := meta[name].(string); ok && strings.TrimSpace(existing) != "" {
				continue
			}
			if value != nil {
				if label := labels[key][*value]; label != "" {
					meta[name] = label
				}
			}
		}
		if raw, err := json.Marshal(meta); err == nil {
			rows[i].Metadata = raw
		}
	}
}
