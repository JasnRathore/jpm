package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a semantic version
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
	Raw        string
}

// Parse parses a version string into a Version struct
// Supports formats like: 1.2.3, v1.2.3, 1.2.3-alpha, 1.2.3+build123
func Parse(versionStr string) (*Version, error) {
	v := &Version{Raw: versionStr}

	// Remove 'v' prefix if present
	versionStr = strings.TrimPrefix(versionStr, "v")
	versionStr = strings.TrimPrefix(versionStr, "V")

	// Split by '+' for build metadata
	parts := strings.SplitN(versionStr, "+", 2)
	if len(parts) == 2 {
		v.Build = parts[1]
		versionStr = parts[0]
	}

	// Split by '-' for prerelease
	parts = strings.SplitN(versionStr, "-", 2)
	if len(parts) == 2 {
		v.Prerelease = parts[1]
		versionStr = parts[0]
	}

	// Parse major.minor.patch
	versionParts := strings.Split(versionStr, ".")
	if len(versionParts) < 1 || len(versionParts) > 3 {
		return nil, fmt.Errorf("invalid version format: %s", v.Raw)
	}

	var err error
	v.Major, err = strconv.Atoi(versionParts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", versionParts[0])
	}

	if len(versionParts) > 1 {
		v.Minor, err = strconv.Atoi(versionParts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid minor version: %s", versionParts[1])
		}
	}

	if len(versionParts) > 2 {
		v.Patch, err = strconv.Atoi(versionParts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid patch version: %s", versionParts[2])
		}
	}

	return v, nil
}

// String returns the string representation of the version
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare compares two versions
// Returns: -1 if v < other, 0 if v == other, 1 if v > other
func (v *Version) Compare(other *Version) int {
	// Compare major
	if v.Major != other.Major {
		if v.Major > other.Major {
			return 1
		}
		return -1
	}

	// Compare minor
	if v.Minor != other.Minor {
		if v.Minor > other.Minor {
			return 1
		}
		return -1
	}

	// Compare patch
	if v.Patch != other.Patch {
		if v.Patch > other.Patch {
			return 1
		}
		return -1
	}

	// Compare prerelease (versions without prerelease are greater)
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease != other.Prerelease {
		// Lexicographic comparison
		if v.Prerelease > other.Prerelease {
			return 1
		}
		return -1
	}

	return 0
}

// LessThan returns true if v < other
func (v *Version) LessThan(other *Version) bool {
	return v.Compare(other) < 0
}

// GreaterThan returns true if v > other
func (v *Version) GreaterThan(other *Version) bool {
	return v.Compare(other) > 0
}

// Equal returns true if v == other
func (v *Version) Equal(other *Version) bool {
	return v.Compare(other) == 0
}

// IsCompatible checks if the version is compatible with a constraint
// Constraint formats: "1.2.3", ">=1.2.0", "^1.2.0", "~1.2.0", "1.2.x"
func (v *Version) IsCompatible(constraint string) (bool, error) {
	constraint = strings.TrimSpace(constraint)

	// Exact version
	if !strings.ContainsAny(constraint, ">=<^~*x") {
		target, err := Parse(constraint)
		if err != nil {
			return false, err
		}
		return v.Equal(target), nil
	}

	// Greater than or equal
	if strings.HasPrefix(constraint, ">=") {
		target, err := Parse(strings.TrimPrefix(constraint, ">="))
		if err != nil {
			return false, err
		}
		return v.GreaterThan(target) || v.Equal(target), nil
	}

	// Greater than
	if strings.HasPrefix(constraint, ">") {
		target, err := Parse(strings.TrimPrefix(constraint, ">"))
		if err != nil {
			return false, err
		}
		return v.GreaterThan(target), nil
	}

	// Less than or equal
	if strings.HasPrefix(constraint, "<=") {
		target, err := Parse(strings.TrimPrefix(constraint, "<="))
		if err != nil {
			return false, err
		}
		return v.LessThan(target) || v.Equal(target), nil
	}

	// Less than
	if strings.HasPrefix(constraint, "<") {
		target, err := Parse(strings.TrimPrefix(constraint, "<"))
		if err != nil {
			return false, err
		}
		return v.LessThan(target), nil
	}

	// Caret (^) - compatible with (same major version)
	if strings.HasPrefix(constraint, "^") {
		target, err := Parse(strings.TrimPrefix(constraint, "^"))
		if err != nil {
			return false, err
		}
		return v.Major == target.Major && (v.GreaterThan(target) || v.Equal(target)), nil
	}

	// Tilde (~) - compatible with (same major.minor version)
	if strings.HasPrefix(constraint, "~") {
		target, err := Parse(strings.TrimPrefix(constraint, "~"))
		if err != nil {
			return false, err
		}
		return v.Major == target.Major && v.Minor == target.Minor &&
			(v.GreaterThan(target) || v.Equal(target)), nil
	}

	// Wildcard (x or *)
	if strings.Contains(constraint, "x") || strings.Contains(constraint, "*") {
		constraint = strings.ReplaceAll(constraint, "*", "x")
		parts := strings.Split(constraint, ".")

		if len(parts) > 0 && parts[0] != "x" {
			major, _ := strconv.Atoi(parts[0])
			if v.Major != major {
				return false, nil
			}
		}

		if len(parts) > 1 && parts[1] != "x" {
			minor, _ := strconv.Atoi(parts[1])
			if v.Minor != minor {
				return false, nil
			}
		}

		if len(parts) > 2 && parts[2] != "x" {
			patch, _ := strconv.Atoi(parts[2])
			if v.Patch != patch {
				return false, nil
			}
		}

		return true, nil
	}

	return false, fmt.Errorf("unsupported constraint format: %s", constraint)
}
