package cli

import (
	"reflect"
	"testing"
)

func TestMergeExcludes(t *testing.T) {
	tests := []struct {
		name       string
		fromConfig []string
		fromCLI    []string
		want       []string
	}{
		{"both empty", nil, nil, nil},
		{"config only", []string{"a", "b"}, nil, []string{"a", "b"}},
		{"cli only", nil, []string{"a", "b"}, []string{"a", "b"}},
		{"dedupe across sources", []string{"a", "b"}, []string{"b", "c"}, []string{"a", "b", "c"}},
		{"dedupe within cli", nil, []string{"a", "a", "b"}, []string{"a", "b"}},
		{"empty string skipped", []string{""}, []string{"a"}, []string{"a"}},
		{"config order before cli", []string{"z", "a"}, []string{"b"}, []string{"z", "a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeExcludes(tt.fromConfig, tt.fromCLI)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeExcludes(%v, %v) = %v, want %v", tt.fromConfig, tt.fromCLI, got, tt.want)
			}
		})
	}
}
