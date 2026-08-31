package helps

import (
	"fmt"
	"runtime"
	"strings"
)

// PiUserAgent returns the Node.js-style platform fingerprint used by Pi.
func PiUserAgent() string {
	return fmt.Sprintf("pi (%s %s; %s)", piPlatform(runtime.GOOS), strings.TrimSpace(piOSRelease()), piArchitecture(runtime.GOARCH))
}

func piPlatform(goos string) string {
	switch goos {
	case "windows":
		return "win32"
	default:
		return goos
	}
}

func piArchitecture(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "386":
		return "ia32"
	default:
		return goarch
	}
}
