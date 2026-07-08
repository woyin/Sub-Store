package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// URIParser provides methods for parsing proxy URIs into Proxy structs.
type URIParser struct{}

// NewURIParser creates a new URIParser instance.
func NewURIParser() *URIParser {
	return &URIParser{}
}

// Parse attempts to parse a proxy URI based on its scheme.
func (p *URIParser) Parse(line string) (*Proxy, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}
	// Must be checked before trojan:// due to prefix overlap
	if strings.HasPrefix(line, "hysteria2://") || strings.HasPrefix(line, "hy2://") {
		return p.ParseHysteria2(line)
	}
	if strings.HasPrefix(line, "hysteria://") || strings.HasPrefix(line, "hy://") {
		return p.ParseHysteria(line)
	}
	if strings.HasPrefix(line, "ss://") {
		return p.ParseSS(line)
	}
	if strings.HasPrefix(line, "ssr://") {
		return p.ParseSSR(line)
	}
	if strings.HasPrefix(line, "vmess://") {
		return p.ParseVMess(line)
	}
	if strings.HasPrefix(line, "vless://") {
		return p.ParseVLESS(line)
	}
	if strings.HasPrefix(line, "trojan://") {
		return p.ParseTrojan(line)
	}
	if strings.HasPrefix(line, "tuic://") {
		return p.ParseTUIC(line)
	}
	if strings.HasPrefix(line, "anytls://") {
		return p.ParseAnyTLS(line)
	}
	if strings.HasPrefix(line, "wireguard://") || strings.HasPrefix(line, "wg://") {
		return p.ParseWireGuard(line)
	}
	if strings.HasPrefix(line, "socks5://") || strings.HasPrefix(line, "socks://") {
		return p.ParseSocks(line)
	}
	if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
		return p.ParseHTTP(line)
	}
	return nil, fmt.Errorf("unsupported URI scheme: %s", line[:min(20, len(line))])
}

// --- Shadowsocks ---

func (p *URIParser) ParseSS(line string) (*Proxy, error) {
	// ss://BASE64(method:password)@server:port#name (SIP002)
	// ss://BASE64(method:password@server:port)#name (legacy)
	rest := strings.TrimPrefix(line, "ss://")

	// Extract fragment (name)
	name := ""
	if idx := strings.LastIndex(rest, "#"); idx >= 0 {
		name, _ = url.PathUnescape(rest[idx+1:])
		rest = rest[:idx]
	}

	// Try SIP002 format: base64@server:port
	if idx := strings.LastIndex(rest, "@"); idx >= 0 {
		encodedPart := rest[:idx]
		serverPart := rest[idx+1:]

		// Decode the userinfo
		decoded, err := base64DecodeFlexible(encodedPart)
		if err != nil {
			return nil, fmt.Errorf("ss: failed to decode userinfo: %w", err)
		}
		// decoded should be "method:password"
		colonIdx := strings.Index(decoded, ":")
		if colonIdx < 0 {
			return nil, fmt.Errorf("ss: invalid userinfo format")
		}
		method := decoded[:colonIdx]
		password := decoded[colonIdx+1:]

		server, port, err := parseServerPort(serverPart)
		if err != nil {
			return nil, fmt.Errorf("ss: %w", err)
		}

		proxy := &Proxy{
			Type:     "ss",
			Name:     name,
			Server:   server,
			Port:     port,
			Cipher:   method,
			Password: password,
		}
		return proxy, nil
	}

	// Legacy format: entire rest is base64
	decoded, err := base64DecodeFlexible(rest)
	if err != nil {
		return nil, fmt.Errorf("ss: failed to decode: %w", err)
	}
	// decoded: method:password@server:port
	atIdx := strings.LastIndex(decoded, "@")
	if atIdx < 0 {
		return nil, fmt.Errorf("ss: invalid legacy format")
	}
	userinfo := decoded[:atIdx]
	serverPart := decoded[atIdx+1:]

	colonIdx := strings.Index(userinfo, ":")
	if colonIdx < 0 {
		return nil, fmt.Errorf("ss: invalid userinfo")
	}

	server, port, err := parseServerPort(serverPart)
	if err != nil {
		return nil, fmt.Errorf("ss: %w", err)
	}

	return &Proxy{
		Type:     "ss",
		Name:     name,
		Server:   server,
		Port:     port,
		Cipher:   userinfo[:colonIdx],
		Password: userinfo[colonIdx+1:],
	}, nil
}

// --- ShadowsocksR ---

func (p *URIParser) ParseSSR(line string) (*Proxy, error) {
	// ssr://base64(server:port:protocol:method:obfs:base64password/?params)
	rest := strings.TrimPrefix(line, "ssr://")
	decoded, err := base64DecodeFlexible(rest)
	if err != nil {
		return nil, fmt.Errorf("ssr: decode error: %w", err)
	}

	// Split main part and parameters
	var mainPart, paramPart string
	if idx := strings.Index(decoded, "/?"); idx >= 0 {
		mainPart = decoded[:idx]
		paramPart = decoded[idx+2:]
	} else {
		mainPart = decoded
	}

	parts := strings.SplitN(mainPart, ":", 6)
	if len(parts) < 6 {
		return nil, fmt.Errorf("ssr: invalid format, expected 6 parts, got %d", len(parts))
	}

	server := parts[0]
	port, _ := strconv.Atoi(parts[1])
	protocol := parts[2]
	method := parts[3]
	obfs := parts[4]
	base64Password := parts[5]

	password, _ := base64DecodeFlexible(base64Password)

	proxy := &Proxy{
		Type:          "ssr",
		Server:        server,
		Port:          port,
		Password:      password,
		Cipher:        method,
		Protocol:      protocol,
		Obfs:          obfs,
	}

	// Parse parameters
	if paramPart != "" {
		params, _ := url.ParseQuery(paramPart)
		if params.Get("remarks") != "" {
			proxy.Name, _ = base64DecodeFlexible(params.Get("remarks"))
		}
		if params.Get("obfsparam") != "" {
			proxy.ObfsParam, _ = base64DecodeFlexible(params.Get("obfsparam"))
		}
		if params.Get("protoparam") != "" {
			proxy.ProtocolParam, _ = base64DecodeFlexible(params.Get("protoparam"))
		}
	}
	if proxy.Name == "" {
		proxy.Name = fmt.Sprintf("SSR %s:%d", server, port)
	}

	return proxy, nil
}

// --- VMess ---

func (p *URIParser) ParseVMess(line string) (*Proxy, error) {
	// vmess://base64(json) - V2RayN format
	rest := strings.TrimPrefix(line, "vmess://")
	decoded, err := base64DecodeFlexible(rest)
	if err != nil {
		return nil, fmt.Errorf("vmess: decode error: %w", err)
	}

	// Try JSON format (V2RayN)
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(decoded), &obj); err == nil {
		proxy := &Proxy{
			Type:   "vmess",
			Name:   getStr(obj, "ps"),
			Server: getStr(obj, "add"),
			Port:   getInt(obj, "port"),
			UUID:   getStr(obj, "id"),
			AlterID: getInt(obj, "aid"),
			Cipher: getStr(obj, "scy"),
			Network: getStr(obj, "net"),
		}
		if proxy.Cipher == "" {
			proxy.Cipher = "auto"
		}
		if proxy.Network == "" {
			proxy.Network = "tcp"
		}

		// TLS
		if getStr(obj, "tls") == "tls" {
			proxy.TLS = true
		}

		// Transport options
		switch proxy.Network {
		case "ws":
			proxy.WSOpts = make(map[string]interface{})
			if path := getStr(obj, "path"); path != "" {
				proxy.WSOpts["path"] = path
			}
			if host := getStr(obj, "host"); host != "" {
				proxy.WSOpts["headers"] = map[string]interface{}{"Host": host}
			}
		case "http":
			proxy.HTTPOpts = make(map[string]interface{})
			if path := getStr(obj, "path"); path != "" {
				proxy.HTTPOpts["path"] = path
			}
			if host := getStr(obj, "host"); host != "" {
				proxy.HTTPOpts["path"] = host
			}
		case "h2":
			proxy.H2Opts = make(map[string]interface{})
			if path := getStr(obj, "path"); path != "" {
				proxy.H2Opts["path"] = path
			}
			if host := getStr(obj, "host"); host != "" {
				proxy.H2Opts["host"] = host
			}
		case "grpc":
			proxy.GRPCOpts = make(map[string]interface{})
			if serviceName := getStr(obj, "path"); serviceName != "" {
				proxy.GRPCOpts["grpc-service-name"] = serviceName
			}
		}

		// SNI
		if sni := getStr(obj, "sni"); sni != "" {
			proxy.SNI = sni
		}

		return proxy, nil
	}

	return nil, fmt.Errorf("vmess: failed to parse JSON")
}

// --- VLESS ---

func (p *URIParser) ParseVLESS(line string) (*Proxy, error) {
	// vless://uuid@server:port?type=tcp&security=tls&sni=xxx&flow=xtls-rprx-vision#name
	parsed, err := url.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("vless: parse URL error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 443
	}

	proxy := &Proxy{
		Type:   "vless",
		Name:   decodeFragment(parsed.Fragment),
		Server: parsed.Hostname(),
		Port:   port,
		UUID:   parsed.User.Username(),
	}

	query := parsed.Query()
	proxy.Network = query.Get("type")
	if proxy.Network == "" {
		proxy.Network = "tcp"
	}

	// Security / TLS
	security := query.Get("security")
	if security == "tls" || security == "reality" {
		proxy.TLS = true
	}
	if sni := query.Get("sni"); sni != "" {
		proxy.SNI = sni
	}
	if flow := query.Get("flow"); flow != "" {
		proxy.Flow = flow
	}
	if fp := query.Get("fp"); fp != "" {
		proxy.ClientFingerprint = fp
	}

	// Reality
	if security == "reality" {
		proxy.RealityOpts = make(map[string]interface{})
		if pbk := query.Get("pbk"); pbk != "" {
			proxy.RealityOpts["public-key"] = pbk
		}
		if sid := query.Get("sid"); sid != "" {
			proxy.RealityOpts["short-id"] = sid
		}
		if spx := query.Get("spx"); spx != "" {
			proxy.RealityOpts["server-x509-name"] = spx
		}
	}

	// Skip cert verify
	if query.Get("allowInsecure") == "1" {
		proxy.SkipCertVerify = true
	}

	// Transport
	switch proxy.Network {
	case "ws":
		proxy.WSOpts = make(map[string]interface{})
		if path := query.Get("path"); path != "" {
			proxy.WSOpts["path"] = path
		}
		if host := query.Get("host"); host != "" {
			proxy.WSOpts["headers"] = map[string]interface{}{"Host": host}
		}
	case "http":
		proxy.HTTPOpts = make(map[string]interface{})
		if path := query.Get("path"); path != "" {
			proxy.HTTPOpts["path"] = path
		}
		if host := query.Get("host"); host != "" {
			proxy.HTTPOpts["path"] = host
		}
	case "grpc":
		proxy.GRPCOpts = make(map[string]interface{})
		if serviceName := query.Get("serviceName"); serviceName != "" {
			proxy.GRPCOpts["grpc-service-name"] = serviceName
		}
	case "h2":
		proxy.H2Opts = make(map[string]interface{})
		if path := query.Get("path"); path != "" {
			proxy.H2Opts["path"] = path
		}
		if host := query.Get("host"); host != "" {
			proxy.H2Opts["host"] = host
		}
	}

	// ALPN
	if alpn := query.Get("alpn"); alpn != "" {
		proxy.ALPN = strings.Split(alpn, ",")
	}

	return proxy, nil
}

// --- Trojan ---

func (p *URIParser) ParseTrojan(line string) (*Proxy, error) {
	// trojan://password@server:port?security=tls&sni=xxx#name
	parsed, err := url.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("trojan: parse URL error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 443
	}

	proxy := &Proxy{
		Type:     "trojan",
		Name:     decodeFragment(parsed.Fragment),
		Server:   parsed.Hostname(),
		Port:     port,
		Password: parsed.User.Username(),
		TLS:      true, // Trojan always uses TLS
	}

	query := parsed.Query()

	// SNI
	if sni := query.Get("sni"); sni != "" {
		proxy.SNI = sni
	}

	// Network type
	proxy.Network = query.Get("type")
	if proxy.Network == "" {
		proxy.Network = "tcp"
	}

	// Skip cert verify
	if query.Get("allowInsecure") == "1" {
		proxy.SkipCertVerify = true
	}

	// Fingerprint
	if fp := query.Get("fp"); fp != "" {
		proxy.ClientFingerprint = fp
	}

	// Transport
	switch proxy.Network {
	case "ws":
		proxy.WSOpts = make(map[string]interface{})
		if path := query.Get("path"); path != "" {
			proxy.WSOpts["path"] = path
		}
		if host := query.Get("host"); host != "" {
			proxy.WSOpts["headers"] = map[string]interface{}{"Host": host}
		}
	case "grpc":
		proxy.GRPCOpts = make(map[string]interface{})
		if serviceName := query.Get("serviceName"); serviceName != "" {
			proxy.GRPCOpts["grpc-service-name"] = serviceName
		}
	case "h2":
		proxy.H2Opts = make(map[string]interface{})
		if path := query.Get("path"); path != "" {
			proxy.H2Opts["path"] = path
		}
		if host := query.Get("host"); host != "" {
			proxy.H2Opts["host"] = host
		}
	}

	// ALPN
	if alpn := query.Get("alpn"); alpn != "" {
		proxy.ALPN = strings.Split(alpn, ",")
	}

	return proxy, nil
}

// --- Hysteria2 ---

func (p *URIParser) ParseHysteria2(line string) (*Proxy, error) {
	// hysteria2://password@server:port?sni=xxx#name or hy2://...
	var rest string
	if strings.HasPrefix(line, "hysteria2://") {
		rest = strings.TrimPrefix(line, "hysteria2://")
	} else {
		rest = strings.TrimPrefix(line, "hy2://")
	}

	parsed, err := url.Parse("hysteria2://" + rest)
	if err != nil {
		return nil, fmt.Errorf("hysteria2: parse error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 443
	}

	proxy := &Proxy{
		Type:     "hysteria2",
		Name:     decodeFragment(parsed.Fragment),
		Server:   parsed.Hostname(),
		Port:     port,
		Password: parsed.User.Username(),
		TLS:      true,
	}

	query := parsed.Query()
	if sni := query.Get("sni"); sni != "" {
		proxy.SNI = sni
	}
	if query.Get("insecure") == "1" {
		proxy.SkipCertVerify = true
	}
	if obfs := query.Get("obfs"); obfs != "" {
		proxy.Obfs = obfs
	}
	if obfsPassword := query.Get("obfs-password"); obfsPassword != "" {
		proxy.ObfsPassword = obfsPassword
	}
	if pin := query.Get("pin"); pin != "" {
		proxy.Extra = map[string]interface{}{"pin": pin}
	}

	return proxy, nil
}

// --- Hysteria ---

func (p *URIParser) ParseHysteria(line string) (*Proxy, error) {
	var rest string
	if strings.HasPrefix(line, "hysteria://") {
		rest = strings.TrimPrefix(line, "hysteria://")
	} else {
		rest = strings.TrimPrefix(line, "hy://")
	}

	parsed, err := url.Parse("hysteria://" + rest)
	if err != nil {
		return nil, fmt.Errorf("hysteria: parse error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 443
	}

	proxy := &Proxy{
		Type:     "hysteria",
		Name:     decodeFragment(parsed.Fragment),
		Server:   parsed.Hostname(),
		Port:     port,
		Password: parsed.User.Username(),
		TLS:      true,
	}

	query := parsed.Query()
	if sni := query.Get("sni"); sni != "" {
		proxy.SNI = sni
	}
	if query.Get("insecure") == "1" {
		proxy.SkipCertVerify = true
	}
	if up := query.Get("up"); up != "" {
		proxy.Up = up
	}
	if down := query.Get("down"); down != "" {
		proxy.Down = down
	}
	if obfs := query.Get("obfs"); obfs != "" {
		proxy.Obfs = obfs
	}
	if alpn := query.Get("alpn"); alpn != "" {
		proxy.ALPN = strings.Split(alpn, ",")
	}
	if auth := query.Get("auth"); auth != "" {
		proxy.Extra = map[string]interface{}{"auth": auth}
	}

	return proxy, nil
}

// --- TUIC ---

func (p *URIParser) ParseTUIC(line string) (*Proxy, error) {
	// tuic://uuid:password@server:port?sni=xxx#name
	parsed, err := url.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("tuic: parse error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 443
	}

	// UUID and password from userinfo
	userInfo := parsed.User.Username()
	password, _ := parsed.User.Password()

	proxy := &Proxy{
		Type:     "tuic",
		Name:     decodeFragment(parsed.Fragment),
		Server:   parsed.Hostname(),
		Port:     port,
		UUID:     userInfo,
		Password: password,
		TLS:      true,
		ALPN:     []string{"h3"},
	}

	query := parsed.Query()
	if sni := query.Get("sni"); sni != "" {
		proxy.SNI = sni
	}
	if query.Get("allowInsecure") == "1" {
		proxy.SkipCertVerify = true
	}
	if cc := query.Get("congestion_control"); cc != "" {
		proxy.CongestionController = cc
	}
	if udpRelay := query.Get("udp_relay_mode"); udpRelay != "" {
		proxy.UDPRelayMode = udpRelay
	}
	if alpn := query.Get("alpn"); alpn != "" {
		proxy.ALPN = strings.Split(alpn, ",")
	}

	if proxy.CongestionController == "" {
		proxy.CongestionController = "cubic"
	}
	if proxy.UDPRelayMode == "" {
		proxy.UDPRelayMode = "native"
	}

	return proxy, nil
}

// --- AnyTLS ---

func (p *URIParser) ParseAnyTLS(line string) (*Proxy, error) {
	// anytls://password@server:port?sni=xxx#name
	parsed, err := url.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("anytls: parse error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 443
	}

	proxy := &Proxy{
		Type:     "anytls",
		Name:     decodeFragment(parsed.Fragment),
		Server:   parsed.Hostname(),
		Port:     port,
		Password: parsed.User.Username(),
		TLS:      true,
	}

	query := parsed.Query()
	if sni := query.Get("sni"); sni != "" {
		proxy.SNI = sni
	}
	if query.Get("insecure") == "1" {
		proxy.SkipCertVerify = true
	}
	if fp := query.Get("fp"); fp != "" {
		proxy.ClientFingerprint = fp
	}

	return proxy, nil
}

// --- WireGuard ---

func (p *URIParser) ParseWireGuard(line string) (*Proxy, error) {
	var rest string
	if strings.HasPrefix(line, "wireguard://") {
		rest = strings.TrimPrefix(line, "wireguard://")
	} else {
		rest = strings.TrimPrefix(line, "wg://")
	}

	parsed, err := url.Parse("wireguard://" + rest)
	if err != nil {
		return nil, fmt.Errorf("wireguard: parse error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 51820
	}

	proxy := &Proxy{
		Type:   "wireguard",
		Name:   decodeFragment(parsed.Fragment),
		Server: parsed.Hostname(),
		Port:   port,
	}

	query := parsed.Query()
	// Private key is in the userinfo
	privateKey := parsed.User.Username()
	if privateKey != "" {
		proxy.PrivateKey = privateKey
	}
	if publicKey := query.Get("publickey"); publicKey != "" {
		proxy.PublicKey = publicKey
	}
	if ip := query.Get("ip"); ip != "" {
		proxy.IP = ip
	}
	if ipv6 := query.Get("ipv6"); ipv6 != "" {
		proxy.IPv6 = ipv6
	}
	if mtu := query.Get("mtu"); mtu != "" {
		proxy.MTU, _ = strconv.Atoi(mtu)
	}
	if reserved := query.Get("reserved"); reserved != "" {
		proxy.Reserved = parseIntSlice(reserved)
	}
	if dns := query.Get("dns"); dns != "" {
		proxy.DNS = strings.Split(dns, ",")
	}
	if pk := query.Get("presharedkey"); pk != "" {
		proxy.PreSharedKey = pk
	}
	if ifName := query.Get("ifname"); ifName != "" {
		proxy.InterfaceName = ifName
	}
	if proxy.InterfaceName == "" {
		proxy.InterfaceName = "wg0"
	}

	// Peers
	peer := map[string]interface{}{
		"public-key":  proxy.PublicKey,
		"pre-shared-key": proxy.PreSharedKey,
	}
	if proxy.IP != "" {
		peer["ip"] = proxy.IP
	}
	proxy.Peers = []map[string]interface{}{peer}

	return proxy, nil
}

// --- SOCKS5 ---

func (p *URIParser) ParseSocks(line string) (*Proxy, error) {
	parsed, err := url.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("socks: parse error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 1080
	}

	proxy := &Proxy{
		Type:   "socks5",
		Name:   decodeFragment(parsed.Fragment),
		Server: parsed.Hostname(),
		Port:   port,
	}

	proxy.Username = parsed.User.Username()
	proxy.Password, _ = parsed.User.Password()

	if query := parsed.Query(); query.Get("tls") == "1" {
		proxy.TLS = true
		if sni := query.Get("sni"); sni != "" {
			proxy.SNI = sni
		}
		if query.Get("skip-cert-verify") == "1" {
			proxy.SkipCertVerify = true
		}
	}

	return proxy, nil
}

// --- HTTP ---

func (p *URIParser) ParseHTTP(line string) (*Proxy, error) {
	parsed, err := url.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("http: parse error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		if parsed.Scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}

	proxy := &Proxy{
		Type:   "http",
		Name:   decodeFragment(parsed.Fragment),
		Server: parsed.Hostname(),
		Port:   port,
	}

	proxy.Username = parsed.User.Username()
	proxy.Password, _ = parsed.User.Password()

	if parsed.Scheme == "https" {
		proxy.TLS = true
	}

	if query := parsed.Query(); query.Get("sni") != "" {
		proxy.SNI = query.Get("sni")
	}
	if query := parsed.Query(); query.Get("skip-cert-verify") == "1" {
		proxy.SkipCertVerify = true
	}

	if proxy.Name == "" {
		if parsed.Scheme == "https" {
			proxy.Name = fmt.Sprintf("HTTPS %s:%d", proxy.Server, proxy.Port)
		} else {
			proxy.Name = fmt.Sprintf("HTTP %s:%d", proxy.Server, proxy.Port)
		}
	}

	return proxy, nil
}

// --- Helper functions ---

func base64DecodeFlexible(s string) (string, error) {
	// Try standard base64 first
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}
	// Try URL-safe base64
	if decoded, err := base64.URLEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}
	// Try with padding
	if decoded, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}
	return "", fmt.Errorf("base64 decode failed")
}

func parseServerPort(s string) (string, int, error) {
	// Handle IPv6: [::1]:port
	if strings.HasPrefix(s, "[") {
		closeBracket := strings.Index(s, "]")
		if closeBracket < 0 {
			return "", 0, fmt.Errorf("invalid IPv6 address")
		}
		server := s[1:closeBracket]
		rest := s[closeBracket+1:]
		if len(rest) > 0 && rest[0] == ':' {
			port, err := strconv.Atoi(rest[1:])
			if err != nil {
				return server, 0, nil
			}
			return server, port, nil
		}
		return server, 0, nil
	}

	// Regular host:port
	lastColon := strings.LastIndex(s, ":")
	if lastColon < 0 {
		return s, 0, nil
	}
	server := s[:lastColon]
	port, err := strconv.Atoi(s[lastColon+1:])
	if err != nil {
		return s, 0, nil
	}
	return server, port, nil
}

func decodeFragment(fragment string) string {
	if fragment == "" {
		return ""
	}
	decoded, err := url.PathUnescape(fragment)
	if err != nil {
		return fragment
	}
	return decoded
}

func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		case string:
			i, _ := strconv.Atoi(val)
			return i
		}
	}
	return 0
}

func parseIntSlice(s string) []int {
	parts := strings.Split(s, ",")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if i, err := strconv.Atoi(p); err == nil {
			result = append(result, i)
		}
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
