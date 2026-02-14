package version

// Build is set at compile time via ldflags. It represents minutes since
// 2026-01-01 00:00:00 UTC. When running via `go run`, it defaults to "dev".
var Build = "dev"
