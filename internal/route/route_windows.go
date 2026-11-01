//go:build windows

package route

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/mello/tunneling/internal/winexec"
)

type Snapshot struct {
	DefaultGateway string
	HasSplitHijack bool
}

func Capture() (Snapshot, error) {
	out, err := winexec.Command("route", "print", "-4").Output()
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
	// Only the /1 prefixes OpenVPN installs for a full tunnel.
	// Never delete 0.0.0.0/0: that is the user's normal default route.
	_ = winexec.Command("route", "delete", "0.0.0.0", "mask", "128.0.0.0").Run()
	_ = winexec.Command("route", "delete", "128.0.0.0", "mask", "128.0.0.0").Run()
}

func AddHighMetricDefault(ifIndex uint32, gateway string) error {
	if ifIndex == 0 || gateway == "" {
		return fmt.Errorf("rota do túnel incompleta")
	}
	out, err := winexec.Command(
		"route", "ADD", "0.0.0.0", "MASK", "0.0.0.0", gateway,
		"METRIC", "9999", "IF", strconv.FormatUint(uint64(ifIndex), 10),
	).CombinedOutput()
	msg := string(out)
	if err != nil && !strings.Contains(strings.ToLower(msg), "already exists") && !strings.Contains(msg, "já existe") {
		return fmt.Errorf("%s", strings.TrimSpace(msg))
	}
	return nil
}

func DeleteHighMetricDefault(gateway string) {
	if gateway == "" {
		return
	}
	_ = winexec.Command("route", "DELETE", "0.0.0.0", "MASK", "0.0.0.0", gateway).Run()
}

func parseRoutePrint(out []byte) (Snapshot, error) {
	var snap Snapshot
	sc := bufio.NewScanner(bytes.NewReader(out))
	inActive := false
	bestMetric := int(^uint(0) >> 1)
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
		if dest == "0.0.0.0" && mask == "0.0.0.0" && gateway != "0.0.0.0" && gateway != "On-link" {
			metric, err := strconv.Atoi(fields[4])
			if err == nil && metric < bestMetric {
				bestMetric = metric
				snap.DefaultGateway = gateway
			}
		}
		if (dest == "0.0.0.0" && mask == "128.0.0.0") ||
			(dest == "128.0.0.0" && mask == "128.0.0.0") {
			snap.HasSplitHijack = true
		}
	}
	return snap, sc.Err()
}
