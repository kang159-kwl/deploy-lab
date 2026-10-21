package buildinfo

// Version 由编译参数注入：
// go build -ldflags "-X deploy-lab/internal/buildinfo.Version=1.0.0"
var Version = "dev"
