package version

import "fmt"

// Set at build time via -ldflags -X.
var (
	Version     = "dev"
	Commit      = "unknown"
	BuildDate   = "unknown"
	BuildNumber = ""
	DockerTag   = ""
)

// String returns a human-readable version string.
func String() string {
	s := fmt.Sprintf("version=%s commit=%s built=%s", Version, Commit, BuildDate)
	if BuildNumber != "" {
		s += fmt.Sprintf(" build=#%s", BuildNumber)
	}
	if DockerTag != "" {
		s += fmt.Sprintf(" docker=%s", DockerTag)
	}
	return s
}
