package fly

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// ShouldStartFlyMachineApiProxy will attempt to run the `fly machine api-proxy` command
// if FLY_HOST env var is not set and no connection to 127.0.0.1:4280 can be made (user
// did not already start the machine api-proxy).
func ShouldStartFlyMachineApiProxy() bool {
	// If FLY_HOST is set, we do nothing, as we assume
	// the user is VPNed into Fly and using _api.internal:4280
	flyHost := os.Getenv("FLY_HOST")

	log.Printf("in ShouldStartFlyMachineApiProxy")
	log.Printf("flyHost: %v", flyHost)

	if len(flyHost) > 0 {
		flyApiHost = flyHost
		log.Print("returning false")
		return false
	}

	// Test if we can connect to the proxy address
	conn, err := net.Dial("tcp", "127.0.0.1:4280")
	if err != nil {
		log.Printf("err=%v, returning true", err)
		// If proxying is not happening unable to connect,
		// we *should* attempt to start the proxy
		return true
	}
	defer conn.Close()

	log.Printf("we connected, returning false")

	// We were able to connect, user likely already has
	// the machine api-proxy running
	return false
}

// FindFlyctlCommandPath determines if Flyctl is
// installed within the user's PATH
func FindFlyctlCommandPath() (string, error) {
	path, err := exec.LookPath("flyctl")

	if err != nil {
		return "", fmt.Errorf("could not find flyctl in PATH: %w", err)
	}

	return path, nil
}

// StartMachineProxy starts the `fly machine api-proxy` command
// and returns a function that can be used to stop it
func StartMachineProxy(exe string) (func() error, error) {
	log.Printf("in StartMachineProxy")
	log.Printf("exe=%v", exe)
	cmd := &exec.Cmd{
		Path: exe,
		Args: []string{
			exe,
			"machine",
			"api-proxy",
		},
	}
	stdout, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("starting proxy")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("could not start machine api-proxy: %w", err)
	}
	log.Printf("started proxy, pid=%v", cmd.Process.Pid)
	log.Printf("process state=%v", cmd.ProcessState)

	conn, err := net.DialTimeout("tcp", "127.0.0.1:4280", time.Second*5)
	if err != nil {
		out, err := io.ReadAll(stdout)
		if err != nil {
			log.Printf("failed to read stderr: %v", err)
		} else {
			log.Printf("error from =%s", out)
			return nil, fmt.Errorf("could not start machine api-proxy: %s", out)
		}
	}
	conn.Close()

	return func() error {
		out, err := io.ReadAll(stdout)
		if err != nil {
			log.Printf("failed to read stdout: %v", err)
		} else {
			log.Printf("output=%s", out)
		}

		if runtime.GOOS == "windows" {
			return cmd.Process.Kill()
		}
		log.Printf("killing api proxy")
		return cmd.Process.Signal(os.Interrupt)
	}, nil
}
