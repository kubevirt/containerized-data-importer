package net_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	utilnet "kubevirt.io/containerized-data-importer/pkg/util/net"
)

var _ = Describe("IP Blocklist and Allowlist", func() {
	DescribeTable("IsBlockedIP should",
		func(ip string, expectBlocked bool, expectError bool) {
			blocked, err := utilnet.IsBlockedIP(ip)
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(blocked).To(Equal(expectBlocked))
			}
		},
		// Link-local (IMDS)
		Entry("block 169.254.169.254", "169.254.169.254", true, false),
		Entry("block 169.254.0.1", "169.254.0.1", true, false),

		// RFC 1918 private
		Entry("block 10.0.0.1", "10.0.0.1", true, false),
		Entry("block 172.16.0.1", "172.16.0.1", true, false),
		Entry("block 192.168.1.1", "192.168.1.1", true, false),

		// Loopback
		Entry("block 127.0.0.1", "127.0.0.1", true, false),
		Entry("block ::1", "::1", true, false),

		// Public IPs (allowed)
		Entry("allow 1.1.1.1", "1.1.1.1", false, false),
		Entry("allow 8.8.8.8", "8.8.8.8", false, false),
		Entry("allow 2606:4700:4700::1111", "2606:4700:4700::1111", false, false),

		// Invalid
		Entry("error on invalid IP", "not-an-ip", false, true),
	)

	DescribeTable("IsAllowedIP should",
		func(ip string, allowlist []string, expectAllowed bool, expectError bool) {
			allowed, err := utilnet.IsAllowedIP(ip, allowlist)
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(Equal(expectAllowed))
			}
		},
		// CIDR matching
		Entry("match single IP in CIDR", "10.96.1.1", []string{"10.96.0.0/12"}, true, false),
		Entry("match edge of CIDR", "10.96.0.1", []string{"10.96.0.0/12"}, true, false),
		Entry("not match outside CIDR", "10.95.0.1", []string{"10.96.0.0/12"}, false, false),

		// Exact IP matching
		Entry("match exact IP", "192.168.1.100", []string{"192.168.1.100"}, true, false),
		Entry("not match different IP", "192.168.1.101", []string{"192.168.1.100"}, false, false),

		// Multiple entries
		Entry("match first entry", "10.0.0.1", []string{"10.0.0.0/8", "192.168.0.0/16"}, true, false),
		Entry("match second entry", "192.168.1.1", []string{"10.0.0.0/8", "192.168.0.0/16"}, true, false),

		// Empty allowlist
		Entry("empty allowlist", "10.0.0.1", []string{}, false, false),
		Entry("nil allowlist", "10.0.0.1", nil, false, false),
	)

	DescribeTable("IsAllowedHostname should",
		func(hostname string, allowlist []string, expectAllowed bool) {
			allowed, err := utilnet.IsAllowedHostname(hostname, allowlist)
			Expect(err).NotTo(HaveOccurred())
			Expect(allowed).To(Equal(expectAllowed))
		},
		// Exact hostname match
		Entry("match exact hostname", "minio.default.svc", []string{"minio.default.svc"}, true),
		Entry("not match different hostname", "redis.default.svc", []string{"minio.default.svc"}, false),

		// Subdomain matching removed for security (exact match only)
		Entry("not match subdomain", "api.minio.default.svc", []string{"minio.default.svc"}, false),
		Entry("not match deep subdomain", "v1.api.minio.default.svc", []string{"minio.default.svc"}, false),

		// Case insensitive
		Entry("match case insensitive", "MINIO.DEFAULT.SVC", []string{"minio.default.svc"}, true),

		// Empty allowlist
		Entry("empty allowlist", "minio.default.svc", []string{}, false),
	)

	DescribeTable("ValidateEndpointHost should",
		func(host string, allowlist []string, expectError bool) {
			err := utilnet.ValidateEndpointHost(host, allowlist)
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		// Public hosts (no allowlist needed)
		Entry("allow public cloudflare.com", "cloudflare.com", nil, false),
		Entry("allow public google.com:443", "google.com:443", nil, false),

		// Blocked IPs without allowlist
		Entry("block 169.254.169.254", "169.254.169.254", nil, true),
		Entry("block 10.0.0.1", "10.0.0.1", nil, true),

		// Blocked IPs with allowlist
		Entry("allow 10.96.1.1 via CIDR allowlist", "10.96.1.1", []string{"10.96.0.0/12"}, false),
		Entry("allow 192.168.1.100 via exact IP", "192.168.1.100", []string{"192.168.1.100"}, false),

		// DNS rebinding protection: hostname allowlist doesn't bypass IP blocklist
		Entry("block localhost even with hostname allowlist", "localhost", []string{"localhost"}, true),
		Entry("allow localhost with combined hostname+IP allowlist", "localhost", []string{"localhost", "127.0.0.0/8", "::1/128"}, false),
	)

	Context("IPv4-mapped IPv6 SSRF bypass prevention", func() {
		DescribeTable("IsBlockedIP should block IPv4-mapped IPv6 addresses",
			func(ip string, expectBlocked bool) {
				blocked, err := utilnet.IsBlockedIP(ip)
				Expect(err).NotTo(HaveOccurred())
				Expect(blocked).To(Equal(expectBlocked))
			},
			// AWS/Azure IMDS bypass attempts
			Entry("block ::ffff:169.254.169.254 (AWS IMDS)", "::ffff:169.254.169.254", true),
			Entry("block ::ffff:169.254.0.1 (link-local)", "::ffff:169.254.0.1", true),

			// RFC 1918 private IP bypass attempts
			Entry("block ::ffff:10.0.0.1 (private 10.x)", "::ffff:10.0.0.1", true),
			Entry("block ::ffff:172.16.0.1 (private 172.16.x)", "::ffff:172.16.0.1", true),
			Entry("block ::ffff:192.168.1.1 (private 192.168.x)", "::ffff:192.168.1.1", true),

			// CGNAT bypass attempt
			Entry("block ::ffff:100.64.0.1 (CGNAT)", "::ffff:100.64.0.1", true),

			// Loopback bypass attempts
			Entry("block ::ffff:127.0.0.1 (loopback)", "::ffff:127.0.0.1", true),
			Entry("block ::ffff:127.1.1.1 (loopback)", "::ffff:127.1.1.1", true),

			// Public IPs as IPv4-mapped should be allowed
			Entry("allow ::ffff:1.1.1.1 (public)", "::ffff:1.1.1.1", false),
			Entry("allow ::ffff:8.8.8.8 (public)", "::ffff:8.8.8.8", false),
		)

		DescribeTable("IsAllowedIP should handle IPv4-mapped IPv6 addresses",
			func(ip string, allowlist []string, expectAllowed bool) {
				allowed, err := utilnet.IsAllowedIP(ip, allowlist)
				Expect(err).NotTo(HaveOccurred())
				Expect(allowed).To(Equal(expectAllowed))
			},
			// IPv4-mapped IPv6 should match IPv4 CIDR allowlist
			Entry("match ::ffff:10.96.1.1 against 10.96.0.0/12", "::ffff:10.96.1.1", []string{"10.96.0.0/12"}, true),
			Entry("match ::ffff:192.168.1.100 against exact IPv4", "::ffff:192.168.1.100", []string{"192.168.1.100"}, true),

			// IPv4-mapped IPv6 should NOT match if outside allowlist
			Entry("not match ::ffff:10.95.0.1 against 10.96.0.0/12", "::ffff:10.95.0.1", []string{"10.96.0.0/12"}, false),

			// Regular IPv4 should match IPv4-mapped allowlist entry
			Entry("match 192.168.1.100 against ::ffff:192.168.1.100", "192.168.1.100", []string{"::ffff:192.168.1.100"}, true),
		)

		DescribeTable("ValidateEndpointHost should block IPv4-mapped IPv6 literals",
			func(host string, allowlist []string, expectError bool) {
				err := utilnet.ValidateEndpointHost(host, allowlist)
				if expectError {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("SSRF protection"))
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			},
			// Should block IPv4-mapped IPv6 IMDS bypass without allowlist
			Entry("block ::ffff:169.254.169.254 without allowlist", "::ffff:169.254.169.254", nil, true),
			Entry("block ::ffff:10.0.0.1 without allowlist", "::ffff:10.0.0.1", nil, true),

			// Should allow with appropriate allowlist
			Entry("allow ::ffff:10.96.1.1 with CIDR allowlist", "::ffff:10.96.1.1", []string{"10.96.0.0/12"}, false),
		)
	})
})
