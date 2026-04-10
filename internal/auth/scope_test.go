package auth

import (
	"reflect"
	"testing"
)

func TestPartitionScopes(t *testing.T) {
	tests := []struct {
		name        string
		granted     string
		required    []string
		wantMatched []string
		wantMissing []string
	}{
		{
			name:     "empty granted empty required",
			granted:  "",
			required: nil,
		},
		{
			name:        "empty granted non-empty required",
			granted:     "",
			required:    []string{"a", "b"},
			wantMissing: []string{"a", "b"},
		},
		{
			name:        "all missing",
			granted:     "x y z",
			required:    []string{"a", "b", "c"},
			wantMissing: []string{"a", "b", "c"},
		},
		{
			name:        "partial missing preserves required order",
			granted:     "a c e",
			required:    []string{"a", "b", "c", "d"},
			wantMatched: []string{"a", "c"},
			wantMissing: []string{"b", "d"},
		},
		{
			name:        "all granted",
			granted:     "a b c",
			required:    []string{"a", "b", "c"},
			wantMatched: []string{"a", "b", "c"},
		},
		{
			name:        "extra whitespace in granted",
			granted:     "  a  b  c  ",
			required:    []string{"a", "b", "c"},
			wantMatched: []string{"a", "b", "c"},
		},
		{
			name:     "empty required with non-empty granted",
			granted:  "a b c",
			required: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, missing := PartitionScopes(tt.granted, tt.required)
			if !reflect.DeepEqual(matched, tt.wantMatched) {
				t.Errorf("matched = %v, want %v", matched, tt.wantMatched)
			}
			if !reflect.DeepEqual(missing, tt.wantMissing) {
				t.Errorf("missing = %v, want %v", missing, tt.wantMissing)
			}
		})
	}
}
