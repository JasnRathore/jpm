package version

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input     string
		wantMajor int
		wantMinor int
		wantPatch int
		wantPre   string
		wantBuild string
		wantErr   bool
	}{
		{"1.2.3", 1, 2, 3, "", "", false},
		{"v1.2.3", 1, 2, 3, "", "", false},
		{"1.2.3-alpha", 1, 2, 3, "alpha", "", false},
		{"1.2.3+build123", 1, 2, 3, "", "build123", false},
		{"1.2.3-beta.1+build", 1, 2, 3, "beta.1", "build", false},
		{"2.0.0", 2, 0, 0, "", "", false},
		{"1.0", 1, 0, 0, "", "", false},
		{"5", 5, 0, 0, "", "", false},
		{"invalid", 0, 0, 0, "", "", true},
		{"1.2.3.4", 0, 0, 0, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if v.Major != tt.wantMajor {
				t.Errorf("Major = %d, want %d", v.Major, tt.wantMajor)
			}
			if v.Minor != tt.wantMinor {
				t.Errorf("Minor = %d, want %d", v.Minor, tt.wantMinor)
			}
			if v.Patch != tt.wantPatch {
				t.Errorf("Patch = %d, want %d", v.Patch, tt.wantPatch)
			}
			if v.Prerelease != tt.wantPre {
				t.Errorf("Prerelease = %s, want %s", v.Prerelease, tt.wantPre)
			}
			if v.Build != tt.wantBuild {
				t.Errorf("Build = %s, want %s", v.Build, tt.wantBuild)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		v1   string
		v2   string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"1.3.0", "1.2.9", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.2.3", "1.3.0", -1},
		{"1.2.3-alpha", "1.2.3", -1},
		{"1.2.3", "1.2.3-beta", 1},
		{"1.2.3-alpha", "1.2.3-beta", -1},
	}

	for _, tt := range tests {
		t.Run(tt.v1+" vs "+tt.v2, func(t *testing.T) {
			v1, _ := Parse(tt.v1)
			v2, _ := Parse(tt.v2)

			got := v1.Compare(v2)
			if got != tt.want {
				t.Errorf("Compare() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestIsCompatible(t *testing.T) {
	tests := []struct {
		version    string
		constraint string
		want       bool
		wantErr    bool
	}{
		// Exact version
		{"1.2.3", "1.2.3", true, false},
		{"1.2.3", "1.2.4", false, false},

		// Greater than or equal
		{"1.2.3", ">=1.2.0", true, false},
		{"1.2.3", ">=1.2.3", true, false},
		{"1.2.3", ">=1.3.0", false, false},

		// Greater than
		{"1.2.3", ">1.2.2", true, false},
		{"1.2.3", ">1.2.3", false, false},

		// Less than or equal
		{"1.2.3", "<=1.2.5", true, false},
		{"1.2.3", "<=1.2.3", true, false},
		{"1.2.3", "<=1.2.2", false, false},

		// Less than
		{"1.2.3", "<1.3.0", true, false},
		{"1.2.3", "<1.2.3", false, false},

		// Caret (^) - same major version
		{"1.2.3", "^1.2.0", true, false},
		{"1.5.0", "^1.2.0", true, false},
		{"2.0.0", "^1.2.0", false, false},
		{"1.1.0", "^1.2.0", false, false},

		// Tilde (~) - same major.minor version
		{"1.2.3", "~1.2.0", true, false},
		{"1.2.9", "~1.2.0", true, false},
		{"1.3.0", "~1.2.0", false, false},
		{"1.1.9", "~1.2.0", false, false},

		// Wildcard (x or *)
		{"1.2.3", "1.2.x", true, false},
		{"1.2.9", "1.2.x", true, false},
		{"1.3.0", "1.2.x", false, false},
		{"1.2.3", "1.x.x", true, false},
		{"1.9.9", "1.x.x", true, false},
		{"2.0.0", "1.x.x", false, false},
		{"1.2.3", "1.*.*", true, false},

		// Invalid constraint
		{"1.2.3", "invalid", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.version+" with "+tt.constraint, func(t *testing.T) {
			v, err := Parse(tt.version)
			if err != nil {
				t.Fatalf("failed to parse version: %v", err)
			}

			got, err := v.IsCompatible(tt.constraint)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("IsCompatible() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.2.3", "1.2.3"},
		{"v1.2.3", "1.2.3"},
		{"1.2.3-alpha", "1.2.3-alpha"},
		{"1.2.3+build", "1.2.3+build"},
		{"1.2.3-beta.1+build123", "1.2.3-beta.1+build123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			got := v.String()
			if got != tt.want {
				t.Errorf("String() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestVersionComparators(t *testing.T) {
	v1, _ := Parse("1.2.3")
	v2, _ := Parse("1.2.4")
	v3, _ := Parse("1.2.3")

	if !v1.LessThan(v2) {
		t.Errorf("1.2.3 should be less than 1.2.4")
	}

	if v2.LessThan(v1) {
		t.Errorf("1.2.4 should not be less than 1.2.3")
	}

	if !v2.GreaterThan(v1) {
		t.Errorf("1.2.4 should be greater than 1.2.3")
	}

	if v1.GreaterThan(v2) {
		t.Errorf("1.2.3 should not be greater than 1.2.4")
	}

	if !v1.Equal(v3) {
		t.Errorf("1.2.3 should equal 1.2.3")
	}

	if v1.Equal(v2) {
		t.Errorf("1.2.3 should not equal 1.2.4")
	}
}

func BenchmarkParse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Parse("1.2.3-alpha.1+build123")
	}
}

func BenchmarkCompare(b *testing.B) {
	v1, _ := Parse("1.2.3")
	v2, _ := Parse("1.2.4")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v1.Compare(v2)
	}
}

func BenchmarkIsCompatible(b *testing.B) {
	v, _ := Parse("1.2.3")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.IsCompatible("^1.2.0")
	}
}
