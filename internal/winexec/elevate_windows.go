//go:build windows

package winexec

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// RelaunchElevated starts this executable again with UAC elevation (no PowerShell window).
func RelaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(cwd)
	if err != nil {
		return err
	}
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	shellExecuteW := shell32.NewProc("ShellExecuteW")
	const swShowNormal = 1
	ret, _, _ := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		0,
		uintptr(unsafe.Pointer(dir)),
		swShowNormal,
	)
	if ret <= 32 {
		return fmt.Errorf("ShellExecuteW: %w", syscall.Errno(ret))
	}
	return nil
}
