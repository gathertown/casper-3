package log

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, "debug")

	logger.Debug("foo")
	if got, want := buf.String(), "level=debug msg=foo"; !strings.Contains(got, want) {
		t.Errorf("expected logging message %q to contain %q", got, want)
	}
	buf.Reset()

	logger.Info("bar")
	if got, want := buf.String(), "level=info msg=bar"; !strings.Contains(got, want) {
		t.Errorf("expected logging message %q to contain %q", got, want)
	}
}

func TestToMap(t *testing.T) {
	tests := []struct {
		name string
		in   []interface{}
		want map[string]interface{}
	}{
		{
			"empty input",
			[]interface{}{},
			map[string]interface{}{},
		},
		{
			"key without value",
			[]interface{}{"k1"},
			map[string]interface{}{},
		},
		{
			"key without value at end",
			[]interface{}{
				"k1", "v1",
				"k2",
			},
			map[string]interface{}{
				"k1": "v1",
			},
		},
		{
			"typical case",
			[]interface{}{
				"k1", "v1",
				"k2", "v2",
			},
			map[string]interface{}{
				"k1": "v1",
				"k2": "v2",
			},
		},
		{
			"same key should overwrite value",
			[]interface{}{
				"k1", "v1",
				"k2", "v2",
				"k2", "v3",
			},
			map[string]interface{}{
				"k1": "v1",
				"k2": "v3",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toMap(tt.in...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toMap(%q) = %q; want %q", tt.in, got, tt.want)
			}
		})
	}
}

func BenchmarkToMap(b *testing.B) {
	// Construct an amount of key-value pairs for the typical logging case.
	n := 5
	var keyvals = make([]interface{}, 0, n*2)
	for i := 0; i < n; i++ {
		keyvals = append(keyvals, fmt.Sprintf("key%d", i))
		keyvals = append(keyvals, fmt.Sprintf("val%d", i))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		toMap(keyvals...)
	}
}
