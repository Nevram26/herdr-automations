package service

import (
	"strings"
	"testing"
)

var spec = Spec{
	Exe:  "/opt/ha/herdr-automations",
	Logs: "/state/daemon.log",
	Env: map[string]string{
		"PATH":                    "/Users/me/.local/bin:/usr/bin",
		"HERDR_PLUGIN_CONFIG_DIR": "/cfg & co",
	},
}

// launchd gives a service a bare PATH and none of the shell's variables. The
// unit must carry the ones the daemon needs to find hc, git, the agents and
// the same automations.yaml the CLI sees.
func TestPlistCarriesTheDaemonAndItsEnvironment(t *testing.T) {
	p := plist(spec)
	for _, want := range []string{
		"<string>" + Label + "</string>",
		"<string>/opt/ha/herdr-automations</string>",
		"<string>daemon</string>", "<string>--standby</string>",
		"<key>PATH</key>", "<string>/Users/me/.local/bin:/usr/bin</string>",
		"<string>/cfg &amp; co</string>", // escaped, or launchd rejects the file
		"<key>KeepAlive</key>", "<key>RunAtLoad</key>",
		"<string>/state/daemon.log</string>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("plist lacks %q:\n%s", want, p)
		}
	}
}

func TestUnitCarriesTheDaemonAndItsEnvironment(t *testing.T) {
	u := unit(spec)
	for _, want := range []string{
		`ExecStart="/opt/ha/herdr-automations" daemon --standby`,
		`Environment="PATH=/Users/me/.local/bin:/usr/bin"`,
		`Environment="HERDR_PLUGIN_CONFIG_DIR=/cfg & co"`,
		"Restart=always",
		"WantedBy=default.target",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("unit lacks %q:\n%s", want, u)
		}
	}
}

// The Windows logon script: env first, then the daemon in a restart loop, with
// % doubled (a batch file would read %NAME% as a variable).
func TestWindowsScriptSetsEnvAndRestartsTheDaemon(t *testing.T) {
	got := script(Spec{Exe: `C:\bin\herdr-automations.exe`, Logs: `C:\state\daemon.log`, Env: map[string]string{"PATH": `C:\a;C:\100%`}})
	for _, want := range []string{
		`set "PATH=C:\a;C:\100%%"` + "\r\n",
		":loop\r\n" + `"C:\bin\herdr-automations.exe" daemon --standby >> "C:\state\daemon.log" 2>&1` + "\r\n",
		"goto loop\r\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("script missing %q:\n%s", want, got)
		}
	}
}
