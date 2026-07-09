package parser

import (
	"fmt"
	"strconv"
	"strings"

	"sub-store/internal/model"
)

type SurgeParser struct{}

func NewSurgeParser() *SurgeParser { return &SurgeParser{} }

func (s *SurgeParser) Test(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "//") {
		return false
	}
	eqIdx := strings.Index(trimmed, "=")
	if eqIdx < 0 {
		return false
	}
	afterEq := strings.TrimSpace(trimmed[eqIdx+1:])
	firstToken := strings.ToLower(strings.SplitN(afterEq, ",", 2)[0])
	surgeTypes := map[string]bool{
		"ss": true, "vmess": true, "trojan": true, "http": true, "https": true,
		"snell": true, "socks5": true, "socks5-tls": true, "tuic": true, "tuic-v5": true,
		"wireguard": true, "hysteria2": true, "ssh": true, "h2-connect": true,
		"anytls": true, "trust-tunnel": true, "direct": true, "external": true,
	}
	return surgeTypes[firstToken]
}

func (s *SurgeParser) Parse(line string) (*model.Proxy, error) {
	trimmed := strings.TrimSpace(line)
	eqIdx := strings.Index(trimmed, "=")
	if eqIdx < 0 {
		return nil, fmt.Errorf("surge: no = found")
	}
	name := strings.TrimSpace(trimmed[:eqIdx])
	rest := strings.TrimSpace(trimmed[eqIdx+1:])

	parts := splitSurgeLine(rest)
	if len(parts) < 3 {
		return nil, fmt.Errorf("surge: too few parts")
	}

	typeStr := strings.ToLower(strings.TrimSpace(parts[0]))
	server := strings.TrimSpace(parts[1])
	portStr := strings.TrimSpace(parts[2])
	port, _ := strconv.Atoi(portStr)

	proxy := &model.Proxy{
		Name:   name,
		Server: server,
		Port:   port,
	}

	kvPairs := make(map[string]string)
	positionalArgs := []string{}
	for i := 3; i < len(parts); i++ {
		p := strings.TrimSpace(parts[i])
		if eq := strings.Index(p, "="); eq > 0 {
			key := strings.TrimSpace(p[:eq])
			val := strings.TrimSpace(p[eq+1:])
			val = unquote(val)
			kvPairs[strings.ToLower(key)] = val
		} else {
			positionalArgs = append(positionalArgs, p)
		}
	}

	switch typeStr {
	case "ss":
		proxy.Type = "ss"
		if v, ok := kvPairs["encrypt-method"]; ok {
			proxy.Cipher = v
		}
		if v, ok := kvPairs["password"]; ok {
			proxy.Password = v
		}
	case "vmess":
		proxy.Type = "vmess"
		if v, ok := kvPairs["username"]; ok {
			proxy.UUID = v
		}
		proxy.Network = "tcp"
		if _, ok := kvPairs["ws"]; ok {
			proxy.Network = "ws"
		}
		proxy.WSOpts = make(map[string]interface{})
		if v, ok := kvPairs["ws-path"]; ok {
			proxy.WSOpts["path"] = v
		}
		if v, ok := kvPairs["ws-headers"]; ok {
			headers := parseSurgeHeaders(v, "|")
			if len(headers) > 0 {
				proxy.WSOpts["headers"] = headers
			}
		}
		if v, ok := kvPairs["vmess-aead"]; ok && v == "true" {
			proxy.AEAD = true
		}
	case "trojan":
		proxy.Type = "trojan"
		if v, ok := kvPairs["password"]; ok {
			proxy.Password = v
		}
		proxy.TLS = true
	case "http", "https":
		proxy.Type = "http"
		if typeStr == "https" {
			proxy.TLS = true
		}
		if len(positionalArgs) >= 1 {
			proxy.Username = positionalArgs[0]
		}
		if len(positionalArgs) >= 2 {
			proxy.Password = positionalArgs[1]
		}
		if v, ok := kvPairs["username"]; ok {
			proxy.Username = v
		}
		if v, ok := kvPairs["password"]; ok {
			proxy.Password = v
		}
	case "snell":
		proxy.Type = "snell"
		if v, ok := kvPairs["psk"]; ok {
			proxy.PSK = v
		}
		if v, ok := kvPairs["version"]; ok {
			proxy.Version, _ = strconv.Atoi(v)
		}
		if v, ok := kvPairs["obfs"]; ok {
			proxy.Obfs = v
		}
		if v, ok := kvPairs["obfs-host"]; ok {
			proxy.ObfsParam = v
		}
	case "socks5", "socks5-tls":
		proxy.Type = "socks5"
		if typeStr == "socks5-tls" {
			proxy.TLS = true
		}
		if len(positionalArgs) >= 1 {
			proxy.Username = positionalArgs[0]
		}
		if len(positionalArgs) >= 2 {
			proxy.Password = positionalArgs[1]
		}
		if v, ok := kvPairs["username"]; ok {
			proxy.Username = v
		}
		if v, ok := kvPairs["password"]; ok {
			proxy.Password = v
		}
	case "tuic", "tuic-v5":
		proxy.Type = "tuic"
		if typeStr == "tuic-v5" {
			proxy.Version = 5
		}
		if v, ok := kvPairs["password"]; ok {
			proxy.Password = v
		}
		if v, ok := kvPairs["uuid"]; ok {
			proxy.UUID = v
		}
		proxy.TLS = true
	case "wireguard":
		proxy.Type = "wireguard-surge"
		if v, ok := kvPairs["private-key"]; ok {
			proxy.PrivateKey = v
		}
		if v, ok := kvPairs["self-ip"]; ok {
			proxy.IP = v
		}
		if v, ok := kvPairs["self-ip-v6"]; ok {
			proxy.IPv6 = v
		}
		if v, ok := kvPairs["dns-server"]; ok {
			proxy.DNS = strings.Split(v, ",")
		}
		if v, ok := kvPairs["mtu"]; ok {
			proxy.MTU, _ = strconv.Atoi(v)
		}
	case "hysteria2":
		proxy.Type = "hysteria2"
		if v, ok := kvPairs["password"]; ok {
			proxy.Password = v
		}
		proxy.TLS = true
		if v, ok := kvPairs["port-hopping"]; ok {
			proxy.Ports = v
		}
		if v, ok := kvPairs["port-hopping-interval"]; ok {
			proxy.HopInterval, _ = strconv.Atoi(v)
		}
	case "ssh":
		proxy.Type = "ssh"
		if len(positionalArgs) >= 1 {
			proxy.Username = positionalArgs[0]
		}
		if len(positionalArgs) >= 2 {
			proxy.Password = positionalArgs[1]
		}
		if v, ok := kvPairs["username"]; ok {
			proxy.Username = v
		}
		if v, ok := kvPairs["password"]; ok {
			proxy.Password = v
		}
	case "h2-connect":
		proxy.Type = "h2-connect"
		if v, ok := kvPairs["password"]; ok {
			proxy.Password = v
		}
		proxy.TLS = true
	case "anytls":
		proxy.Type = "anytls"
		if v, ok := kvPairs["password"]; ok {
			proxy.Password = v
		}
		proxy.TLS = true
	case "trust-tunnel":
		proxy.Type = "trusttunnel"
		if v, ok := kvPairs["password"]; ok {
			proxy.Password = v
		}
		proxy.TLS = true
	case "direct":
		proxy.Type = "direct"
	case "external":
		proxy.Type = "external"
	default:
		return nil, fmt.Errorf("surge: unsupported type %s", typeStr)
	}

	// Common options
	if v, ok := kvPairs["tls"]; ok && v == "true" {
		proxy.TLS = true
	}
	if v, ok := kvPairs["sni"]; ok {
		proxy.SNI = v
	}
	if v, ok := kvPairs["skip-cert-verify"]; ok && v == "true" {
		proxy.SkipCertVerify = true
	}
	if v, ok := kvPairs["tfo"]; ok && v == "true" {
		proxy.TCPFastOpen = true
	}
	if v, ok := kvPairs["udp-relay"]; ok && v == "true" {
		proxy.UDP = true
	}
	if v, ok := kvPairs["underlying-proxy"]; ok {
		proxy.UnderlyingProxy = v
	}
	if v, ok := kvPairs["alpn"]; ok {
		proxy.ALPN = strings.Split(v, ",")
	}
	if v, ok := kvPairs["server-cert-fingerprint"]; ok {
		proxy.TLSFingerprint = v
	}

	return proxy, nil
}

func splitSurgeLine(s string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' && (i == 0 || s[i-1] != '\\') {
			inQuote = !inQuote
			current.WriteByte(ch)
		} else if ch == ',' && !inQuote {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func parseSurgeHeaders(s string, separator string) map[string]interface{} {
	s = unquote(s)
	headers := make(map[string]interface{})
	pairs := strings.Split(s, separator)
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if idx := strings.Index(pair, ":"); idx > 0 {
			key := strings.TrimSpace(pair[:idx])
			val := strings.TrimSpace(pair[idx+1:])
			headers[key] = val
		}
	}
	return headers
}
