package parser

import (
	"fmt"
	"strconv"
	"strings"

	"sub-store/internal/model"
)

type LoonParser struct{}

func NewLoonParser() *LoonParser { return &LoonParser{} }

func (l *LoonParser) Test(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "//") {
		return false
	}
	eqIdx := strings.Index(trimmed, "=")
	if eqIdx < 0 {
		return false
	}
	afterEq := strings.TrimSpace(trimmed[eqIdx+1:])
	firstToken := strings.ToLower(strings.TrimSpace(splitFirst(afterEq, ",")))
	loonTypes := map[string]bool{
		"shadowsocks": true, "shadowsocksr": true, "vmess": true, "vless": true,
		"trojan": true, "hysteria2": true, "anytls": true,
		"http": true, "https": true, "socks5": true, "wireguard": true,
	}
	if !loonTypes[firstToken] {
		return false
	}
	if firstToken == "vmess" {
		if strings.Contains(strings.ToLower(trimmed), "username=") {
			return false
		}
	}
	return true
}

func (l *LoonParser) Parse(line string) (*model.Proxy, error) {
	trimmed := strings.TrimSpace(line)
	eqIdx := strings.Index(trimmed, "=")
	if eqIdx < 0 {
		return nil, fmt.Errorf("loon: no = found")
	}
	name := strings.TrimSpace(trimmed[:eqIdx])
	rest := strings.TrimSpace(trimmed[eqIdx+1:])

	parts := splitSurgeLine(rest)
	if len(parts) < 3 {
		return nil, fmt.Errorf("loon: too few parts")
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
	var positionalArgs []string
	for i := 3; i < len(parts); i++ {
		p := strings.TrimSpace(parts[i])
		if eq := strings.Index(p, "="); eq > 0 {
			key := strings.TrimSpace(p[:eq])
			val := strings.TrimSpace(p[eq+1:])
			val = unquote(val)
			kvPairs[strings.ToLower(key)] = val
		} else {
			positionalArgs = append(positionalArgs, unquote(p))
		}
	}

	switch typeStr {
	case "shadowsocks":
		proxy.Type = "ss"
		if len(positionalArgs) >= 1 {
			proxy.Cipher = positionalArgs[0]
		}
		if len(positionalArgs) >= 2 {
			proxy.Password = positionalArgs[1]
		}
		parseLoonSSObfs(proxy, kvPairs)
		parseLoonShadowTLS(proxy, kvPairs)

	case "shadowsocksr":
		proxy.Type = "ssr"
		if len(positionalArgs) >= 1 {
			proxy.Cipher = positionalArgs[0]
		}
		if len(positionalArgs) >= 2 {
			proxy.Password = positionalArgs[1]
		}
		if v, ok := kvPairs["protocol"]; ok {
			proxy.Protocol = v
		}
		if v, ok := kvPairs["protocol-param"]; ok {
			proxy.ProtocolParam = v
		}
		if v, ok := kvPairs["obfs"]; ok {
			proxy.Obfs = v
		}
		if v, ok := kvPairs["obfs-param"]; ok {
			proxy.ObfsParam = v
		}
		parseLoonShadowTLS(proxy, kvPairs)

	case "vmess":
		proxy.Type = "vmess"
		if len(positionalArgs) >= 1 {
			proxy.Cipher = normalizeLoonVmessSecurity(positionalArgs[0])
		} else {
			proxy.Cipher = "auto"
		}
		if len(positionalArgs) >= 2 {
			proxy.UUID = positionalArgs[1]
		}
		proxy.AlterID = 0
		parseLoonTransport(proxy, kvPairs)
		parseLoonTLS(proxy, kvPairs)
		parseLoonReality(proxy, kvPairs)
		parseLoonShadowTLS(proxy, kvPairs)

	case "vless":
		proxy.Type = "vless"
		if len(positionalArgs) >= 1 {
			proxy.UUID = positionalArgs[0]
		}
		parseLoonTransport(proxy, kvPairs)
		parseLoonTLS(proxy, kvPairs)
		parseLoonReality(proxy, kvPairs)
		if v, ok := kvPairs["flow"]; ok {
			proxy.Flow = v
		}

	case "trojan":
		proxy.Type = "trojan"
		if len(positionalArgs) >= 1 {
			proxy.Password = positionalArgs[0]
		}
		proxy.TLS = true
		parseLoonTransport(proxy, kvPairs)
		parseLoonTLS(proxy, kvPairs)
		parseLoonReality(proxy, kvPairs)

	case "hysteria2":
		proxy.Type = "hysteria2"
		if len(positionalArgs) >= 1 {
			proxy.Password = positionalArgs[0]
		}
		proxy.TLS = true
		parseLoonTLS(proxy, kvPairs)
		if v, ok := kvPairs["download-bandwidth"]; ok {
			proxy.Down = v
		}
		if v, ok := kvPairs["server-ports"]; ok {
			proxy.Ports = v
		}
		if v, ok := kvPairs["hop-interval"]; ok {
			proxy.HopInterval, _ = strconv.Atoi(v)
		}
		if v, ok := kvPairs["salamander-password"]; ok {
			proxy.Obfs = "salamander"
			proxy.ObfsParam = v
		}
		if v, ok := kvPairs["ecn"]; ok && v == "true" {
			proxy.ECN = true
		}
		if v, ok := kvPairs["block-quic"]; ok {
			proxy.BlockQUIC = v
		}

	case "anytls":
		proxy.Type = "anytls"
		if len(positionalArgs) >= 1 {
			proxy.Password = positionalArgs[0]
		}
		proxy.TLS = true
		parseLoonTLS(proxy, kvPairs)
		parseLoonReality(proxy, kvPairs)
		if v, ok := kvPairs["idle-session-timeout"]; ok {
			proxy.IdleSessionTimeout, _ = strconv.Atoi(v)
		}
		if v, ok := kvPairs["max-stream-count"]; ok {
			proxy.MaxStreamCount, _ = strconv.Atoi(v)
		}

	case "http", "https":
		proxy.Type = "http"
		if typeStr == "https" {
			proxy.TLS = true
		}
		if v, ok := kvPairs["username"]; ok {
			proxy.Username = v
		}
		if v, ok := kvPairs["password"]; ok {
			proxy.Password = v
		}
		parseLoonTLS(proxy, kvPairs)

	case "socks5":
		proxy.Type = "socks5"
		if v, ok := kvPairs["username"]; ok {
			proxy.Username = v
		}
		if v, ok := kvPairs["password"]; ok {
			proxy.Password = v
		}
		parseLoonTLS(proxy, kvPairs)

	case "wireguard":
		return parseLoonWireGuard(name, rest)

	default:
		return nil, fmt.Errorf("loon: unsupported type %s", typeStr)
	}

	parseLoonCommon(proxy, kvPairs)
	return proxy, nil
}

func parseLoonSSObfs(proxy *model.Proxy, kv map[string]string) {
	if v, ok := kv["obfs-name"]; ok {
		proxy.Plugin = "obfs-local"
		opts := map[string]interface{}{"obfs": v}
		if h, ok := kv["obfs-host"]; ok {
			opts["obfs-host"] = h
		}
		proxy.PluginOpts = opts
		return
	}
	if v, ok := kv["obfs"]; ok {
		switch v {
		case "http", "tls":
			proxy.Plugin = "obfs-local"
			opts := map[string]interface{}{"obfs": v}
			if h, ok := kv["obfs-host"]; ok {
				opts["obfs-host"] = h
			}
			proxy.PluginOpts = opts
		}
	}
}

func parseLoonTransport(proxy *model.Proxy, kv map[string]string) {
	transport, ok := kv["transport"]
	if !ok {
		return
	}
	switch strings.ToLower(transport) {
	case "tcp":
		proxy.Network = "tcp"
	case "ws":
		proxy.Network = "ws"
		proxy.WSOpts = make(map[string]interface{})
		if v, ok := kv["ws-path"]; ok {
			proxy.WSOpts["path"] = v
		}
		if v, ok := kv["ws-host"]; ok {
			proxy.WSOpts["headers"] = map[string]interface{}{"Host": v}
		}
	case "http":
		proxy.Network = "http"
		proxy.HTTPOpts = make(map[string]interface{})
		if v, ok := kv["http-path"]; ok {
			proxy.HTTPOpts["path"] = v
		}
		if v, ok := kv["http-host"]; ok {
			proxy.HTTPOpts["headers"] = map[string]interface{}{"Host": v}
		}
	}
}

func parseLoonTLS(proxy *model.Proxy, kv map[string]string) {
	if v, ok := kv["over-tls"]; ok && v == "true" {
		proxy.TLS = true
	}
	if v, ok := kv["tls-name"]; ok && v != "" {
		proxy.SNI = v
	}
	if v, ok := kv["sni"]; ok && v != "" {
		proxy.SNI = v
	}
	if v, ok := kv["skip-cert-verify"]; ok {
		proxy.SkipCertVerify = v == "true"
	}
	if v, ok := kv["tls-verification"]; ok {
		proxy.SkipCertVerify = v == "false"
	}
	if v, ok := kv["tls-cert-sha256"]; ok {
		proxy.TLSFingerprint = v
	}
	if v, ok := kv["alpn"]; ok && v != "" {
		proxy.ALPN = strings.Split(v, ",")
	}
	if v, ok := kv["tls-profile"]; ok {
		proxy.ClientFingerprint = loonClientFingerprint(v)
	}
}

func parseLoonReality(proxy *model.Proxy, kv map[string]string) {
	pubKey, hasPub := kv["public-key"]
	shortID, hasShort := kv["short-id"]
	if !hasPub && !hasShort {
		return
	}
	if proxy.RealityOpts == nil {
		proxy.RealityOpts = make(map[string]interface{})
	}
	if hasPub {
		proxy.RealityOpts["public-key"] = pubKey
	}
	if hasShort {
		proxy.RealityOpts["short-id"] = shortID
	}
}

func parseLoonShadowTLS(proxy *model.Proxy, kv map[string]string) {
	version, ok := kv["shadow-tls-version"]
	if !ok {
		return
	}
	v, _ := strconv.Atoi(version)
	if v < 2 {
		return
	}
	proxy.Plugin = "shadow-tls"
	opts := map[string]interface{}{"version": version}
	if sni, ok := kv["shadow-tls-sni"]; ok {
		opts["sni"] = sni
	}
	if pwd, ok := kv["shadow-tls-password"]; ok {
		opts["password"] = pwd
	}
	proxy.PluginOpts = opts
}

func parseLoonCommon(proxy *model.Proxy, kv map[string]string) {
	if v, ok := kv["fast-open"]; ok && v == "true" {
		proxy.TCPFastOpen = true
	}
	if v, ok := kv["udp"]; ok && v == "true" {
		proxy.UDP = true
	}
	if v, ok := kv["ip-mode"]; ok {
		proxy.IPVersion = v
	}
	if v, ok := kv["underlying-proxy"]; ok {
		proxy.UnderlyingProxy = v
	}
	if v, ok := kv["udp-over-tcp"]; ok && v == "true" {
		proxy.UDPOverTCP = true
	}
}

func parseLoonWireGuard(name, rest string) (*model.Proxy, error) {
	proxy := &model.Proxy{
		Name: name,
		Type: "wireguard",
	}

	kvPairs := make(map[string]string)
	parts := splitSurgeLine(rest)
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if eq := strings.Index(p, "="); eq > 0 {
			key := strings.TrimSpace(p[:eq])
			val := strings.TrimSpace(p[eq+1:])
			val = unquote(val)
			kvPairs[strings.ToLower(key)] = val
		}
	}

	if v, ok := kvPairs["private-key"]; ok {
		proxy.PrivateKey = v
	}
	if v, ok := kvPairs["interface-ip"]; ok {
		proxy.IP = v
	}
	if v, ok := kvPairs["interface-ipv6"]; ok {
		proxy.IPv6 = v
	}
	if v, ok := kvPairs["mtu"]; ok {
		proxy.MTU, _ = strconv.Atoi(v)
	}
	if v, ok := kvPairs["keepalive"]; ok {
		proxy.KeepAlive, _ = strconv.Atoi(v)
	}
	if v, ok := kvPairs["dns"]; ok {
		proxy.DNS = strings.Split(v, ",")
	}

	if v, ok := kvPairs["peers"]; ok {
		parseLoonWGPeers(proxy, v)
	}

	return proxy, nil
}

func parseLoonWGPeers(proxy *model.Proxy, peersStr string) {
	peersStr = strings.TrimSpace(peersStr)
	if strings.HasPrefix(peersStr, "[") {
		peersStr = peersStr[1:]
	}
	if strings.HasSuffix(peersStr, "]") {
		peersStr = peersStr[:len(peersStr)-1]
	}
	if strings.HasPrefix(peersStr, "{") {
		peersStr = peersStr[1:]
	}
	if strings.HasSuffix(peersStr, "}") {
		peersStr = peersStr[:len(peersStr)-1]
	}

	kvPairs := make(map[string]string)
	parts := splitSurgeLine(peersStr)
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if eq := strings.Index(p, "="); eq > 0 {
			key := strings.TrimSpace(p[:eq])
			val := strings.TrimSpace(p[eq+1:])
			val = unquote(val)
			kvPairs[strings.ToLower(key)] = val
		}
	}

	if v, ok := kvPairs["public-key"]; ok {
		proxy.PublicKey = v
	}
	if v, ok := kvPairs["preshared-key"]; ok {
		proxy.PreSharedKey = v
	}
	if v, ok := kvPairs["endpoint"]; ok {
		if idx := strings.LastIndex(v, ":"); idx > 0 {
			proxy.Server = v[:idx]
			proxy.Port, _ = strconv.Atoi(v[idx+1:])
		}
	}
	if v, ok := kvPairs["allowed-ips"]; ok {
		proxy.AllowedIPs = v
	}
	if v, ok := kvPairs["reserved"]; ok {
		proxy.Reserved = v
	}
}

func loonClientFingerprint(profile string) string {
	switch strings.ToLower(profile) {
	case "chrome":
		return "chrome"
	case "ios18":
		return "ios"
	case "ios26":
		return "ios"
	default:
		return profile
	}
}

func normalizeLoonVmessSecurity(cipher string) string {
	switch strings.ToLower(cipher) {
	case "none":
		return "none"
	case "auto", "":
		return "auto"
	case "aes-128-gcm":
		return "aes-128-gcm"
	case "chacha20-poly1305", "chacha20-ietf-poly1305":
		return "chacha20-poly1305"
	case "zero":
		return "zero"
	default:
		return "auto"
	}
}

func splitFirst(s, sep string) string {
	if idx := strings.Index(s, sep); idx >= 0 {
		return s[:idx]
	}
	return s
}
