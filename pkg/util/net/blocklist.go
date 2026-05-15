/*
Copyright 2024 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package net

import (
	"fmt"
	"net"
	"net/netip"
	"strings"

	"k8s.io/klog/v2"
)

// Blocked CIDR ranges for SSRF protection
var blockedCIDRs = []string{
	"169.254.0.0/16", // Link-local (AWS/Azure IMDS)
	"10.0.0.0/8",     // RFC 1918 private
	"172.16.0.0/12",  // RFC 1918 private
	"192.168.0.0/16", // RFC 1918 private
	"100.64.0.0/10",  // CGNAT (RFC 6598)
	"127.0.0.0/8",    // Loopback
	"0.0.0.0/8",      // Current network
	"224.0.0.0/4",    // Multicast
	"240.0.0.0/4",    // Reserved
	"fd00::/8",       // IPv6 ULA
	"fe80::/10",      // IPv6 link-local
	"::1/128",        // IPv6 loopback
}

// Validate hardcoded blocklist at package initialization
func init() {
	for _, cidr := range blockedCIDRs {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			panic(fmt.Sprintf("invalid hardcoded CIDR %q in blockedCIDRs: %v", cidr, err))
		}
	}
}

// IsBlockedIP returns true if the IP is in a blocked CIDR range
func IsBlockedIP(ip string) (bool, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false, fmt.Errorf("invalid IP address %q: %w", ip, err)
	}

	// Normalize IPv4-mapped IPv6 addresses to IPv4 to prevent SSRF bypass
	// e.g., ::ffff:169.254.169.254 -> 169.254.169.254
	addr = addr.Unmap()

	for _, cidr := range blockedCIDRs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			continue
		}
		if prefix.Contains(addr) {
			return true, nil
		}
	}
	return false, nil
}

// IsAllowedIP returns true if the IP matches any allowlist entry (CIDR or exact IP)
func IsAllowedIP(ip string, allowlist []string) (bool, error) {
	if len(allowlist) == 0 {
		return false, nil
	}

	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false, fmt.Errorf("invalid IP address %q: %w", ip, err)
	}

	// Normalize IPv4-mapped IPv6 addresses to IPv4 for consistent matching
	// e.g., ::ffff:192.168.1.1 -> 192.168.1.1
	addr = addr.Unmap()

	for _, entry := range allowlist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Try parsing as CIDR first
		if strings.Contains(entry, "/") {
			prefix, err := netip.ParsePrefix(entry)
			if err != nil {
				klog.Warningf("Ignoring malformed CIDR in allowlist: %q: %v", entry, err)
				continue
			}
			if prefix.Contains(addr) {
				return true, nil
			}
		} else {
			// Try parsing as single IP
			allowedAddr, err := netip.ParseAddr(entry)
			if err != nil {
				klog.Warningf("Ignoring invalid IP in allowlist: %q: %v", entry, err)
				continue
			}
			if allowedAddr.Unmap() == addr {
				return true, nil
			}
		}
	}
	return false, nil
}

// IsAllowedHostname returns true if the hostname matches any allowlist entry
// Only exact matches are allowed for security reasons
func IsAllowedHostname(hostname string, allowlist []string) (bool, error) {
	if len(allowlist) == 0 {
		return false, nil
	}

	hostname = strings.ToLower(strings.TrimSpace(hostname))
	for _, entry := range allowlist {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		// Skip CIDR entries for hostname matching
		if strings.Contains(entry, "/") {
			if _, err := netip.ParsePrefix(entry); err != nil {
				klog.Warningf("Ignoring malformed CIDR in allowlist: %q: %v", entry, err)
			}
			continue
		}
		// Skip IP addresses for hostname matching
		if _, err := netip.ParseAddr(entry); err == nil {
			continue
		}
		// Exact hostname match only (no subdomain matching for security)
		if entry == hostname {
			return true, nil
		}
	}
	return false, nil
}

// ValidateEndpointHost validates that a hostname resolves to allowed IPs
// allowlist entries can be CIDRs (10.96.0.0/12) or hostnames (minio.default.svc)
func ValidateEndpointHost(host string, allowlist []string) error {
	// Parse host:port or just host
	hostname, _, err := net.SplitHostPort(host)
	if err != nil {
		hostname = host
	}

	// Resolve hostname to IPs
	// Note: We always validate resolved IPs against blocklist, even for allowlisted hostnames,
	// to prevent DNS rebinding attacks (e.g., allowlisted hostname → 169.254.169.254)
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve host %q: %w", hostname, err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("host %q resolved to no IPs", hostname)
	}

	// Check each resolved IP against allowlist then blocklist
	// Require ALL IPs to pass validation (not just one)
	for _, ip := range ips {
		ipStr := ip.String()

		// Check allowlist first
		allowed, err := IsAllowedIP(ipStr, allowlist)
		if err != nil {
			return err
		}
		if allowed {
			// This IP is explicitly allowed, continue to next IP
			continue
		}

		// Not in allowlist, check if it's blocked
		blocked, err := IsBlockedIP(ipStr)
		if err != nil {
			return err
		}
		if blocked {
			// This IP is blocked and not in allowlist - fail immediately
			return fmt.Errorf("host %q resolves to blocked IP %s (SSRF protection)", hostname, ipStr)
		}

		// This IP is neither allowed nor blocked (i.e., it's a public IP) - OK, continue to next IP
	}

	// All IPs passed validation (either explicitly allowed or public)
	return nil
}
