package customfields

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

var severity = Definition{Key: "severity", Type: TypeSelect, Options: []Option{{Value: "low", Label: "Low"}, {Value: "high", Label: "High"}}}

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		def  Definition
		raw  any
		want any
		err  bool
	}{
		{"text collapses spaces", Definition{Key: "plan", Type: TypeText}, "  Pro   plan ", "Pro plan", false},
		{"empty text clears", Definition{Key: "plan", Type: TypeText}, "   ", nil, false},
		{"text too long", Definition{Key: "plan", Type: TypeText}, strings.Repeat("x", MaxTextLength+1), nil, true},
		{"long text keeps lines", Definition{Key: "steps", Type: TypeLongText}, "1. Open\n2. Click", "1. Open\n2. Click", false},
		{"number", Definition{Key: "seats", Type: TypeNumber}, float64(12), float64(12), false},
		{"number from text", Definition{Key: "seats", Type: TypeNumber}, " 12.5 ", 12.5, false},
		{"number rejects words", Definition{Key: "seats", Type: TypeNumber}, "12 seats", nil, true},
		{"number rejects exponents", Definition{Key: "seats", Type: TypeNumber}, "1e9", nil, true},
		{"date", Definition{Key: "renewal", Type: TypeDate}, "2026-10-29", "2026-10-29", false},
		{"date rejects other formats", Definition{Key: "renewal", Type: TypeDate}, "29/10/2026", nil, true},
		{"boolean", Definition{Key: "vip", Type: TypeBoolean}, true, true, false},
		{"boolean rejects text", Definition{Key: "vip", Type: TypeBoolean}, "yes", nil, true},
		{"select", severity, "high", "high", false},
		{"select unknown option", severity, "urgent", nil, true},
		{"multi select dedupes", Definition{Key: "areas", Type: TypeMultiSelect, Options: severity.Options}, []any{"low", "high", "low"}, []string{"low", "high"}, false},
		{"multi select empty clears", Definition{Key: "areas", Type: TypeMultiSelect, Options: severity.Options}, []any{}, nil, false},
		{"multi select unknown", Definition{Key: "areas", Type: TypeMultiSelect, Options: severity.Options}, []any{"nope"}, nil, true},
		{"nil clears", severity, nil, nil, false},
	} {
		got, err := Validate(tc.def, tc.raw)
		if (err != nil) != tc.err {
			t.Errorf("%s: err %v", tc.name, err)
			continue
		}
		if err != nil && !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: error not ErrInvalid: %v", tc.name, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %#v, want %#v", tc.name, got, tc.want)
		}
	}
}

func TestFitsAndDisplay(t *testing.T) {
	multi := Definition{Key: "areas", Type: TypeMultiSelect, Options: severity.Options}
	for _, tc := range []struct {
		def     Definition
		stored  any
		fits    bool
		display string
	}{
		{severity, "high", true, "High"},
		{severity, "gone", false, ""},
		{multi, []any{"low", "high"}, true, "Low, High"},
		{Definition{Key: "seats", Type: TypeNumber}, "12 seats", false, ""},
		{Definition{Key: "seats", Type: TypeNumber}, float64(12), true, "12"},
		{Definition{Key: "vip", Type: TypeBoolean}, false, true, "No"},
		{Definition{Key: "renewal", Type: TypeDate}, "soon", false, ""},
		{Definition{Key: "plan", Type: TypeText}, float64(3), false, ""},
		{Definition{Key: "plan", Type: TypeText}, nil, true, ""},
	} {
		if got := Fits(tc.def, tc.stored); got != tc.fits {
			t.Errorf("Fits(%s, %#v) = %v", tc.def.Key, tc.stored, got)
		}
		if got := Display(tc.def, tc.stored); got != tc.display {
			t.Errorf("Display(%s, %#v) = %q", tc.def.Key, tc.stored, got)
		}
	}
}

func TestNormalizeOptions(t *testing.T) {
	options, err := NormalizeOptions([]Option{{Value: "high"}, {Value: "low", Label: "  Very   low "}})
	if err != nil || options[0].Label != "High" || options[1].Label != "Very low" {
		t.Fatalf("options %+v %v", options, err)
	}
	options, err = NormalizeOptions([]Option{{Value: " Enterprise  plan "}, {Value: "DE"}})
	if err != nil || options[0].Value != "Enterprise plan" || options[0].Label != "Enterprise plan" || options[1].Label != "DE" {
		t.Fatalf("free-form options %+v %v", options, err)
	}
	for _, bad := range [][]Option{{{Value: "a"}, {Value: "A"}}, {{Value: ""}}, {{Value: "a\x00b"}}, {{Value: strings.Repeat("x", 101)}}} {
		if _, err := NormalizeOptions(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v accepted", bad)
		}
	}
	if DisplayLabel(Definition{Key: "invoice_number"}) != "Invoice number" || DisplayLabel(Definition{Key: "x", Label: "Invoice"}) != "Invoice" {
		t.Fatal("display label")
	}
	if !ValidKey("invoice_number") || ValidKey("Invoice") || ValidKey("1st") || !ValidType(TypeMultiSelect) || ValidType("file") {
		t.Fatal("key or type check")
	}
}
