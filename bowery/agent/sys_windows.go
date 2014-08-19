// Copyright 2013-2014 Bowery, Inc.
// Contains system specific routines.
package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel         = syscall.NewLazyDLL("kernel32.dll")
	processFirst   = kernel.NewProc("Process32First")
	processNext    = kernel.NewProc("Process32Next")
	createSnapshot = kernel.NewProc("CreateToolhelp32Snapshot")
)

// processEntry describes a process snapshot.
type processEntry struct {
	dwSize            uint32 // REQUIRED: FILL THIS OUT WITH unsafe.Sizeof(processEntry{})
	cntUsage          uint32
	pid               uint32
	th32DefaultHeapID uintptr
	th32ModuleID      uint32
	cntThreads        uint32
	ppid              uint32
	pcPriClassBase    int32
	dwFlags           uint32
	szExeFile         [260]byte // MAX_PATH is 260, only use byte if using ascii ver procs.
}

// GetPidTree gets the processes tree.
func GetPidTree(cpid int) (*Proc, error) {
	var root *Proc
	procs, err := listProcs()
	if err != nil {
		return nil, err
	}
	children := make([]*Proc, 0)

	for _, proc := range procs {
		if proc.Pid == cpid {
			root = proc
			continue
		}

		if proc.Ppid == cpid {
			p, err := GetPidTree(proc.Pid)
			if err != nil {
				return nil, err
			}

			children = append(children, p)
		}
	}

	root.Children = children
	return root, nil
}

// listProcs retrieves a list of all the process pids/ppids running.
func listProcs() ([]*Proc, error) {
	handle, _, err := createSnapshot.Call(2, 0)
	if syscall.Handle(handle) == syscall.InvalidHandle {
		return nil, err
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	procs := make([]*Proc, 0)
	procEntry := new(processEntry)
	procEntry.dwSize = uint32(unsafe.Sizeof(*procEntry))

	ret, _, err := processFirst.Call(handle, uintptr(unsafe.Pointer(procEntry)))
	if ret == 0 {
		if err == syscall.ERROR_NO_MORE_FILES {
			return procs, nil
		}

		return nil, err
	}
	procs = append(procs, &Proc{Pid: int(procEntry.pid), Ppid: int(procEntry.ppid)})

	for {
		ret, _, err := processNext.Call(handle, uintptr(unsafe.Pointer(procEntry)))
		if ret == 0 {
			if err == syscall.ERROR_NO_MORE_FILES {
				break
			}

			return nil, err
		}

		procs = append(procs, &Proc{Pid: int(procEntry.pid), Ppid: int(procEntry.ppid)})
	}

	return procs, nil
}
