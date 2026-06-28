// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

package custom

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// proxyFromEnvironmentWithWildcard is like http.ProxyFromEnvironment but
// supports wildcard patterns in NO_PROXY (e.g. *.huawei.com, 10.*).
func proxyFromEnvironmentWithWildcard(req *http.Request) (*url.URL, error) {
	noProxy := os.Getenv("NO_PROXY")
	if noProxy == "" {
		noProxy = os.Getenv("no_proxy")
	}
	if noProxy != "" {
		host := req.URL.Hostname()
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if matchNoProxyWildcard(host, noProxy) {
			return nil, nil
		}
	}
	return http.ProxyFromEnvironment(req)
}

// matchNoProxyWildcard checks if host matches any NO_PROXY entry with wildcard support.
// Supports: exact match, *.domain.com (suffix), 10.* (prefix)
func matchNoProxyWildcard(host, noProxy string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, entry := range strings.Split(noProxy, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		// Exact match
		if entry == host {
			return true
		}
		// Suffix wildcard: *.domain.com
		if strings.HasPrefix(entry, "*.") {
			suffix := entry[1:] // .domain.com
			if strings.HasSuffix(host, suffix) || host == entry[2:] {
				return true
			}
		}
		// Prefix wildcard: 10.*
		if strings.HasSuffix(entry, ".*") {
			prefix := entry[:len(entry)-1] // 10.
			if strings.HasPrefix(host, prefix) {
				return true
			}
		}
	}
	return false
}
