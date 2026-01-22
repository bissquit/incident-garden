// Package version contains build version information.
package version

// Version is the current application version.
// This value is updated automatically by Release Please.
var Version = "0.0.0"

// GitCommit is the git commit hash.
// This value is set at build time via ldflags.
var GitCommit = "unknown"

// BuildDate is the build date.
// This value is set at build time via ldflags.
var BuildDate = "unknown"
