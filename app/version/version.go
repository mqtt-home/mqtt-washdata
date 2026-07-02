package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic version of the application
	Version = "dev"
	// GitCommit is the git commit hash
	GitCommit = "unknown"
	// BuildTime is the time when the binary was built
	BuildTime = "unknown"
	// GoVersion is the Go version used to build the binary
	GoVersion = runtime.Version()
)

// Info returns formatted version information
func Info() string {
	return fmt.Sprintf("Version: %s, Git Commit: %s, Build Time: %s, Go Version: %s",
		Version, GitCommit, BuildTime, GoVersion)
}

// Short returns a short version string
func Short() string {
	commit := GitCommit
	if len(commit) > 8 {
		commit = commit[:8]
	}
	return fmt.Sprintf("v%s (%s)", Version, commit)
}
