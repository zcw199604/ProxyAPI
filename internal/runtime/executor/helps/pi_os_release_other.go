//go:build !windows && !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package helps

import "runtime"

func piOSRelease() string {
	return runtime.GOOS
}
