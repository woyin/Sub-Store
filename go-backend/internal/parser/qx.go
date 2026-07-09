package parser

import (
	"fmt"
	"strconv"
	"strings"

	"sub-store/internal/model"
)

type QXParser struct{}

func NewQXParser() *QXParser { return &QXParser{} }

func (q *QXParser) Test(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "//") {
		return false
	}
	eqIdx := strings.Index(trimmed, "=")
	if eqIdx < 0 {
		return false
	}
	typeStr := strings.ToLower(strings.TrimSpace(trimmed[:eqIdx]))
	qxTypes := map[string]bool{
		"shadowsocks": true, "vmess": true, "vless": true,
		"trojan": true, "http": true, "https": true,
		"socks5": true, "anytls": true,
	}
	return qxTypes[typeStr]
}

func (q *QXParser) Parse(line string) (*model.Proxy, error) {
	trimmed := strings.TrimSpace(line)
	eqIdx := strings.Index(trimmed, "=")
	if eqIdx < 0 {
		return nil, fmt.Errorf("qx: no = found")
	}
	typeStr := strings.ToLower(strings.TrimSpace(trimmed[:eqIdx]))
	rest := strings.TrimSpace(trimmed[eqIdx+1:])

	parts := splitSurgeLine(rest)
	if len(parts) < 1 {
		return nil, fmt.Errorf("qx: empty content")
	}

	serverPort := strings.TrimSpace(parts[0])
	server, port := parseQXServerPort(serverPort)

	proxy := &model.Proxy{
		Server: server,
		Port:   port,
	}

	kvPairs := make(map[string]string)
	for i := 1; i < len(parts); i++ {
		p := strings.TrimSpace(parts[i])
		if eq := strings.Index(p, "="); eq > 0 {
			key := strings.TrimSpace(p[:eq])
			val := strings.TrimSpace(p[eq+1:])
			val = unquote(val)
			kvPairs[strings.ToLower(key)] = val
		}
	}

	if v, ok := kvPairs["tag"]; ok {
		proxy.Name = v
	}

	switch typeStr {
	case "shadowsocks":
		parseQXSS(proxy, kvPairs, rest)
	case "vmess":
		parseQXVMess(proxy, kvPairs)
	case "vless":
		parseQXVLESS(proxy, kvPairs)
	case "trojan":
		parseQXTrojan(proxy, kvPairs)
	case "http", "https":
		parseQXHTTP(proxy, kvPairs, typeStr)
	case "socks5":
		parseQXSocks5(proxy, kvPairs)
	case "anytls":
		parseQXAnyTLS(proxy, kvPairs)
	default:
		return nil, fmt.Errorf("qx: unsupported type %s", typeStr)
	}

	parseQXTLS(proxy, kvPairs)
	parseQXReality(proxy, kvPairs)

	if v, ok := kvPairs["udp-relay"]; ok && v == "true" {
		proxy.UDP = true
	}
	if v, ok := kvPairs["fast-open"]; ok && v == "true" {
		proxy.TCPFastOpen = true
	}
	if v, ok := kvPairs["server_check_url"]; ok {
		if proxy.Extra == nil {
			proxy.Extra = make(map[string]interface{})
		}
		proxy.Extra["test-url"] = v
	}

	return proxy, nil
}

func parseQXServerPort(s string) (string, int) {
	if idx := strings.LastIndex(s, ":"); idx > 0 {
		server := s[:idx]
		port, _ := strconv.Atoi(s[idx+1:])
		return server, port
	}
	return s, 0
}

func parseQXSS(proxy *model.Proxy, kv map[string]string, rawLine string) {
	if v, ok := kv["password"]; ok {
		proxy.Password = v
	}
	if v, ok := kv["method"]; ok {
		proxy.Cipher = v
	}

	isSSR := strings.Contains(strings.ToLower(rawLine), "ssr-protocol")

	if isSSR {
		proxy.Type = "ssr"
		if v, ok := kv["obfs"]; ok {
			proxy.Obfs = v
		}
		if v, ok := kv["obfs-host"]; ok {
			proxy.ObfsParam = v
		}
		if v, ok := kv["ssr-protocol"]; ok {
			proxy.Protocol = v
		}
		if v, ok := kv["ssr-protocol-param"]; ok {
			proxy.ProtocolParam = v
		}
		return
	}

	proxy.Type = "ss"
	obfsVal, hasObfs := kv["obfs"]
	obfsHost, hasHost := kv["obfs-host"]
	obfsURI, hasURI := kv["obfs-uri"]

	if hasObfs {
		switch strings.ToLower(obfsVal) {
		case "tls", "http":
			proxy.Plugin = "obfs-local"
			opts := "obfs=" + strings.ToLower(obfsVal)
			if hasHost {
				opts += ";obfs-host=" + obfsHost
			}
			proxy.PluginOpts = opts
		case "ws":
			proxy.Plugin = "v2ray-plugin"
			proxy.Network = "ws"
			proxy.WSOpts = make(map[string]interface{})
			if hasHost {
				proxy.WSOpts["headers"] = map[string]interface{}{"Host": obfsHost}
			}
			if hasURI {
				proxy.WSOpts["path"] = obfsURI
			}
		case "wss":
			proxy.Plugin = "v2ray-plugin"
			proxy.TLS = true
			proxy.Network = "ws"
			proxy.WSOpts = make(map[string]interface{})
			if hasHost {
				proxy.WSOpts["headers"] = map[string]interface{}{"Host": obfsHost}
			}
			if hasURI {
				proxy.WSOpts["path"] = obfsURI
			}
		case "over-tls":
			proxy.TLS = true
			if hasHost {
				proxy.SNI = obfsHost
			}
		case "vmess-http", "vemss-http", "shadowsocks-http":
			proxy.Network = "http"
			proxy.HTTPOpts = make(map[string]interface{})
			if hasHost {
				proxy.HTTPOpts["headers"] = map[string]interface{}{"Host": obfsHost}
			}
			if hasURI {
				proxy.HTTPOpts["path"] = obfsURI
			}
		}
	}
}

func parseQXVMess(proxy *model.Proxy, kv map[string]string) {
	proxy.Type = "vmess"
	if v, ok := kv["password"]; ok {
		proxy.UUID = v
	}
	if v, ok := kv["method"]; ok {
		proxy.Cipher = v
	}
	if v, ok := kv["aead"]; ok {
		proxy.AEAD = v == "true"
		if v == "true" {
			proxy.AlterID = 0
		} else {
			proxy.AlterID = 1
		}
	}

	parseQXObfs(proxy, kv)
}

func parseQXVLESS(proxy *model.Proxy, kv map[string]string) {
	proxy.Type = "vless"
	if v, ok := kv["password"]; ok {
		proxy.UUID = v
	}
	if v, ok := kv["method"]; ok {
		proxy.Cipher = v
	}
	if v, ok := kv["vless-flow"]; ok {
		proxy.Flow = v
	}

	parseQXObfs(proxy, kv)
}

func parseQXTrojan(proxy *model.Proxy, kv map[string]string) {
	proxy.Type = "trojan"
	proxy.TLS = true
	if v, ok := kv["password"]; ok {
		proxy.Password = v
	}

	if v, ok := kv["obfs"]; ok {
		switch strings.ToLower(v) {
		case "ws":
			proxy.Network = "ws"
			proxy.WSOpts = make(map[string]interface{})
			if h, ok := kv["obfs-host"]; ok {
				proxy.WSOpts["headers"] = map[string]interface{}{"Host": h}
			}
			if u, ok := kv["obfs-uri"]; ok {
				proxy.WSOpts["path"] = u
			}
		case "wss":
			proxy.Network = "ws"
			proxy.TLS = true
			proxy.WSOpts = make(map[string]interface{})
			if h, ok := kv["obfs-host"]; ok {
				proxy.WSOpts["headers"] = map[string]interface{}{"Host": h}
			}
			if u, ok := kv["obfs-uri"]; ok {
				proxy.WSOpts["path"] = u
			}
		case "over-tls":
			proxy.TLS = true
			if h, ok := kv["obfs-host"]; ok {
				proxy.SNI = h
			}
		case "http":
			proxy.Network = "http"
			proxy.HTTPOpts = make(map[string]interface{})
			if h, ok := kv["obfs-host"]; ok {
				proxy.HTTPOpts["headers"] = map[string]interface{}{"Host": h}
			}
			if u, ok := kv["obfs-uri"]; ok {
				proxy.HTTPOpts["path"] = u
			}
		}
	}
}

func parseQXHTTP(proxy *model.Proxy, kv map[string]string, typeStr string) {
	proxy.Type = "http"
	if typeStr == "https" {
		proxy.TLS = true
	}
	if v, ok := kv["username"]; ok {
		proxy.Username = v
	}
	if v, ok := kv["password"]; ok {
		proxy.Password = v
	}
}

func parseQXSocks5(proxy *model.Proxy, kv map[string]string) {
	proxy.Type = "socks5"
	if v, ok := kv["username"]; ok {
		proxy.Username = v
	}
	if v, ok := kv["password"]; ok {
		proxy.Password = v
	}
}

func parseQXAnyTLS(proxy *model.Proxy, kv map[string]string) {
	proxy.Type = "anytls"
	proxy.TLS = true
	if v, ok := kv["password"]; ok {
		proxy.Password = v
	}
}

func parseQXObfs(proxy *model.Proxy, kv map[string]string) {
	obfsVal, hasObfs := kv["obfs"]
	if !hasObfs {
		return
	}
	obfsHost, hasHost := kv["obfs-host"]
	obfsURI, hasURI := kv["obfs-uri"]

	switch strings.ToLower(obfsVal) {
	case "ws":
		proxy.Network = "ws"
		proxy.WSOpts = make(map[string]interface{})
		if hasHost {
			proxy.WSOpts["headers"] = map[string]interface{}{"Host": obfsHost}
		}
		if hasURI {
			proxy.WSOpts["path"] = obfsURI
		}
	case "wss":
		proxy.Network = "ws"
		proxy.TLS = true
		proxy.WSOpts = make(map[string]interface{})
		if hasHost {
			proxy.WSOpts["headers"] = map[string]interface{}{"Host": obfsHost}
		}
		if hasURI {
			proxy.WSOpts["path"] = obfsURI
		}
	case "over-tls":
		proxy.TLS = true
		if hasHost {
			proxy.SNI = obfsHost
		}
	case "http":
		proxy.Network = "http"
		proxy.HTTPOpts = make(map[string]interface{})
		if hasHost {
			proxy.HTTPOpts["headers"] = map[string]interface{}{"Host": obfsHost}
		}
		if hasURI {
			proxy.HTTPOpts["path"] = obfsURI
		}
	case "vmess-http", "vemss-http", "shadowsocks-http":
		proxy.Network = "http"
		proxy.HTTPOpts = make(map[string]interface{})
		if hasHost {
			proxy.HTTPOpts["headers"] = map[string]interface{}{"Host": obfsHost}
		}
		if hasURI {
			proxy.HTTPOpts["path"] = obfsURI
		}
	}
}

func parseQXTLS(proxy *model.Proxy, kv map[string]string) {
	if v, ok := kv["over-tls"]; ok && v == "true" {
		proxy.TLS = true
	}
	if v, ok := kv["tls-host"]; ok && v != "" {
		proxy.SNI = v
	}
	if v, ok := kv["tls-verification"]; ok {
		proxy.SkipCertVerify = v == "false"
	}
	if v, ok := kv["tls-cert-sha256"]; ok {
		proxy.TLSFingerprint = v
	}
	if v, ok := kv["tls-pubkey-sha256"]; ok {
		if proxy.Extra == nil {
			proxy.Extra = make(map[string]interface{})
		}
		proxy.Extra["tls-pubkey-sha256"] = v
	}
	if v, ok := kv["tls-alpn"]; ok && v != "" {
		proxy.ALPN = strings.Split(v, ",")
	}
	if v, ok := kv["tls-no-session-ticket"]; ok && v == "true" {
		if proxy.Extra == nil {
			proxy.Extra = make(map[string]interface{})
		}
		proxy.Extra["tls-no-session-ticket"] = true
	}
	if v, ok := kv["tls-no-session-reuse"]; ok && v == "true" {
		if proxy.Extra == nil {
			proxy.Extra = make(map[string]interface{})
		}
		proxy.Extra["tls-no-session-reuse"] = true
	}
}

func parseQXReality(proxy *model.Proxy, kv map[string]string) {
	pubKey, hasPub := kv["reality-base64-pubkey"]
	shortID, hasShort := kv["reality-hex-shortid"]
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
