// Copyright 2013-2014 Bowery, Inc.
// Contains system specific routines.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	// "os"
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

type proc struct {
	pid  int
	ppid int
}

func GetPidTree(cpid int) (*Proc, error) {
	ppid, err := getPpid(cpid)
	if err != nil {
		return nil, err
	}
	proc := &Proc{Pid: cpid, Ppid: ppid, Children: make([]*Proc, 0)}

	pids, err := pidList()
	if err != nil {
		return nil, err
	}

	for _, pid := range pids {
		if pid == cpid {
			continue
		}

		ppid, err := getPpid(pid)
		if err != nil {
			return nil, err
		}

		if ppid == cpid {
			p, err := GetPidTree(pid)
			if err != nil {
				return nil, err
			}

			proc.Children = append(proc.Children, p)
		}
	}

	return proc, nil
}

func ps(args ...string) (bytes.Buffer, error) {
	cmd := exec.Command("ps", args...)
	var stdOut, stdErr bytes.Buffer
	cmd.Stderr = &stdErr
	cmd.Stdout = &stdOut

	if err := cmd.Run(); err != nil {
		return stdErr, errors.New(strings.TrimSpace(stdErr.String()))
	}

	return stdOut, nil
}

func getPpid(pid int) (int, error) {
	pidStr := strconv.Itoa(pid)
	ppidErr := errors.New("No ppid found for pid " + pidStr)
	ppidBytes, err := ps("-p "+pidStr, "-o ppid=")
	if err != nil {
		return -1, err
	}

	ppidStr := ppidBytes.String()
	if ppidStr == "" {
		return -1, ppidErr
	}

	var ppid int
	fmt.Sscanf(strings.TrimSpace(ppidStr), "%d", &ppid)

	return ppid, nil
}

func pidList() ([]int, error) {
	pidsBuffer, err := ps("-x", "-o pid=")
	if err != nil {
		return nil, err
	}
	pids := []int{}
	scanner := bufio.NewScanner(bytes.NewReader(pidsBuffer.Bytes()))
	for scanner.Scan() {
		var pid int
		fmt.Sscanf(strings.TrimSpace(scanner.Text()), "%d", &pid)
		pids = append(pids, pid)
	}

	return pids, nil
}
