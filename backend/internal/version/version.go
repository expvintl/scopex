package version

// Version 由构建脚本通过 -ldflags "-X .../internal/version.Version=v1.0.0" 注入。
var Version = "dev"
