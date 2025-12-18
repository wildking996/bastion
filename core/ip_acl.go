package core

import (
	"fmt"
	"net"
	"strings"
)

type IPAccessControl struct {
	allow []*net.IPNet
	deny  []*net.IPNet
}

func NewIPAccessControl(allowCIDRs, denyCIDRs []string) (*IPAccessControl, error) {
	acl := &IPAccessControl{}

	parseAll := func(list []string) ([]*net.IPNet, error) {
		if len(list) == 0 {
			return nil, nil
		}
		out := make([]*net.IPNet, 0, len(list))
		for _, raw := range list {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}

			ip, ipNet, err := parseCIDROrIP(raw)
			if err != nil {
				return nil, err
			}
			if ipNet == nil {
				continue
			}
			// Ensure the IPNet has a valid IP (for Contains checks)
			if ipNet.IP == nil && ip != nil {
				ipNet.IP = ip
			}
			out = append(out, ipNet)
		}
		return out, nil
	}

	var err error
	acl.allow, err = parseAll(allowCIDRs)
	if err != nil {
		return nil, fmt.Errorf("invalid allow_cidrs: %w", err)
	}
	acl.deny, err = parseAll(denyCIDRs)
	if err != nil {
		return nil, fmt.Errorf("invalid deny_cidrs: %w", err)
	}

	if len(acl.allow) == 0 && len(acl.deny) == 0 {
		return nil, nil
	}
	return acl, nil
}

func (a *IPAccessControl) Allows(ip net.IP) bool {
	if a == nil {
		return true
	}
	if ip == nil {
		return false
	}

	for _, n := range a.deny {
		if n != nil && n.Contains(ip) {
			return false
		}
	}

	if len(a.allow) == 0 {
		return true
	}
	for _, n := range a.allow {
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}

func parseCIDROrIP(value string) (net.IP, *net.IPNet, error) {
	if strings.Contains(value, "/") {
		ip, ipNet, err := net.ParseCIDR(value)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid CIDR %q", value)
		}
		return ip, ipNet, nil
	}

	ip := net.ParseIP(value)
	if ip == nil {
		return nil, nil, fmt.Errorf("invalid IP %q", value)
	}

	if ip4 := ip.To4(); ip4 != nil {
		_, ipNet, _ := net.ParseCIDR(ip4.String() + "/32")
		return ip4, ipNet, nil
	}

	_, ipNet, _ := net.ParseCIDR(ip.String() + "/128")
	return ip, ipNet, nil
}
