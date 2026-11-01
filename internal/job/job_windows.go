//go:build windows

package job

import (
	"fmt"
	"syscall"
	"unsafe"
)

const jobObjectExtendedLimitInformation = 9
const jobObjectLimitKillOnJobClose = 0x00002000

type ioCounters struct {
	readOperationCount, writeOperationCount, otherOperationCount  uint64
	readTransferCount, writeTransferCount, otherTransferCount    uint64
}

type basicLimitInfo struct {
	perProcessUserTimeLimit int64
	perJobUserTimeLimit     int64
	limitFlags              uint32
	minimumWorkingSetSize   uintptr
	maximumWorkingSetSize   uintptr
	activeProcessLimit      uint32
	affinity                uintptr
	priorityClass           uint32
	schedulingClass         uint32
}

type extendedLimitInfo struct {
	basicLimitInfo basicLimitInfo
	ioInfo         ioCounters
	processMemoryLimit, jobMemoryLimit uintptr
	peakProcessMemoryUsed, peakJobMemoryUsed uintptr
}

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObject    = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJob  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJob = kernel32.NewProc("AssignProcessToJobObject")
)

type Object struct {
	handle syscall.Handle
}

func NewKillOnClose() (*Object, error) {
	h, _, err := procCreateJobObject.Call(0, 0)
	if h == 0 {
		return nil, fmt.Errorf("CreateJobObject: %v", err)
	}
	info := extendedLimitInfo{
		basicLimitInfo: basicLimitInfo{limitFlags: jobObjectLimitKillOnJobClose},
	}
	r, _, err := procSetInformationJob.Call(
		h,
		uintptr(jobObjectExtendedLimitInformation),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	if r == 0 {
		syscall.CloseHandle(syscall.Handle(h))
		return nil, fmt.Errorf("SetInformationJobObject: %v", err)
	}
	return &Object{handle: syscall.Handle(h)}, nil
}

func (j *Object) Assign(pid int) error {
	const processSetQuota = 0x0100
	const processTerminate = 0x0001
	h, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(pid))
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)
	r, _, err := procAssignProcessToJob.Call(uintptr(j.handle), uintptr(h))
	if r == 0 {
		return fmt.Errorf("AssignProcessToJobObject: %v", err)
	}
	return nil
}

func (j *Object) Close() {
	if j.handle != 0 {
		syscall.CloseHandle(j.handle)
		j.handle = 0
	}
}
