//go:build !linux

// Package network — tunnel_stub.go provides no-op stubs for TunnelManager
// on non-Linux platforms (e.g. Windows developer workstations).
// The real implementation lives in tunnel.go which is linux-only.
package network

import "fmt"

// TunnelManager is a stub on non-Linux platforms.
type TunnelManager struct{}

func GlobalTunnelManager() *TunnelManager { return &TunnelManager{} }

func (tm *TunnelManager) Configure(port, secret string) {}

func (tm *TunnelManager) Start() error {
	return fmt.Errorf("reverse tunnel is only supported on Linux")
}

func (tm *TunnelManager) Stop() error { return nil }

func (tm *TunnelManager) IsRunning() bool { return false }

func (tm *TunnelManager) AgentConnected() (ifaceName, ip string, connected bool, err error) {
	return "", "", false, nil
}

// SetupRoute is a no-op stub on non-Linux platforms.
func SetupRoute(cidr, iface string) error {
	return fmt.Errorf("route management is only supported on Linux")
}

// VerifyRoute is a no-op stub on non-Linux platforms.
func VerifyRoute(targetIP, iface string) error {
	return nil
}
