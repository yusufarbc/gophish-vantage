//go:build linux

// Package network — tunnel.go provides the TunnelManager, which manages
// the lifecycle of a Chisel reverse-tunnel server subprocess on Linux.
// This file is compiled ONLY on Linux (the Docker container target).
// On Windows (developer workstation) the stub in tunnel_stub.go is used instead.
package network

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	"github.com/gophish/gophish/logger"
)

// TunnelManager manages the Chisel server subprocess that accepts reverse
// connections from internal agents to create a TUN device on this host.
type TunnelManager struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
	port    string
	secret  string
}

// defaultTunnelManager is the process-wide singleton TunnelManager.
var defaultTunnelManager = &TunnelManager{
	port:   "9090",
	secret: "",
}

// GlobalTunnelManager returns the singleton TunnelManager.
func GlobalTunnelManager() *TunnelManager {
	return defaultTunnelManager
}

// Configure sets the listen port and shared secret for the Chisel server.
// Call this before Start(). Idempotent if the server is already stopped.
func (tm *TunnelManager) Configure(port, secret string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if port != "" {
		tm.port = port
	}
	tm.secret = secret
}

// Start launches the Chisel server as a subprocess.
// If the server is already running this is a no-op and returns nil.
// The Chisel binary must be in PATH inside the container.
func (tm *TunnelManager) Start() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.running {
		return nil // already running; idempotent
	}

	args := []string{
		"server",
		"--port", tm.port,
		"--reverse",
	}
	if tm.secret != "" {
		args = append(args, "--auth", tm.secret)
	}

	tm.cmd = exec.Command("chisel", args...)
	if err := tm.cmd.Start(); err != nil {
		return fmt.Errorf("starting chisel server: %w", err)
	}
	tm.running = true
	log.Printf("[tunnel] chisel server started on port %s (pid=%d)", tm.port, tm.cmd.Process.Pid)

	// Watch for the process to exit so we can reset the running flag
	go func() {
		if err := tm.cmd.Wait(); err != nil {
			logger.Warnf("[tunnel] chisel server exited: %v", err)
		}
		tm.mu.Lock()
		tm.running = false
		tm.mu.Unlock()
		log.Println("[tunnel] chisel server process ended")
	}()

	return nil
}

// Stop terminates the Chisel server subprocess gracefully (SIGTERM).
// If the server is not running, this is a no-op.
func (tm *TunnelManager) Stop() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if !tm.running || tm.cmd == nil || tm.cmd.Process == nil {
		return nil
	}

	if err := tm.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("stopping chisel server: %w", err)
	}
	tm.running = false
	log.Println("[tunnel] chisel server stopped")
	return nil
}

// IsRunning returns true if the Chisel server subprocess is active.
func (tm *TunnelManager) IsRunning() bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.running
}

// AgentConnected checks whether a remote agent has established a reverse
// connection by detecting a TUN interface on this host.
// Returns (ifaceName, ip, connected, error).
func (tm *TunnelManager) AgentConnected() (ifaceName, ip string, connected bool, err error) {
	return GetActiveTUNIP()
}

// SetupRoute adds a host route through the specified interface.
// Example: SetupRoute("192.168.1.0/24", "tun0")
// This executes: ip route add 192.168.1.0/24 dev tun0
// Requires NET_ADMIN capability (set in docker-compose.yml).
func SetupRoute(cidr, iface string) error {
	if cidr == "" || iface == "" {
		return fmt.Errorf("cidr and interface must not be empty")
	}
	// Validate CIDR format minimally — prevent command injection
	if strings.ContainsAny(cidr, " ;&|`$") || strings.ContainsAny(iface, " ;&|`$") {
		return fmt.Errorf("invalid characters in cidr or interface name")
	}

	cmd := exec.Command("ip", "route", "add", cidr, "dev", iface)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip route add %s dev %s: %w (output: %s)", cidr, iface, err, string(out))
	}
	log.Printf("[tunnel] route added: %s via %s", cidr, iface)
	return nil
}

// VerifyRoute checks whether a route to the given IP exists via the
// specified interface. Returns nil on success, error if the route is missing.
func VerifyRoute(targetIP, iface string) error {
	if targetIP == "" {
		return nil // no specific check requested
	}
	cmd := exec.Command("ip", "route", "get", targetIP)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("no route to %s: %w", targetIP, err)
	}
	if iface != "" && !strings.Contains(string(out), iface) {
		return fmt.Errorf("route to %s does not pass through interface %s (got: %s)", targetIP, iface, string(out))
	}
	return nil
}
