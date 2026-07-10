package parser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"sub-store/internal/model"

	"gopkg.in/yaml.v3"
)

// URIParser provides methods for parsing proxy URIs into Proxy structs.
type URIParser struct{}

func NewURIParser() *URIParser {
	return &URIParser{}
}

func (p *URIParser) Parse(line string) (*model.Proxy, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}
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
	prefix := line
	if len(prefix) > 20 {
		prefix = prefix[:20]
	}
	return nil, fmt.Errorf("unsupported URI scheme: %s", prefix)
}

func (p *URIParser) ParseSS(line string) (*model.Proxy, error) {
	rest := strings.TrimPrefix(line, "ss://")

	name := ""
	if idx := strings.LastIndex(rest, "#"); idx >= 0 {
		name, _ = url.PathUnescape(rest[idx+1:])
		rest = rest[:idx]
	}

	if idx := strings.LastIndex(rest, "@"); idx >= 0 {
		encodedPart := rest[:idx]
		serverPart := rest[idx+1:]

		decoded, err := base64DecodeFlexible(encodedPart)
		if err != nil {
			return nil, fmt.Errorf("ss: failed to decode userinfo: %w", err)
		}
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

		return &model.Proxy{
			Type:     "ss",
			Name:     name,
			Server:   server,
			Port:     port,
			Cipher:   method,
			Password: password,
		}, nil
	}

	decoded, err := base64DecodeFlexible(rest)
	if err != nil {
		return nil, fmt.Errorf("ss: failed to decode: %w", err)
	}
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

	return &model.Proxy{
		Type:     "ss",
		Name:     name,
		Server:   server,
		Port:     port,
		Cipher:   userinfo[:colonIdx],
		Password: userinfo[colonIdx+1:],
	}, nil
}

func (p *URIParser) ParseSSR(line string) (*model.Proxy, error) {
	rest := strings.TrimPrefix(line, "ssr://")
	decoded, err := base64DecodeFlexible(rest)
	if err != nil {
		return nil, fmt.Errorf("ssr: decode error: %w", err)
	}

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

	proxy := &model.Proxy{
		Type:     "ssr",
		Server:   server,
		Port:     port,
		Password: password,
		Cipher:   method,
		Protocol: protocol,
		Obfs:     obfs,
	}

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

func (p *URIParser) ParseVMess(line string) (*model.Proxy, error) {
	rest := strings.TrimPrefix(line, "vmess://")
	decoded, err := base64DecodeFlexible(rest)
	if err != nil {
		return nil, fmt.Errorf("vmess: decode error: %w", err)
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(decoded), &obj); err == nil {
		proxy := &model.Proxy{
			Type:    "vmess",
			Name:    getStr(obj, "ps"),
			Server:  getStr(obj, "add"),
			Port:    getInt(obj, "port"),
			UUID:    getStr(obj, "id"),
			AlterID: getInt(obj, "aid"),
			Cipher:  getStr(obj, "scy"),
			Network: getStr(obj, "net"),
		}
		if proxy.Cipher == "" {
			proxy.Cipher = "auto"
		}
		if proxy.Network == "" {
			proxy.Network = "tcp"
		}

		if getStr(obj, "tls") == "tls" {
			proxy.TLS = true
		}

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

		if sni := getStr(obj, "sni"); sni != "" {
			proxy.SNI = sni
		}

		return proxy, nil
	}

	return nil, fmt.Errorf("vmess: failed to parse JSON")
}

func (p *URIParser) ParseVLESS(line string) (*model.Proxy, error) {
	parsed, err := url.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("vless: parse URL error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 443
	}

	proxy := &model.Proxy{
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

	if query.Get("allowInsecure") == "1" {
		proxy.SkipCertVerify = true
	}

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

	if alpn := query.Get("alpn"); alpn != "" {
		proxy.ALPN = strings.Split(alpn, ",")
	}

	return proxy, nil
}

func (p *URIParser) ParseTrojan(line string) (*model.Proxy, error) {
	parsed, err := url.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("trojan: parse URL error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 443
	}

	proxy := &model.Proxy{
		Type:     "trojan",
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

	proxy.Network = query.Get("type")
	if proxy.Network == "" {
		proxy.Network = "tcp"
	}

	if query.Get("allowInsecure") == "1" {
		proxy.SkipCertVerify = true
	}

	if fp := query.Get("fp"); fp != "" {
		proxy.ClientFingerprint = fp
	}

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

	if alpn := query.Get("alpn"); alpn != "" {
		proxy.ALPN = strings.Split(alpn, ",")
	}

	return proxy, nil
}

func (p *URIParser) ParseHysteria2(line string) (*model.Proxy, error) {
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

	proxy := &model.Proxy{
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

func (p *URIParser) ParseHysteria(line string) (*model.Proxy, error) {
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

	proxy := &model.Proxy{
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

func (p *URIParser) ParseTUIC(line string) (*model.Proxy, error) {
	parsed, err := url.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("tuic: parse error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 443
	}

	userInfo := parsed.User.Username()
	password, _ := parsed.User.Password()

	proxy := &model.Proxy{
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

func (p *URIParser) ParseAnyTLS(line string) (*model.Proxy, error) {
	parsed, err := url.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("anytls: parse error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 443
	}

	proxy := &model.Proxy{
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

func (p *URIParser) ParseWireGuard(line string) (*model.Proxy, error) {
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

	proxy := &model.Proxy{
		Type:   "wireguard",
		Name:   decodeFragment(parsed.Fragment),
		Server: parsed.Hostname(),
		Port:   port,
	}

	query := parsed.Query()
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
		proxy.Reserved = reserved
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

	peer := map[string]interface{}{
		"public-key":     proxy.PublicKey,
		"pre-shared-key": proxy.PreSharedKey,
	}
	if proxy.IP != "" {
		peer["ip"] = proxy.IP
	}
	proxy.Peers = []map[string]interface{}{peer}

	return proxy, nil
}

func (p *URIParser) ParseSocks(line string) (*model.Proxy, error) {
	parsed, err := url.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("socks: parse error: %w", err)
	}

	port, _ := strconv.Atoi(parsed.Port())
	if port == 0 {
		port = 1080
	}

	proxy := &model.Proxy{
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

func (p *URIParser) ParseHTTP(line string) (*model.Proxy, error) {
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

	proxy := &model.Proxy{
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

func base64DecodeFlexible(s string) (string, error) {
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}
	return "", fmt.Errorf("base64 decode failed")
}

func parseServerPort(s string) (string, int, error) {
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

// ClashParser parses Clash YAML/JSON proxy configurations into Proxy structs.
type ClashParser struct{}

func NewClashParser() *ClashParser {
	return &ClashParser{}
}

func (c *ClashParser) Parse(raw string) ([]*model.Proxy, error) {
	var root map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &root); err != nil {
		return nil, fmt.Errorf("clash parse error: %w", err)
	}

	proxiesRaw, ok := root["proxies"]
	if !ok {
		return nil, fmt.Errorf("clash config missing 'proxies' field")
	}

	proxiesList, ok := proxiesRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("clash config 'proxies' is not an array")
	}

	proxies := make([]*model.Proxy, 0, len(proxiesList))
	for _, item := range proxiesList {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		proxy := c.parseProxy(m)
		if proxy != nil {
			proxies = append(proxies, proxy)
		}
	}

	return proxies, nil
}

func (c *ClashParser) parseProxy(m map[string]interface{}) *model.Proxy {
	proxyType := strings.ToLower(model.MapGetString(m, "type"))
	if proxyType == "" {
		return nil
	}

	p := &model.Proxy{
		Type:   proxyType,
		Name:   model.MapGetString(m, "name"),
		Server: model.MapGetString(m, "server"),
		Port:   model.MapGetInt(m, "port"),
	}

	p.Password = model.MapGetString(m, "password")
	p.UUID = model.MapGetString(m, "uuid")
	p.Cipher = model.MapGetString(m, "cipher")
	p.AlterID = model.MapGetInt(m, "alterId")
	p.Network = model.MapGetString(m, "network")
	p.TLS = model.MapGetBool(m, "tls")
	p.SNI = model.MapGetString(m, "sni")
	p.TCPFastOpen = model.MapGetBool(m, "tfo")
	p.UDP = model.MapGetBool(m, "udp")
	p.Mux = model.MapGetBool(m, "mux")
	p.IPVersion = model.MapGetString(m, "ip-version")
	p.FastOpen = model.MapGetBool(m, "fast-open")
	p.Obfs = model.MapGetString(m, "obfs")
	p.ObfsParam = model.MapGetString(m, "obfs-param")
	p.Protocol = model.MapGetString(m, "protocol")
	p.ProtocolParam = model.MapGetString(m, "protocol-param")
	p.Username = model.MapGetString(m, "username")
	p.Host = model.MapGetString(m, "host")
	p.Path = model.MapGetString(m, "path")
	p.Flow = model.MapGetString(m, "flow")
	p.Plugin = model.MapGetString(m, "plugin")
	p.PSK = model.MapGetString(m, "psk")
	p.Version = model.MapGetInt(m, "version")
	p.Mode = model.MapGetString(m, "mode")
	p.PrivateKey = model.MapGetString(m, "private-key")
	p.PublicKey = model.MapGetString(m, "public-key")
	p.PreSharedKey = model.MapGetString(m, "pre-shared-key")
	p.IP = model.MapGetString(m, "ip")
	p.IPv6 = model.MapGetString(m, "ipv6")
	p.MTU = model.MapGetInt(m, "mtu")
	p.KeepAlive = model.MapGetInt(m, "keepalive")
	p.PersistentKeepalive = model.MapGetInt(m, "persistent-keepalive")
	p.InterfaceName = model.MapGetString(m, "interface")
	p.UnderlyingProxy = model.MapGetString(m, "dialer-proxy")
	p.Token = model.MapGetString(m, "token")
	p.CongestionController = model.MapGetString(m, "congestion-controller")
	p.UDPRelayMode = model.MapGetString(m, "udp-relay-mode")
	p.HopInterval = model.MapGetInt(m, "hop-interval")
	p.ObfsPassword = model.MapGetString(m, "obfs-password")
	p.Down = model.MapGetString(m, "down")
	p.Up = model.MapGetString(m, "up")
	p.AEAD = model.MapGetBool(m, "aead")
	p.SkipCertVerify = model.MapGetBool(m, "skip-cert-verify")
	p.Insecure = model.MapGetBool(m, "insecure")
	p.CaStr = model.MapGetString(m, "ca-str")
	p.PacketEncoding = model.MapGetString(m, "packet-encoding")
	p.RenameDev = model.MapGetString(m, "rename-dev")
	p.MPTCP = model.MapGetBool(m, "mptcp")
	p.Interface = model.MapGetString(m, "interface")
	p.CA = model.MapGetString(m, "ca")
	p.Workers = model.MapGetInt(m, "workers")
	p.MaxUDPRelayPacketSize = model.MapGetInt(m, "max-udp-relay-packet-size")
	p.MaxOpenStreams = model.MapGetInt(m, "max-open-streams")
	p.HeartbeatInterval = model.MapGetInt(m, "heartbeat-interval")
	p.ReduceRTT = model.MapGetBool(m, "reduce-rtt")
	p.RequestTimeout = model.MapGetInt(m, "request-timeout")
	p.ServerKey = model.MapGetString(m, "server-key")
	p.ServerKeyAlgorithm = model.MapGetString(m, "server-key-algorithm")
	p.UDPOverTCPVersion = model.MapGetInt(m, "udp-over-tcp-version")

	if sni := model.MapGetString(m, "servername"); sni != "" {
		p.SNI = sni
	}
	if fp := model.MapGetString(m, "fingerprint"); fp != "" {
		p.TLSFingerprint = fp
	}
	if fp := model.MapGetString(m, "tls-fingerprint"); fp != "" {
		p.TLSFingerprint = fp
	}
	if cf := model.MapGetString(m, "client-fingerprint"); cf != "" {
		p.ClientFingerprint = cf
	}
	if dp := model.MapGetString(m, "dialer-proxy"); dp != "" {
		p.DialerProxy = dp
	}
	if dp := model.MapGetString(m, "underlying-proxy"); dp != "" {
		p.UnderlyingProxy = dp
	}

	p.ALPN = model.MapGetStringSlice(m, "alpn")
	if reserved := model.MapGetIntSlice(m, "reserved"); len(reserved) > 0 {
		parts := make([]string, len(reserved))
		for i, v := range reserved {
			parts[i] = strconv.Itoa(v)
		}
		p.Reserved = strings.Join(parts, ",")
	}
	if ips := model.MapGetStringSlice(m, "allowed-ips"); len(ips) > 0 {
		p.AllowedIPs = strings.Join(ips, ",")
	}
	p.DNS = model.MapGetStringSlice(m, "dns")

	if v, ok := m["ws-opts"]; ok {
		p.WSOpts = normalizeMap(v)
	}
	if v, ok := m["http-opts"]; ok {
		p.HTTPOpts = normalizeMap(v)
	}
	if v, ok := m["h2-opts"]; ok {
		p.H2Opts = normalizeMap(v)
	}
	if v, ok := m["grpc-opts"]; ok {
		p.GRPCOpts = normalizeMap(v)
	}
	if v, ok := m["plugin-opts"]; ok {
		p.PluginOpts = normalizeMap(v)
	}
	if v, ok := m["reality-opts"]; ok {
		p.RealityOpts = normalizeMap(v)
	}
	if v, ok := m["brutal-opts"]; ok {
		p.BrutalOpts = normalizeMap(v)
	}
	if v, ok := m["local-dns"]; ok {
		if arr, ok := v.([]interface{}); ok {
			res := make([]string, 0, len(arr))
			for _, item := range arr {
				res = append(res, fmt.Sprintf("%v", item))
			}
			p.LocalDNS = res
		}
	}
	if v, ok := m["obfs-opts"]; ok {
		p.ObfsOpts = normalizeMap(v)
	}
	if v, ok := m["override"]; ok {
		p.Override = normalizeMap(v)
	}
	if v, ok := m["smux"]; ok {
		p.Smux = normalizeMap(v)
	}
	if v, ok := m["ech-opts"]; ok {
		p.ECH = normalizeMap(v)
	}
	if v, ok := m["headers"]; ok {
		headers := normalizeMap(v)
		p.Headers = make(map[string]string, len(headers))
		for k, val := range headers {
			p.Headers[k] = fmt.Sprintf("%v", val)
		}
	}
	if v, ok := m["peers"]; ok {
		if arr, ok := v.([]interface{}); ok {
			p.Peers = make([]map[string]interface{}, 0, len(arr))
			for _, item := range arr {
				if peer, ok := item.(map[string]interface{}); ok {
					p.Peers = append(p.Peers, peer)
				}
			}
		}
	}

	return p
}

func normalizeMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	if m, ok := v.(map[interface{}]interface{}); ok {
		result := make(map[string]interface{}, len(m))
		for k, val := range m {
			if ks, ok := k.(string); ok {
				result[ks] = val
			}
		}
		return result
	}
	return nil
}

// Preprocessor preprocesses raw subscription content.
type Preprocessor interface {
	Name() string
	Test(raw string) bool
	Parse(raw string) string
}

type Base64Preprocessor struct{}

func (b *Base64Preprocessor) Name() string { return "Base64 Pre-processor" }

var base64Indicators = []string{
	"dm1lc3M",
	"c3NyOi8v",
	"c29ja3M6Ly",
	"dHJvamFu",
	"c3M6Ly",
	"c3NkOi8v",
	"c2hhZG93",
	"aHR0c",
	"dmxlc3M=",
	"aHlzdGVyaWEy",
	"aHkyOi8v",
	"d2lyZWd1YXJkOi8v",
	"d2c6Ly8=",
	"dHVpYzovLw==",
}

func (b *Base64Preprocessor) Test(raw string) bool {
	if regexp.MustCompile(`^\w+:\/\/\w+`).MatchString(raw) {
		return false
	}
	for _, k := range base64Indicators {
		if strings.Contains(raw, k) {
			return true
		}
	}
	return false
}

func (b *Base64Preprocessor) Parse(raw string) string {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(raw)
		if err != nil {
			return raw
		}
	}
	result := string(decoded)
	if !regexp.MustCompile(`^\w+(:\/\/|\s*=\s*)\w+`).MatchString(result) {
		return raw
	}
	return result
}

type FallbackBase64Preprocessor struct{}

func (f *FallbackBase64Preprocessor) Name() string { return "Fallback Base64 Pre-processor" }
func (f *FallbackBase64Preprocessor) Test(raw string) bool { return true }
func (f *FallbackBase64Preprocessor) Parse(raw string) string {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(raw)
		if err != nil {
			return raw
		}
	}
	result := string(decoded)
	if !regexp.MustCompile(`^\w+(:\/\/|\s*=\s*)\w+`).MatchString(result) {
		return raw
	}
	return result
}

type HTMLPreprocessor struct{}

func (h *HTMLPreprocessor) Name() string { return "HTML Pre-processor" }
func (h *HTMLPreprocessor) Test(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), "<!DOCTYPE html>")
}
func (h *HTMLPreprocessor) Parse(raw string) string { return "" }

type ClashPreprocessor struct{}

func (c *ClashPreprocessor) Name() string { return "Clash Pre-processor" }
func (c *ClashPreprocessor) Test(raw string) bool {
	if !strings.Contains(raw, "proxies:") {
		return false
	}
	var root map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &root); err != nil {
		return false
	}
	proxies, ok := root["proxies"]
	if !ok {
		return false
	}
	_, ok = proxies.([]interface{})
	return ok
}
func (c *ClashPreprocessor) Parse(raw string) string {
	var root map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &root); err != nil {
		return raw
	}
	proxies, ok := root["proxies"].([]interface{})
	if !ok {
		return raw
	}
	var sb strings.Builder
	sb.WriteString("proxies:\n")
	for _, p := range proxies {
		if m, ok := p.(map[string]interface{}); ok {
			data, _ := json.Marshal(m)
			sb.WriteString("  - ")
			sb.Write(data)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

type SSDPreprocessor struct{}

func (s *SSDPreprocessor) Name() string { return "SSD Pre-processor" }
func (s *SSDPreprocessor) Test(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), "ssd://")
}

func (s *SSDPreprocessor) Parse(raw string) string {
	encoded := strings.TrimPrefix(strings.TrimSpace(raw), "ssd://")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(encoded)
		if err != nil {
			return raw
		}
	}

	var ssdInfo map[string]interface{}
	if err := json.Unmarshal(decoded, &ssdInfo); err != nil {
		return raw
	}

	defaultPort := model.MapGetInt(ssdInfo, "port")
	defaultMethod := model.MapGetString(ssdInfo, "encryption")
	defaultPassword := model.MapGetString(ssdInfo, "password")

	serversRaw, ok := ssdInfo["servers"].([]interface{})
	if !ok {
		return raw
	}

	var results []string
	for i, srv := range serversRaw {
		server, ok := srv.(map[string]interface{})
		if !ok {
			continue
		}
		hostname := model.MapGetString(server, "server")
		if hostname == "" {
			continue
		}
		port := model.MapGetInt(server, "port")
		if port == 0 {
			port = defaultPort
		}
		method := model.MapGetString(server, "encryption")
		if method == "" {
			method = defaultMethod
		}
		password := model.MapGetString(server, "password")
		if password == "" {
			password = defaultPassword
		}
		tag := model.MapGetString(server, "remarks")
		if tag == "" {
			tag = fmt.Sprintf("%d", i)
		}

		userinfo := base64.StdEncoding.EncodeToString([]byte(method + ":" + password))
		plugin := ""
		if pluginOpts := model.MapGetString(server, "plugin_options"); pluginOpts != "" {
			pluginName := model.MapGetString(server, "plugin")
			plugin = "/?plugin=" + url.QueryEscape(pluginName+";"+pluginOpts)
		}

		uri := fmt.Sprintf("ss://%s@%s:%d%s#%s", userinfo, hostname, port, plugin, tag)
		results = append(results, uri)
	}

	return strings.Join(results, "\n")
}

type FullConfigPreprocessor struct{}

func (f *FullConfigPreprocessor) Name() string { return "Full Config Preprocessor" }
func (f *FullConfigPreprocessor) Test(raw string) bool {
	return regexp.MustCompile(`^(\[server_local\]|\[Proxy\])`).MatchString(raw)
}
func (f *FullConfigPreprocessor) Parse(raw string) string {
	re := regexp.MustCompile(`(?im)^\[(?:server_local|Proxy)\]([\s\S]+?)^\[.+?\](?:\r?\n|$)`)
	match := re.FindStringSubmatch(raw)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return raw
}

var DefaultPreprocessors = []Preprocessor{
	&HTMLPreprocessor{},
	&ClashPreprocessor{},
	&Base64Preprocessor{},
	&SSDPreprocessor{},
	&FullConfigPreprocessor{},
	&FallbackBase64Preprocessor{},
}

func Preprocess(raw string) string {
	for _, pp := range DefaultPreprocessors {
		if pp.Test(raw) {
			raw = pp.Parse(raw)
		}
	}
	return raw
}

func ParseContent(raw string) ([]*model.Proxy, error) {
	processed := Preprocess(raw)

	if isClashYAML(processed) {
		cp := &ClashParser{}
		if proxies, err := cp.Parse(processed); err == nil && len(proxies) > 0 {
			return proxies, nil
		}
	}

	uriParser := NewURIParser()
	surgeParser := NewSurgeParser()
	loonParser := NewLoonParser()
	qxParser := NewQXParser()

	var proxies []*model.Proxy
	lines := strings.Split(processed, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//") {
			continue
		}

		if proxy, err := uriParser.Parse(line); err == nil && proxy != nil {
			proxies = append(proxies, proxy)
			continue
		}

		if surgeParser.Test(line) {
			if proxy, err := surgeParser.Parse(line); err == nil && proxy != nil {
				proxies = append(proxies, proxy)
				continue
			}
		}

		if loonParser.Test(line) {
			if proxy, err := loonParser.Parse(line); err == nil && proxy != nil {
				proxies = append(proxies, proxy)
				continue
			}
		}

		if qxParser.Test(line) {
			if proxy, err := qxParser.Parse(line); err == nil && proxy != nil {
				proxies = append(proxies, proxy)
				continue
			}
		}
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("no valid proxies found")
	}
	return proxies, nil
}

func isClashYAML(content string) bool {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "proxies:") && !strings.HasPrefix(trimmed, "proxy-providers:") {
		return false
	}
	return strings.Contains(trimmed, "\n")
}
