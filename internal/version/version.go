package version

// These variables are injected at build time via ldflags.
// Example:
//
//	go build -ldflags="-w -s \
//	  -X git.uhomes.net/uhs-go/wechat-subscription-svc/internal/version.Version=${VERSION} \
//	  -X git.uhomes.net/uhs-go/wechat-subscription-svc/internal/version.BuildTime=${BUILD_TIME} \
//	  -X git.uhomes.net/uhs-go/wechat-subscription-svc/internal/version.GitCommit=${GIT_COMMIT}"
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)
