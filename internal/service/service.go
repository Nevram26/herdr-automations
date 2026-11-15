// Package service installs the daemon as a user service: a launchd agent on
// macOS, a systemd user unit on Linux, a logon script on Windows. The Herdr desktop app has no plugin
// startup hook, and a scheduler must fire with no window open (ADR 0011).
package service

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Label names the service in launchd and systemd.
const Label = "dev.dnzzl.herdr-automations"

// Spec is what the service runs, settled at install time.
type Spec struct {
	Exe  string
	Logs string
	Env  map[string]string
}

// Install writes the service running program and starts it. env is baked in:
// the shell's PATH, so the daemon finds hc, git and the agents, and the
// resolved directories, so it reads the automations.yaml the CLI reads.
func Install(program string, env map[string]string, logs string) (string, error) {
	s := Spec{Exe: program, Logs: logs, Env: env}
	switch runtime.GOOS {
	case "darwin":
		path := plistPath()
		if err := writeFile(path, plist(s)); err != nil {
			return "", err
		}
		_ = launchctl("bootout", domain()+"/"+Label) // a previous install, if any
		return path, launchctl("bootstrap", domain(), path)
	case "linux":
		path := unitPath()
		if err := writeFile(path, unit(s)); err != nil {
			return "", err
		}
		if err := systemctl("daemon-reload"); err != nil {
			return "", err
		}
		return path, systemctl("enable", "--now", "herdr-automations.service")
	case "windows":
		path := scriptPath()
		stopScript() // a previous install's loop, if any
		// cmd opens the log before the daemon runs, so its folder must exist.
		if err := os.MkdirAll(filepath.Dir(s.Logs), 0o755); err != nil {
			return "", err
		}
		if err := writeFile(path, script(s)); err != nil {
			return "", err
		}
		if err := reg("add", runKey, "/v", runValue, "/t", "REG_SZ", "/d", launch(path), "/f"); err != nil {
			return "", err
		}
		return path, startDetached(launch(path))
	}
	return "", fmt.Errorf("no service support on %s; run `herdr-automations daemon` yourself", runtime.GOOS)
}

// Uninstall stops the service and removes its file.
func Uninstall() error {
	switch runtime.GOOS {
	case "darwin":
		_ = launchctl("bootout", domain()+"/"+Label)
		return removeIfThere(plistPath())
	case "linux":
		_ = systemctl("disable", "--now", "herdr-automations.service")
		if err := removeIfThere(unitPath()); err != nil {
			return err
		}
		return systemctl("daemon-reload")
	case "windows":
		// The caller stops the daemon; this stops the loop that would restart it.
		_ = reg("delete", runKey, "/v", runValue, "/f")
		stopScript()
		return removeIfThere(scriptPath())
	}
	return fmt.Errorf("no service support on %s", runtime.GOOS)
}

// Installed reports whether the service file exists.
func Installed() bool {
	path := unitPath()
	switch runtime.GOOS {
	case "darwin":
		path = plistPath()
	case "windows":
		path = scriptPath()
	}
	_, err := os.Stat(path)
	return err == nil
}

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist")
}

func unitPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", "herdr-automations.service")
}

func domain() string { return "gui/" + strconv.Itoa(os.Getuid()) }

// plist renders the launchd agent. --standby keeps it from exiting while the
// TUI's startup hook holds the scheduler lock: KeepAlive would relaunch it
// every ten seconds otherwise.
func plist(s Spec) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>` + esc(Label) + `</string>
  <key>ProgramArguments</key>
  <array><string>` + esc(s.Exe) + `</string><string>daemon</string><string>--standby</string></array>
  <key>EnvironmentVariables</key>
  <dict>
`)
	for _, k := range sortedKeys(s.Env) {
		b.WriteString("    <key>" + esc(k) + "</key><string>" + esc(s.Env[k]) + "</string>\n")
	}
	b.WriteString(`  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>` + esc(s.Logs) + `</string>
  <key>StandardErrorPath</key><string>` + esc(s.Logs) + `</string>
</dict>
</plist>
`)
	return b.String()
}

func unit(s Spec) string {
	var b strings.Builder
	b.WriteString("[Unit]\nDescription=herdr-automations scheduler\n\n[Service]\n")
	b.WriteString(fmt.Sprintf("ExecStart=%q daemon --standby\n", s.Exe))
	for _, k := range sortedKeys(s.Env) {
		b.WriteString(fmt.Sprintf("Environment=%q\n", k+"="+s.Env[k]))
	}
	b.WriteString(fmt.Sprintf("StandardOutput=append:%s\nStandardError=append:%s\n", s.Logs, s.Logs))
	b.WriteString("Restart=always\nRestartSec=10\n\n[Install]\nWantedBy=default.target\n")
	return b.String()
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func removeIfThere(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func launchctl(args ...string) error {
	if out, err := exec.Command("launchctl", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl %s: %s", args[0], strings.TrimSpace(string(out)))
	}
	return nil
}

func systemctl(args ...string) error {
	all := append([]string{"--user"}, args...)
	if out, err := exec.Command("systemctl", all...).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl %s: %s", args[0], strings.TrimSpace(string(out)))
	}
	return nil
}

// Windows: no admin rights needed. A script restarts the daemon whenever it
// exits (launchd's KeepAlive, systemd's Restart=always); the user's Run key
// starts the script at logon, through a headless console so no window opens.
const (
	runKey   = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	runValue = "herdr-automations"
)

func scriptPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "herdr-automations", "service.cmd")
}

func launch(path string) string { return `conhost.exe --headless cmd.exe /d /c "` + path + `"` }

// script renders the logon script. A batch file reads % as a variable, so
// values double it.
func script(s Spec) string {
	q := func(v string) string { return strings.ReplaceAll(v, "%", "%%") }
	var b strings.Builder
	b.WriteString("@echo off\r\nrem herdr-automations scheduler (service install). Started at logon by the Run key.\r\n")
	for _, k := range sortedKeys(s.Env) {
		b.WriteString(`set "` + q(k) + "=" + q(s.Env[k]) + "\"\r\n")
	}
	b.WriteString(":loop\r\n\"" + q(s.Exe) + `" daemon --standby >> "` + q(s.Logs) + "\" 2>&1\r\n")
	b.WriteString("ping -n 11 127.0.0.1 >nul\r\ngoto loop\r\n")
	return b.String()
}

func reg(args ...string) error {
	if out, err := exec.Command("reg", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("reg %s: %v: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return nil
}

// stopScript ends every running copy of the script (cmd and its conhost), so a
// reinstall or uninstall leaves no old loop restarting the daemon.
func stopScript() {
	ps := `Get-CimInstance Win32_Process | Where-Object { $_.Name -in 'cmd.exe','conhost.exe' -and $_.CommandLine -like ('*' + $env:HA_SCRIPT + '*') } | Invoke-CimMethod -MethodName Terminate | Out-Null`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.Env = append(os.Environ(), "HA_SCRIPT="+scriptPath())
	_ = cmd.Run()
}

// startDetached starts the script now, through WMI: a process WMI creates
// belongs to no job of ours, so it keeps running after whatever ran install
// (a terminal, the Herdr panel) closes.
func startDetached(cmdline string) error {
	ps := `$r = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{CommandLine=$env:HA_CMD}; exit $r.ReturnValue`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.Env = append(os.Environ(), "HA_CMD="+cmdline)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("start the scheduler: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
