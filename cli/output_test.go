package cli

import (
	"bytes"
	"strings"
	"testing"
)

func row() Row {
	return Row{
		Cols:  []string{"id", "name", "url"},
		Vals:  []string{"42", "NASA", "https://www.facebook.com/nasa"},
		Value: map[string]string{"id": "42", "name": "NASA", "url": "https://www.facebook.com/nasa"},
	}
}

func render(t *testing.T, format Format, fields []string) string {
	t.Helper()
	var buf bytes.Buffer
	o := &Output{format: format, fields: fields, w: &buf}
	if err := o.Emit(row()); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if err := o.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return buf.String()
}

func TestEmitJSONL(t *testing.T) {
	got := render(t, FormatJSONL, nil)
	if !strings.Contains(got, `"name":"NASA"`) {
		t.Errorf("jsonl = %q", got)
	}
}

func TestEmitJSONArray(t *testing.T) {
	got := render(t, FormatJSON, nil)
	if !strings.HasPrefix(got, "[") || !strings.HasSuffix(strings.TrimSpace(got), "]") {
		t.Errorf("json not array-wrapped: %q", got)
	}
}

func TestEmitCSVHeader(t *testing.T) {
	got := render(t, FormatCSV, nil)
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if lines[0] != "id,name,url" {
		t.Errorf("csv header = %q", lines[0])
	}
	if !strings.Contains(lines[1], "NASA") {
		t.Errorf("csv row = %q", lines[1])
	}
}

func TestEmitURL(t *testing.T) {
	got := strings.TrimSpace(render(t, FormatURL, nil))
	if got != "https://www.facebook.com/nasa" {
		t.Errorf("url = %q", got)
	}
}

func TestEmitFieldsProjection(t *testing.T) {
	got := render(t, FormatCSV, []string{"name", "id"})
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if lines[0] != "name,id" {
		t.Errorf("projected header = %q", lines[0])
	}
	if lines[1] != "NASA,42" {
		t.Errorf("projected row = %q", lines[1])
	}
}

func TestEmitYAML(t *testing.T) {
	got := render(t, FormatYAML, nil)
	if !strings.Contains(got, "name: \"NASA\"") {
		t.Errorf("yaml = %q", got)
	}
}
