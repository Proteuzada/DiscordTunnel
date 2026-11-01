//go:build windows

package redirect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func FindBinary() (string, error) {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, name := range []string{"redirector.exe", filepath.Join("..", "target", "release", "redirector.exe")} {
			c := filepath.Join(dir, name)
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c, nil
			}
		}
	}
	wd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(wd, "target", "release", "redirector.exe"),
		filepath.Join(wd, "target", "debug", "redirector.exe"),
		filepath.Join(wd, "third_party", "openvpn-split", "target", "release", "redirector.exe"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("redirector.exe não encontrado — execute build.ps1")
}

func Start() (*exec.Cmd, error) {
	path, err := FindBinary()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path, "observe")
	cmd.Dir = filepath.Dir(path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
