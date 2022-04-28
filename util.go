package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func currentNode() string {
	name, _ := os.Hostname()
	return strings.Replace(name, ".cern.ch", "", -1)
}

func currentEpoch() int64 {
	return time.Now().UnixNano()
}

func newUUID() (string, error) {
	value, err := exec.Command("uuidgen").Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %v", err)
	}

	return strings.TrimSpace(string(value)), nil
}
