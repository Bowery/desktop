// Copyright 2013-2014 Bowery, Inc.
// Contains system specific routines.
package main

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func ps(args ...string) (*bytes.Buffer, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("ps", args...)
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		eerr, ok := err.(*exec.ExitError)
		if ok && !eerr.Success() {
			return &stdout, nil
		}

		return nil, err
	}

	return &stdout, nil
}

func GetPidTree(cpid int) (*Proc, error) {
	proc := &Proc{Pid: cpid, Ppid: -1, Children: make([]*Proc, 0)}

	ppid, err := getPpid(cpid)
	if err != nil {
		if os.IsNotExist(err) {
			return proc, nil
		}

		return nil, err
	}
	proc.Ppid = ppid

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
			// The process has already exited, so just ignore.
			if os.IsNotExist(err) {
				continue
			}

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

func getPpid(pid int) (int, error) {
	pidStr := strconv.Itoa(pid)
	buf, err := ps("-p", pidStr, "-o", "ppid=")
	if err != nil {
		return 0, err
	}

	out := strings.TrimSpace(buf.String())
	if out == "" {
		return 0, os.ErrNotExist
	}

	return strconv.Atoi(out)
}

func pidList() ([]int, error) {
	buf, err := ps("-x", "-o", "pid=")
	if err != nil {
		return nil, err
	}

	pids := []int{}
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		pid, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}

		pids = append(pids, pid)
	}

	return pids, scanner.Err()
}
