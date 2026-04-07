// Package network provides utilities for managing and inspecting network
// interfaces on the Vantage VPS host, including detection of reverse-tunnel
// TUN/TAP virtual interfaces created by Chisel or WireGuard agents.
package network

import (
	"fmt"
	"net"
	"strings"
)

// NetworkInterface represents a single network interface and its addresses.
type NetworkInterface struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	IsUp      bool     `json:"is_up"`
	IsVirtual bool     `json:"is_virtual"`
	IsTUN     bool     `json:"is_tun"`
	MTU       int      `json:"mtu"`
}

// tunPrefixes are the interface name prefixes that indicate a TUN/TAP or VPN
// virtual interface. Chisel reverse-TUN devices show up as tun0, tun1, etc.
var tunPrefixes = []string{"tun", "tap", "wg", "utun", "vpn", "chisel"}

// virtualPrefixes are prefixes common to Docker/container virtual interfaces.
var virtualPrefixes = []string{"docker", "veth", "br-", "virbr", "dummy", "bond", "vlan"}

// ListInterfaces returns all active network interfaces on the host along
// with their IP addresses, up/down status, and TUN classification.
// This function is safe to call concurrently from multiple goroutines.
func ListInterfaces() ([]NetworkInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing network interfaces: %w", err)
	}

	result := make([]NetworkInterface, 0, len(ifaces))
	for _, iface := range ifaces {
		// Skip loopback — not useful for scanning
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			// Non-fatal: interface may have been removed between listing and
			// querying. Skip it gracefully.
			continue
		}

		var ipStrings []string
		for _, addr := range addrs {
			ipStr := addr.String()
			// Skip link-local IPv6 — they're not useful for scanning targets
			if !strings.HasPrefix(ipStr, "fe80::") {
				ipStrings = append(ipStrings, ipStr)
			}
		}

		ni := NetworkInterface{
			Name:      iface.Name,
			Addresses: ipStrings,
			IsUp:      iface.Flags&net.FlagUp != 0,
			MTU:       iface.MTU,
			IsTUN:     isTUNInterface(iface.Name),
			IsVirtual: isVirtualInterface(iface.Name),
		}
		result = append(result, ni)
	}

	return result, nil
}

// GetTUNInterfaces returns only the TUN/TAP virtual interfaces.
// Returns nil (not error) when no TUN devices are present — this is the
// normal state before an agent connects.
func GetTUNInterfaces() ([]NetworkInterface, error) {
	all, err := ListInterfaces()
	if err != nil {
		return nil, err
	}

	var tuns []NetworkInterface
	for _, iface := range all {
		if iface.IsTUN && iface.IsUp {
			tuns = append(tuns, iface)
		}
	}
	return tuns, nil
}

// GetActiveTUNIP returns the first IP address of the first active TUN interface.
// Returns ("", false, nil) when no TUN is connected.
func GetActiveTUNIP() (ifaceName, ip string, connected bool, err error) {
	tuns, err := GetTUNInterfaces()
	if err != nil {
		return "", "", false, err
	}
	if len(tuns) == 0 {
		return "", "", false, nil
	}
	tun := tuns[0]
	if len(tun.Addresses) == 0 {
		return tun.Name, "", true, nil
	}
	// Strip CIDR notation to return only the IP
	ip = stripCIDR(tun.Addresses[0])
	return tun.Name, ip, true, nil
}

// isTUNInterface returns true if the interface name looks like a TUN/TAP or
// VPN virtual interface.
func isTUNInterface(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range tunPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// isVirtualInterface returns true if the interface name looks like a virtual
// (Docker, bridge, VETH) interface.
func isVirtualInterface(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// stripCIDR removes the /prefix-length from an address string.
func stripCIDR(addr string) string {
	if idx := strings.Index(addr, "/"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
