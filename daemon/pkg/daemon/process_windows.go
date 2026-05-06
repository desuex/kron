//go:build windows

package daemon

func pidRunning(pid int) bool {
	return false
}
