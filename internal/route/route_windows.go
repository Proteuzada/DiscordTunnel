//go:build windows

package route

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type Snapshot struct {
	DefaultGateway string
	HasSplitHijack bool
}

func Capture() (Snapshot, error) {
	out, err := exec.Command("route", "print", "-4").Output()
	if err != nil {
		return Snapshot{}, err
	}
	return parseRoutePrint(out)
}

func Validate(before Snapshot, after Snapshot) error {
	if after.HasSplitHijack {
		return fmt.Errorf("rota de túnel completo detectada (0.0.0.0/1 ou 128.0.0.0/1)")
	}
	if before.DefaultGateway != "" && after.DefaultGateway != "" &&
		before.DefaultGateway != after.DefaultGateway {
		return fmt.Errorf("gateway padrão mudou: era %s, agora %s", before.DefaultGateway, after.DefaultGateway)
	}
	return nil
}

func CleanupSplitRoutes() {
	out, err := exec.Command("route", "print", "-4").Output()
	if err != nil {
		return
	}
	snap, _ := parseRoutePrint(out)
	if !snap.HasSplitHijack {
		return
	}
	for _, dest := range []string{"0.0.0.0", "128.0.0.0"} {
		_ = exec.Command("route", "delete", dest).Run()
	}
}

func parseRoutePrint(out []byte) (Snapshot, error) {
	var snap Snapshot
	sc := bufio.NewScanner(bytes.NewReader(out))
	inActive := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "Active Routes:") {
			inActive = true
			continue
		}
		if !inActive {
			continue
		}
		if line == "" || strings.HasPrefix(line, "Persistent") {
			break
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		dest, mask, gateway := fields[0], fields[1], fields[2]
		if dest == "0.0.0.0" && mask == "0.0.0.0" && gateway != "0.0.0.0" {
			snap.DefaultGateway = gateway
		}
		if (dest == "0.0.0.0" && mask == "128.0.0.0") ||
			(dest == "128.0.0.0" && mask == "128.0.0.0") {
			snap.HasSplitHijack = true
		}
	}
	return snap, sc.Err()
}
