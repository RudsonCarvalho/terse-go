package terse

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func mustSerialize(t *testing.T, v any) string {
	t.Helper()
	out, err := Serialize(v)
	if err != nil {
		t.Fatalf("Serialize error: %v", err)
	}
	return out
}

func mustParse(t *testing.T, src string) any {
	t.Helper()
	v, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", src, err)
	}
	return v
}

// ---------------------------------------------------------------------------
// B1 – Primitives
// ---------------------------------------------------------------------------

func TestB1_Null(t *testing.T) {
	if got := mustSerialize(t, nil); got != "~" {
		t.Errorf("want ~ got %q", got)
	}
	if got := mustParse(t, "~"); got != nil {
		t.Errorf("want nil got %v", got)
	}
}

func TestB1_BoolTrue(t *testing.T) {
	if got := mustSerialize(t, true); got != "T" {
		t.Errorf("want T got %q", got)
	}
	if got := mustParse(t, "T"); got != true {
		t.Errorf("want true got %v", got)
	}
}

func TestB1_BoolFalse(t *testing.T) {
	if got := mustSerialize(t, false); got != "F" {
		t.Errorf("want F got %q", got)
	}
	if got := mustParse(t, "F"); got != false {
		t.Errorf("want false got %v", got)
	}
}

func TestB1_Integer(t *testing.T) {
	if got := mustSerialize(t, float64(42)); got != "42" {
		t.Errorf("want 42 got %q", got)
	}
	v := mustParse(t, "42")
	if v.(float64) != 42 {
		t.Errorf("want 42 got %v", v)
	}
}

func TestB1_Float(t *testing.T) {
	if got := mustSerialize(t, 3.14); got != "3.14" {
		t.Errorf("want 3.14 got %q", got)
	}
	v := mustParse(t, "3.14")
	if math.Abs(v.(float64)-3.14) > 1e-10 {
		t.Errorf("want 3.14 got %v", v)
	}
}

func TestB1_SafeString(t *testing.T) {
	if got := mustSerialize(t, "hello"); got != "hello" {
		t.Errorf("want hello got %q", got)
	}
}

func TestB1_QuotedString(t *testing.T) {
	s := "hello world"
	got := mustSerialize(t, s)
	if got != `"hello world"` {
		t.Errorf("want quoted got %q", got)
	}
	if v := mustParse(t, got); v.(string) != s {
		t.Errorf("round-trip failed: want %q got %q", s, v)
	}
}

// ---------------------------------------------------------------------------
// B2 – Objects
// ---------------------------------------------------------------------------

func TestB2_InlineObject_TrailingSpace(t *testing.T) {
	// inline-obj = "{" *( key ":" value SP ) "}"
	got := mustSerialize(t, map[string]any{"a": float64(1)})
	if got != "{a:1 }" {
		t.Errorf("want {a:1 } got %q", got)
	}
}

func TestB2_InlineObject_RoundTrip(t *testing.T) {
	m := map[string]any{"a": float64(1), "b": float64(2)}
	got := mustSerialize(t, m)
	v := mustParse(t, got)
	if !reflect.DeepEqual(v, m) {
		t.Errorf("round-trip: %v vs %v", v, m)
	}
}

func TestB2_BlockObject(t *testing.T) {
	m := map[string]any{
		"name":    "Alice",
		"age":     float64(30),
		"active":  true,
		"balance": 9999.99,
		"address": "123 Main Street, Springfield, Some Long State Name",
	}
	got := mustSerialize(t, m)
	v := mustParse(t, got)
	if !reflect.DeepEqual(v, m) {
		t.Errorf("round-trip:\ngot  %v\nwant %v", v, m)
	}
}

func TestB2_EmptyObject(t *testing.T) {
	if got := mustSerialize(t, map[string]any{}); got != "{}" {
		t.Errorf("want {} got %q", got)
	}
}

// ---------------------------------------------------------------------------
// B3 – Arrays
// ---------------------------------------------------------------------------

func TestB3_InlineArray_TrailingSpace(t *testing.T) {
	// inline-arr = "[" *( value SP ) "]"
	got := mustSerialize(t, []any{float64(1), float64(2), float64(3)})
	if got != "[1 2 3 ]" {
		t.Errorf("want [1 2 3 ] got %q", got)
	}
}

func TestB3_InlineArray_RoundTrip(t *testing.T) {
	arr := []any{float64(1), float64(2), float64(3)}
	got := mustSerialize(t, arr)
	v := mustParse(t, got)
	if !reflect.DeepEqual(v, arr) {
		t.Errorf("round-trip: %v vs %v", v, arr)
	}
}

func TestB3_EmptyArray(t *testing.T) {
	if got := mustSerialize(t, []any{}); got != "[]" {
		t.Errorf("want [] got %q", got)
	}
}

func TestB3_MixedArray(t *testing.T) {
	arr := []any{"hello", float64(42), true, nil}
	got := mustSerialize(t, arr)
	v := mustParse(t, got)
	if !reflect.DeepEqual(v, arr) {
		t.Errorf("round-trip: %v vs %v", v, arr)
	}
}

func TestB3_BlockArray_NoDash(t *testing.T) {
	// block-arr must NOT use "- value" YAML syntax
	arr := []any{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota"}
	got := mustSerialize(t, arr)
	if strings.Contains(got, "- ") {
		t.Errorf("block array must not use '- ' prefix, got:\n%s", got)
	}
	if !strings.HasPrefix(got, "[") {
		t.Errorf("block array must start with '[', got:\n%s", got)
	}
	v := mustParse(t, got)
	if !reflect.DeepEqual(v, arr) {
		t.Errorf("round-trip:\ngot  %v\nwant %v", v, arr)
	}
}

// ---------------------------------------------------------------------------
// B4 – Schema Arrays
// ---------------------------------------------------------------------------

func TestB4_SchemaArray_TrailingSpace(t *testing.T) {
	// header: #[k1 k2 ]   rows: v1 v2
	arr := []any{
		map[string]any{"id": float64(1), "name": "Alice"},
		map[string]any{"id": float64(2), "name": "Bob"},
	}
	got := mustSerialize(t, arr)
	lines := strings.Split(got, "\n")
	header := lines[0]
	if !strings.HasSuffix(header, " ]") {
		t.Errorf("schema header must end with ' ]', got: %q", header)
	}
	for _, row := range lines[1:] {
		if row != "" && !strings.HasSuffix(row, " ") {
			t.Errorf("schema row must end with SP, got: %q", row)
		}
	}
}

func TestB4_SchemaArray_RoundTrip(t *testing.T) {
	arr := []any{
		map[string]any{"id": float64(1), "name": "Alice", "score": float64(95)},
		map[string]any{"id": float64(2), "name": "Bob", "score": float64(87)},
		map[string]any{"id": float64(3), "name": "Carol", "score": float64(92)},
	}
	got := mustSerialize(t, arr)
	if !strings.HasPrefix(got, "#[") {
		t.Errorf("expected schema array, got %q", got)
	}
	v := mustParse(t, got)
	if !reflect.DeepEqual(v, arr) {
		t.Errorf("round-trip:\ngot  %v\nwant %v", v, arr)
	}
}

func TestB4_SchemaArray_NotEligible_OneRow(t *testing.T) {
	arr := []any{map[string]any{"id": float64(1), "name": "Alice"}}
	got := mustSerialize(t, arr)
	if strings.HasPrefix(got, "#[") {
		t.Errorf("single-element array must not use schema format")
	}
}

func TestB4_SchemaArray_NotEligible_DifferentKeys(t *testing.T) {
	arr := []any{
		map[string]any{"id": float64(1), "name": "Alice"},
		map[string]any{"id": float64(2), "email": "bob@example.com"},
	}
	got := mustSerialize(t, arr)
	if strings.HasPrefix(got, "#[") {
		t.Errorf("different-keyed maps must not use schema format")
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestEdge_NullInMap(t *testing.T) {
	m := map[string]any{"x": nil}
	got := mustSerialize(t, m)
	v := mustParse(t, got)
	if !reflect.DeepEqual(v, m) {
		t.Errorf("round-trip: %v vs %v", v, m)
	}
}

func TestEdge_BackslashInString(t *testing.T) {
	s := `path\to\file`
	got := mustSerialize(t, s)
	v := mustParse(t, got)
	if v.(string) != s {
		t.Errorf("round-trip: want %q got %q", s, v)
	}
}

func TestEdge_QuotesInString(t *testing.T) {
	s := `say "hello"`
	got := mustSerialize(t, s)
	v := mustParse(t, got)
	if v.(string) != s {
		t.Errorf("round-trip: want %q got %q", s, v)
	}
}

func TestEdge_NestedObject(t *testing.T) {
	m := map[string]any{
		"user": map[string]any{"name": "Bob", "age": float64(25)},
	}
	got := mustSerialize(t, m)
	v := mustParse(t, got)
	if !reflect.DeepEqual(v, m) {
		t.Errorf("round-trip:\ngot  %v\nwant %v", v, m)
	}
}

func TestEdge_IntegerValuedFloat(t *testing.T) {
	if got := mustSerialize(t, float64(42)); got != "42" {
		t.Errorf("want 42 got %q", got)
	}
}
