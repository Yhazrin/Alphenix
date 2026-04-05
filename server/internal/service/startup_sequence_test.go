package service

import (
	"testing"
)

func TestBuildTagSet(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want map[string]bool
	}{
		{"nil", nil, map[string]bool{}},
		{"empty", []string{}, map[string]bool{}},
		{"single", []string{"gpu"}, map[string]bool{"gpu": true}},
		{"multiple", []string{"gpu", "linux", "x86"}, map[string]bool{"gpu": true, "linux": true, "x86": true}},
		{"duplicate", []string{"gpu", "gpu"}, map[string]bool{"gpu": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTagSet(tt.tags)
			if len(got) != len(tt.want) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tt.want))
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("missing key %q", k)
				}
			}
		})
	}
}

func TestRuntimeTagsSatisfyPolicy(t *testing.T) {
	tests := []struct {
		name     string
		runtime  []string
		required []string
		forbid   []string
		want     bool
	}{
		{"no constraints matches anything", []string{"gpu"}, nil, nil, true},
		{"required tag present", []string{"gpu", "linux"}, []string{"gpu"}, nil, true},
		{"required tag missing", []string{"linux"}, []string{"gpu"}, nil, false},
		{"forbidden tag absent", []string{"gpu"}, nil, []string{"windows"}, true},
		{"forbidden tag present", []string{"gpu", "windows"}, nil, []string{"windows"}, false},
		{"required present + forbidden absent", []string{"gpu", "linux"}, []string{"gpu"}, []string{"windows"}, true},
		{"required present + forbidden present", []string{"gpu", "windows"}, []string{"gpu"}, []string{"windows"}, false},
		{"empty runtime with required fails", []string{}, []string{"gpu"}, nil, false},
		{"empty runtime with no constraints passes", []string{}, nil, nil, true},
		{"multiple required all present", []string{"gpu", "linux", "x86"}, []string{"gpu", "linux"}, nil, true},
		{"multiple required one missing", []string{"gpu"}, []string{"gpu", "linux"}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tagSet := buildTagSet(tt.runtime)
			got := runtimeTagsSatisfyPolicy(tagSet, tt.required, tt.forbid)
			if got != tt.want {
				t.Errorf("runtimeTagsSatisfyPolicy(%v, required=%v, forbid=%v) = %v, want %v",
					tt.runtime, tt.required, tt.forbid, got, tt.want)
			}
		})
	}
}
