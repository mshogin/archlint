package analyzer

import "testing"

func TestMatchesExclude(t *testing.T) {
	tests := []struct {
		name    string
		dir     string
		exclude []string
		want    bool
	}{
		{"empty exclude list", "Library", nil, false},
		{"exact match", "Library", []string{"Library"}, true},
		{"one of many", "Caches", []string{"Library", "Caches", "Movies"}, true},
		{"no match", "Documents", []string{"Library", "Caches"}, false},
		{"case sensitive", "library", []string{"Library"}, false},
		{"empty dir name", "", []string{"Library"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesExclude(tt.dir, tt.exclude); got != tt.want {
				t.Errorf("MatchesExclude(%q, %v) = %v, want %v", tt.dir, tt.exclude, got, tt.want)
			}
		})
	}
}
