package pypi

import (
	"regexp"
	"strings"
)

var separators = regexp.MustCompile(`[-_.]+`)

// Normalize returns the PEP 503 normalized form of a project name.
func Normalize(name string) string {
	return strings.ToLower(separators.ReplaceAllString(name, "-"))
}

var sdistExt = []string{".tar.gz", ".tar.bz2", ".tar.xz", ".tgz", ".zip", ".tar"}

// Version returns the version part of a distribution file name.
// Wheels: {name}-{version}(-{build})?-{python}-{abi}-{platform}.whl, where the name has no "-".
// Other files: {name}-{version}{ext}. The version is after the last "-".
func Version(filename string) string {
	if base, ok := strings.CutSuffix(filename, ".whl"); ok {
		if parts := strings.Split(base, "-"); len(parts) >= 5 {
			return parts[1]
		}
		return ""
	}
	base := filename
	for _, ext := range sdistExt {
		if b, ok := strings.CutSuffix(base, ext); ok {
			base = b
			break
		}
	}
	if i := strings.LastIndex(base, "-"); i >= 0 {
		return base[i+1:]
	}
	return ""
}
