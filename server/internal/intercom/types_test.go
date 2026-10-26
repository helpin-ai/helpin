package intercom

import (
	"encoding/json"
	"testing"
)

func TestNextCursorAcceptsObjectAndLink(t *testing.T) {
	tests := []struct {
		name string
		json string
		want NextCursor
	}{
		{name: "object", json: `{"next":{"page":3,"starting_after":"abc"}}`, want: NextCursor{Page: 3, StartingAfter: "abc"}},
		{name: "link", json: `{"next":"https://api.intercom.io/articles?page=4&per_page=50"}`, want: NextCursor{Page: 4}},
		{name: "cursor link", json: `{"next":"https://api.intercom.io/x?starting_after=WzE2"}`, want: NextCursor{StartingAfter: "WzE2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pages Pages
			if err := json.Unmarshal([]byte(tt.json), &pages); err != nil {
				t.Fatal(err)
			}
			if pages.Next == nil || *pages.Next != tt.want {
				t.Fatalf("next = %+v, want %+v", pages.Next, tt.want)
			}
		})
	}
	var last Pages
	if err := json.Unmarshal([]byte(`{"next":null}`), &last); err != nil || last.Next != nil {
		t.Fatalf("last page: %+v %v", last.Next, err)
	}
}
