package daemon

import "errors"

// Windows can't replace a running process's image. A new build lands beside
// the running daemon (a running .exe can be renamed over), and the service
// daemon leaves so the service script starts the new one (see restart).
var execve = func(string, []string, []string) error {
	return errors.New("re-exec is not supported on Windows")
}
