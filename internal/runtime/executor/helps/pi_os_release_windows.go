//go:build windows

package helps

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func piOSRelease() string {
	version := windows.RtlGetVersion()
	if version == nil {
		return "windows"
	}
	return fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.BuildNumber)
}
