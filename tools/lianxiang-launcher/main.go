// Command lianxiang-launcher starts the installed browser with a stable
// Program Files location in its environment.
//
// Windows keeps Program Files on the system drive in the usual environment
// variables. An installation moved to another volume (for example
// D:\\Program Files\\链享浏览器) can therefore be mistaken for a writable
// portable install by an older browser binary. The wrapper supplies the
// actual installation directory before starting the original binary.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const coreExecutable = "lianxiang-browser-core.exe"

func main() {
	launcherPath, err := os.Executable()
	if err != nil {
		return
	}

	installRoot := filepath.Dir(launcherPath)
	corePath := filepath.Join(installRoot, coreExecutable)

	// Keep the caller's environment, replacing the three Windows variables
	// used by the pre-fix binary's protected-install detection.
	// Replace existing entries rather than appending duplicates. Windows uses
	// the first matching entry in the environment block.
	env := make([]string, 0, len(os.Environ())+3)
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if ok && (strings.EqualFold(name, "ProgramFiles") || strings.EqualFold(name, "ProgramFiles(x86)") || strings.EqualFold(name, "ProgramW6432")) {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "ProgramFiles="+installRoot, "ProgramFiles(x86)="+installRoot, "ProgramW6432="+installRoot)

	cmd := exec.Command(corePath, os.Args[1:]...)
	cmd.Dir = installRoot
	cmd.Env = env
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}
