package main

import (
	"fmt"
	"strings"
)

// NormalizeProxy applies all normalizations to a parsed proxy (equivalent to lastParse).
func NormalizeProxy(p *Proxy) *Proxy {
	if p == nil {
		return nil
	}

	// Lowercase cipher
	if p.Cipher != "" {
		p.Cipher = strings.ToLower(p.Cipher)
	}

	// Handle hop-interval range splitting (e.g., "10-20")
	if p.HopInterval > 0 {
		// If already parsed as int, keep it. If originally string ranged, it should be handled during parsing.
		// This is a no-op for already-converted ints.
	}

	// SS cipher=none + no password -> password: ""
	if p.Type == "ss" && p.Cipher == "none" && p.Password == "" {
		p.Password = ""
	}

	// interface -> interface-name
	if p.InterfaceName != "" && p.UnderlyingProxy == "" {
		// Only if no dialer-proxy is set
	}

	// Remove brackets from IPv6 server addresses
	if p.Server != "" {
		p.Server = strings.TrimPrefix(strings.TrimSpace(p.Server), "[")
		p.Server = strings.TrimSuffix(p.Server, "]")
	}

	// WS old field migration (ws-path/ws-headers -> ws-opts)
	if p.Network == "ws" || p.WSOpts != nil {
		if p.WSOpts == nil {
			p.WSOpts = make(map[string]interface{})
		}
		// If path was stored in p.Path, migrate it
		if p.Path != "" && p.WSOpts["path"] == nil {
			p.WSOpts["path"] = p.Path
		}
	}

	// Ensure path starts with /
	normalizeTransportPath(p)

	// Default network for vmess/vless/trojan -> tcp
	if (p.Type == "vmess" || p.Type == "vless" || p.Type == "trojan") && p.Network == "" {
		p.Network = "tcp"
	}

	// Force TLS for specific proxy types
	forceTLSTypes := map[string]bool{
		"trojan": true, "tuic": true, "hysteria": true, "hysteria2": true,
		"juicity": true, "anytls": true, "trusttunnel": true,
		"h2-connect": true, "naive": true,
	}
	if forceTLSTypes[p.Type] {
		p.TLS = true
	}

	// Auto-infer SNI from transport host or server (if not IP)
	if p.TLS && p.SNI == "" && p.SNI != "off" {
		// Try transport host first
		if host := getTransportHost(p); host != "" {
			p.SNI = host
		} else if !isIP(p.Server) {
			p.SNI = p.Server
		}
	}

	// Hysteria2 obfs normalization
	if p.Type == "hysteria2" && p.Obfs != "" && p.ObfsPassword == "" {
		if p.Obfs != "salamander" {
			p.ObfsPassword = p.Obfs
			p.Obfs = "salamander"
		}
	}
	if p.Type == "hysteria2" && p.ObfsPassword == "" {
		// Check alternate field
		if p.Extra != nil {
			if v, ok := p.Extra["obfs_password"]; ok {
				p.ObfsPassword = fmt.Sprintf("%v", v)
				delete(p.Extra, "obfs_password")
			}
		}
	}

	// VLESS flow/reality cleanup
	if p.Type == "vless" {
		if p.RealityOpts != nil && len(p.RealityOpts) == 0 {
			p.RealityOpts = nil
		}
		if p.GRPCOpts != nil && len(p.GRPCOpts) == 0 {
			p.GRPCOpts = nil
		}
		// Delete empty flow
		if (p.RealityOpts == nil && p.Flow == "") || p.Flow == "null" {
			p.Flow = ""
		}
	}

	// TUIC defaults
	if p.Type == "tuic" {
		if len(p.ALPN) == 0 {
			p.ALPN = []string{"h3"}
		}
		if p.CongestionController == "" {
			p.CongestionController = "cubic"
		}
		if p.UDPRelayMode == "" {
			p.UDPRelayMode = "native"
		}
	}

	// WireGuard interface normalization
	if p.Type == "wireguard" {
		if p.InterfaceName == "" && p.Name != "" {
			p.InterfaceName = p.Name
		}
		if len(p.Peers) > 0 {
			peer := p.Peers[0]
			if p.IP == "" && peer["ip"] != nil {
				p.IP = fmt.Sprintf("%v", peer["ip"])
			}
			if p.IPv6 == "" && peer["ipv6"] != nil {
				p.IPv6 = fmt.Sprintf("%v", peer["ipv6"])
			}
			if p.PublicKey == "" && peer["public-key"] != nil {
				p.PublicKey = fmt.Sprintf("%v", peer["public-key"])
			}
			if p.PreSharedKey == "" && peer["pre-shared-key"] != nil {
				p.PreSharedKey = fmt.Sprintf("%v", peer["pre-shared-key"])
			}
		}
	}

	// Handle ports format normalization
	if p.Ports != "" {
		p.Ports = strings.ReplaceAll(p.Ports, "/", ",")
	}

	// Ensure password is string
	if p.Password != "" {
		// Ensure it's a string, not a number
		p.Password = fmt.Sprintf("%v", p.Password)
	}

	// Default vmess cipher to none and alterId to 0
	if p.Type == "vmess" {
		if p.Cipher == "" {
			p.Cipher = "none"
		}
		if p.AlterID == 0 {
			p.AlterID = 0
		}
	}

	// Ensure http transport path for vmess/vless
	if (p.Type == "vmess" || p.Type == "vless") && p.Network == "http" {
		if p.HTTPOpts == nil {
			p.HTTPOpts = make(map[string]interface{})
		}
		if p.HTTPOpts["path"] == nil {
			p.HTTPOpts["path"] = []string{"/"}
		}
	}

	// Handle disable-sni -> sni = "off"
	if p.SNI == "" || p.SNI == "off" {
		if p.DisableSNI {
			p.SNI = "off"
		}
	}

	// Handle name as string
	if p.Name == "" {
		p.Name = fmt.Sprintf("%s %s:%d", p.Type, p.Server, p.Port)
	}

	// Normalize client-fingerprint / tls-fingerprint
	if p.ClientFingerprint != "" && p.TLSFingerprint == "" {
		p.TLSFingerprint = p.ClientFingerprint
	}

	return p
}

// normalizeTransportPath ensures transport paths start with "/".
func normalizeTransportPath(p *Proxy) {
	networks := []string{"ws", "http", "h2"}
	for _, net := range networks {
		var opts map[string]interface{}
		switch net {
		case "ws":
			opts = p.WSOpts
		case "http":
			opts = p.HTTPOpts
		case "h2":
			opts = p.H2Opts
		}
		if opts == nil {
			continue
		}
		pathVal, ok := opts["path"]
		if !ok {
			continue
		}
		switch v := pathVal.(type) {
		case string:
			opts["path"] = formatPath(v)
		case []interface{}:
			for i, item := range v {
				if s, ok := item.(string); ok {
					v[i] = formatPath(s)
				}
			}
		case []string:
			for i, s := range v {
				v[i] = formatPath(s)
			}
		}
	}
}

func formatPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

// getTransportHost extracts the Host header from transport options.
func getTransportHost(p *Proxy) string {
	var headers map[string]interface{}
	switch p.Network {
	case "ws":
		if p.WSOpts != nil {
			if h, ok := p.WSOpts["headers"].(map[string]interface{}); ok {
				headers = h
			}
		}
	case "http":
		if p.HTTPOpts != nil {
			if h, ok := p.HTTPOpts["headers"].(map[string]interface{}); ok {
				headers = h
			}
		}
	case "h2":
		if p.H2Opts != nil {
			if hosts, ok := p.H2Opts["host"]; ok {
				switch v := hosts.(type) {
				case string:
					return v
				case []interface{}:
					if len(v) > 0 {
						return fmt.Sprintf("%v", v[0])
					}
				case []string:
					if len(v) > 0 {
						return v[0]
					}
				}
			}
		}
	}
	if headers != nil {
		if host, ok := headers["Host"]; ok {
			return fmt.Sprintf("%v", host)
		}
		if host, ok := headers["host"]; ok {
			return fmt.Sprintf("%v", host)
		}
	}
	return ""
}

// NOTE: isIP, isIPv4, isIPv6 are defined in models.go
