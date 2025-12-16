package version

// Version info injected via ldflags at build time
var (
	// Version is set via -ldflags "-X bastion/version.Version=x.x.x"
	Version = "1.0.0"

	// CommitHash is set via -ldflags "-X bastion/version.CommitHash=xxx"
	CommitHash = "unknown"

	// BuildTime is set via -ldflags "-X bastion/version.BuildTime=xxx"
	BuildTime = "unknown"
)

// GetVersion returns the version string
func GetVersion() string {
	return Version
}

// GetFullVersion returns the version including the commit hash
func GetFullVersion() string {
	if CommitHash == "unknown" || CommitHash == "" {
		return Version
	}
	return Version + " (" + CommitHash[:7] + ")"
}

// GetBuildInfo returns build metadata
func GetBuildInfo() string {
	return "Version: " + Version + "\nCommit: " + CommitHash + "\nBuild Time: " + BuildTime
}
