package semver

import (
	"strconv"
	"strings"
)

// SemVer represents a semantic version
type SemVer struct {
	Original string // Original string (e.g., "v1.2.3" or "latest")
	Parts    []int  // Parsed numeric parts [1, 2, 3]
}

// Parse parses a version string into a SemVer struct
func Parse(v string) SemVer {
	original := v

	// Handle special versions
	if v == "latest" {
		return SemVer{Original: v, Parts: nil}
	}

	// Remove 'v' prefix if present
	v = strings.TrimPrefix(v, "v")

	// Parse version parts
	parts := strings.Split(v, ".")
	var nums []int
	for _, part := range parts {
		// Extract numeric prefix from part (e.g., "3-beta" -> 3)
		numPart := ""
		for _, r := range part {
			if r >= '0' && r <= '9' {
				numPart += string(r)
			} else {
				break
			}
		}
		if numPart == "" {
			nums = append(nums, 0)
		} else {
			n, _ := strconv.Atoi(numPart)
			nums = append(nums, n)
		}
	}

	return SemVer{
		Original: original,
		Parts:    nums,
	}
}

// String returns the original version string
func (v SemVer) String() string {
	return v.Original
}

// Normalized returns the version without 'v' prefix
func (v SemVer) Normalized() string {
	return strings.TrimPrefix(v.Original, "v")
}

// IsLatest returns true if this is a "latest" version
func (v SemVer) IsLatest() bool {
	return v.Original == "latest"
}

// Compare compares two versions
// Returns: -1 if v < other, 0 if equal, 1 if v > other
func (v SemVer) Compare(other SemVer) int {
	// latest is always considered "greater" for sorting purposes
	if v.IsLatest() && !other.IsLatest() {
		return 1
	}
	if !v.IsLatest() && other.IsLatest() {
		return -1
	}
	if v.IsLatest() && other.IsLatest() {
		return 0
	}

	// Compare numeric parts
	maxLen := len(v.Parts)
	if len(other.Parts) > maxLen {
		maxLen = len(other.Parts)
	}

	for i := 0; i < maxLen; i++ {
		vPart := 0
		otherPart := 0

		if i < len(v.Parts) {
			vPart = v.Parts[i]
		}
		if i < len(other.Parts) {
			otherPart = other.Parts[i]
		}

		if vPart < otherPart {
			return -1
		}
		if vPart > otherPart {
			return 1
		}
	}

	return 0
}

// Less returns true if v < other (for sorting)
func (v SemVer) Less(other SemVer) bool {
	return v.Compare(other) < 0
}

// Greater returns true if v > other (for sorting)
func (v SemVer) Greater(other SemVer) bool {
	return v.Compare(other) > 0
}

// Equal returns true if versions are equal
func (v SemVer) Equal(other SemVer) bool {
	return v.Compare(other) == 0
}

// SemVers is a slice of SemVer that implements sort.Interface
type SemVers []SemVer

func (v SemVers) Len() int           { return len(v) }
func (v SemVers) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
func (v SemVers) Less(i, j int) bool { return v[i].Less(v[j]) }
