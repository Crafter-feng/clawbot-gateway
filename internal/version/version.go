package version

// 版本信息，通过 -ldflags 注入
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)