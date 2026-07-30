package version

// 版本信息，通过 -ldflags 注入
var (
	Version   = "0.0.1"
	Commit    = "unknown"
	BuildTime = "unknown"
)