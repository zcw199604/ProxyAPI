//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package helps

import (
	"runtime"

	"golang.org/x/sys/unix"
)

func piOSRelease() string {
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return runtime.GOOS
	}
	release := make([]byte, 0, len(uname.Release))
	for _, char := range uname.Release {
		if char == 0 {
			break
		}
		release = append(release, byte(char))
	}
	if len(release) == 0 {
		return runtime.GOOS
	}
	return string(release)
}
