package producer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"sub-store/internal/model"
	"sub-store/internal/normalizer"

	"gopkg.in/yaml.v3"
)

type Producer interface {
	Produce(proxies []*model.Proxy) (string, error)
	ProduceSingle(proxy *model.Proxy) (string, error)
	Type() string
}

var PlatformProducers = map[string]Producer{
	"clash":        &clashProducer{},
	"clashmeta":    &clashMetaProducer{},
	"meta":         &clashMetaProducer{},
	"mihomo":       &clashMetaProducer{},
	"surge":        &surgeProducer{},
	"loon":         &loonProducer{},
	"qx":           &qxProducer{},
	"singbox":      &singBoxProducer{},
	"sing-box":     &singBoxProducer{},
	"v2ray":        &v2rayProducer{},
	"uri":          &uriProducer{},
	"shadowrocket": &shadowrocketProducer{},
	"stash":        &stashProducer{},
	"surfboard":    &surfboardProducer{},
	"egern":        &egernProducer{},
	"surgemac":     &surgeMacProducer{},
	"json":         &jsonProducer{},
}

var PlatformSupport = map[string]map[string]bool{
	"clash":     {"ss": true, "vmess": true, "trojan": true, "ssr": false, "vless": false, "hysteria": false, "hysteria2": true, "tuic": true},
	"clashmeta": {"ss": true, "vmess": true, "trojan": true, "ssr": true, "vless": true, "hysteria": true, "hysteria2": true, "tuic": true, "anytls": true},
	"surge":     {"ss": true, "vmess": true, "trojan": true, "ssr": false, "vless": false, "hysteria": false, "hysteria2": true, "tuic": true, "wireguard": true, "socks5": true, "http": true, "snell": true},
	"loon":      {"ss": true, "vmess": true, "trojan": true, "ssr": false, "vless": true, "hysteria2": true, "tuic": true, "wireguard": true, "socks5": true, "http": true},
	"qx":        {"ss": true, "vmess": true, "trojan": true, "ssr": true, "vless": true, "hysteria2": true, "tuic": true, "wireguard": true, "socks5": true, "http": true},
	"singbox":   {"ss": true, "vmess": true, "trojan": true, "ssr": true, "vless": true, "hysteria": true, "hysteria2": true, "tuic": true, "anytls": true, "wireguard": true, "socks5": true, "http": true},
	"v2ray":     {"vmess": true},
	"uri":       {"ss": true, "vmess": true, "trojan": true, "ssr": true, "vless": true, "hysteria": true, "hysteria2": true, "tuic": true, "wireguard": true, "socks5": true, "http": true},
	"shadowrocket": {"ss": true, "vmess": true, "trojan": true, "ssr": true, "vless": true, "hysteria": true, "hysteria2": true, "tuic": true, "anytls": true, "wireguard": true, "socks5": true, "http": true, "snell": true},
	"stash":     {"ss": true, "ssr": true, "vmess": true, "trojan": true, "vless": true, "hysteria": true, "hysteria2": true, "tuic": true, "anytls": true, "wireguard": true, "socks5": true, "http": true, "snell": true, "ssh": true, "juicity": true, "trusttunnel": true},
	"surfboard": {"ss": true, "vmess": true, "trojan": true, "hysteria2": true, "anytls": true, "socks5": true, "http": true, "snell": true, "wireguard": true},
	"egern":     {"ss": true, "vmess": true, "trojan": true, "vless": true, "hysteria2": true, "tuic": true, "anytls": true, "wireguard": true, "socks5": true, "http": true, "ssh": true, "snell": true},
	"surgemac":  {"ss": true, "vmess": true, "trojan": true, "hysteria2": true, "tuic": true, "wireguard": true, "socks5": true, "http": true, "snell": true, "ssh": true, "h2-connect": true, "anytls": true, "trusttunnel": true, "direct": true, "external": true},
	"json":      {"ss": true, "ssr": true, "vmess": true, "trojan": true, "vless": true, "hysteria": true, "hysteria2": true, "tuic": true, "anytls": true, "wireguard": true, "socks5": true, "http": true, "snell": true, "ssh": true},
}

func IsProxySupported(platform, proxyType string) bool {
	if support, ok := PlatformSupport[platform]; ok {
		return support[proxyType]
	}
	return false
}

func GetProducer(platform string) Producer {
	switch strings.ToLower(platform) {
	case "clash":
		return &clashProducer{}
	case "clashmeta", "meta", "mihomo", "clash.meta":
		return &clashMetaProducer{}
	case "surge":
		return &surgeProducer{}
	case "loon":
		return &loonProducer{}
	case "qx", "quantumultx":
		return &qxProducer{}
	case "singbox", "sing-box":
		return &singBoxProducer{}
	case "v2ray", "v2":
		return &v2rayProducer{}
	case "uri":
		return &uriProducer{}
	case "shadowrocket":
		return &shadowrocketProducer{}
	case "stash":
		return &stashProducer{}
	case "surfboard":
		return &surfboardProducer{}
	case "egern":
		return &egernProducer{}
	case "surgemac", "surge-mac":
		return &surgeMacProducer{}
	case "json":
		return &jsonProducer{}
	default:
		return nil
	}
}

var clashCipherList = []string{
	"aes-128-gcm", "aes-192-gcm", "aes-256-gcm",
	"aes-128-cfb", "aes-192-cfb", "aes-256-cfb",
	"aes-128-ctr", "aes-192-ctr", "aes-256-ctr",
	"rc4-md5", "chacha20-ietf", "xchacha20",
	"chacha20-ietf-poly1305", "xchacha20-ietf-poly1305",
}

type clashProducer struct{}

func (c *clashProducer) Type() string { return "ALL" }

func (c *clashProducer) Produce(proxies []*model.Proxy) (string, error) {
	list := make([]map[string]interface{}, 0, len(proxies))
	for _, p := range proxies {
		if !c.isSupported(p) {
			continue
		}
		m := c.produceSingleMap(p)
		if m != nil {
			list = append(list, m)
		}
	}
	if len(list) == 0 {
		return "proxies: []\n", nil
	}
	root := map[string]interface{}{"proxies": list}
	data, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *clashProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	if !c.isSupported(proxy) {
		return "", fmt.Errorf("proxy type %s not supported by Clash", proxy.Type)
	}
	m := c.produceSingleMap(proxy)
	if m == nil {
		return "", fmt.Errorf("failed to produce proxy")
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *clashProducer) isSupported(p *model.Proxy) bool {
	if p == nil {
		return false
	}
	support := PlatformSupport["clash"]
	if !support[p.Type] {
		return false
	}
	if p.Type == "ss" {
		found := false
		for _, cipher := range clashCipherList {
			if p.Cipher == cipher {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if p.Type == "snell" && p.Version >= 4 {
		return false
	}
	if p.Type == "vless" && (p.Flow != "" || p.RealityOpts != nil) {
		return false
	}
	if p.Network == "ws" && p.WSOpts != nil {
		if v, ok := p.WSOpts["v2ray-http-upgrade"]; ok {
			if b, ok := v.(bool); ok && b {
				return false
			}
		}
	}
	if p.UnderlyingProxy != "" || p.DialerProxy != "" {
		return false
	}
	return true
}

func (c *clashProducer) produceSingleMap(p *model.Proxy) map[string]interface{} {
	m := p.ToMap()

	if p.Type == "vmess" {
		if p.SNI != "" {
			m["servername"] = p.SNI
			delete(m, "sni")
		}
		if p.AEAD {
			if aead, ok := m["aead"]; ok {
				if b, ok := aead.(bool); ok && b {
					m["alterId"] = 0
				}
				delete(m, "aead")
			}
		}
		if p.Cipher != "" {
			m["cipher"] = normalizeClashVmessSecurity(p.Cipher)
		}
	}

	if p.Type == "vless" && p.SNI != "" {
		m["servername"] = p.SNI
		delete(m, "sni")
	}

	if p.Type == "wireguard" {
		if p.KeepAlive > 0 {
			m["keepalive"] = p.KeepAlive
			m["persistent-keepalive"] = p.KeepAlive
		}
		if p.PreSharedKey != "" {
			m["preshared-key"] = p.PreSharedKey
			m["pre-shared-key"] = p.PreSharedKey
		}
	}

	if p.Type == "snell" && p.Version < 3 {
		delete(m, "udp")
	}

	if p.TLSFingerprint != "" {
		m["fingerprint"] = p.TLSFingerprint
		delete(m, "tls-fingerprint")
	}

	forceTLS := map[string]bool{"trojan": true, "tuic": true, "hysteria": true, "hysteria2": true, "juicity": true, "anytls": true, "trusttunnel": true, "naive": true}
	if forceTLS[p.Type] {
		delete(m, "tls")
	}

	delete(m, "subName")
	delete(m, "collectionName")
	delete(m, "id")
	delete(m, "resolved")
	delete(m, "no-resolve")
	delete(m, "ip-cidr")
	delete(m, "ipv6-cidr")
	delete(m, "client-fingerprint")

	for k, v := range m {
		if v == nil || strings.HasPrefix(k, "_") {
			delete(m, k)
		}
	}

	for _, key := range []string{"grpc-opts", "ws-opts", "http-opts", "h2-opts", "reality-opts"} {
		if v, ok := m[key]; ok {
			if vm, ok := v.(map[string]interface{}); ok && len(vm) == 0 {
				delete(m, key)
			}
		}
	}

	return m
}

func normalizeClashVmessSecurity(cipher string) string {
	switch strings.ToLower(cipher) {
	case "auto", "aes-128-gcm", "chacha20-poly1305", "chacha20-ietf-poly1305", "none":
		return "auto"
	case "zero":
		return "zero"
	default:
		return cipher
	}
}

type clashMetaProducer struct{}

func (c *clashMetaProducer) Type() string { return "ALL" }

func (c *clashMetaProducer) Produce(proxies []*model.Proxy) (string, error) {
	list := make([]map[string]interface{}, 0, len(proxies))
	for _, p := range proxies {
		if !c.isSupported(p) {
			continue
		}
		m := c.produceSingleMap(p)
		if m != nil {
			list = append(list, m)
		}
	}
	if len(list) == 0 {
		return "proxies: []\n", nil
	}
	root := map[string]interface{}{"proxies": list}
	data, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *clashMetaProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	if !c.isSupported(proxy) {
		return "", fmt.Errorf("proxy type %s not supported by ClashMeta", proxy.Type)
	}
	m := c.produceSingleMap(proxy)
	if m == nil {
		return "", fmt.Errorf("failed to produce proxy")
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *clashMetaProducer) isSupported(p *model.Proxy) bool {
	if p == nil {
		return false
	}
	support := PlatformSupport["clashmeta"]
	return support[p.Type]
}

func (c *clashMetaProducer) produceSingleMap(p *model.Proxy) map[string]interface{} {
	m := p.ToMap()

	if p.Type == "vmess" && p.SNI != "" {
		m["servername"] = p.SNI
		delete(m, "sni")
	}
	if p.Type == "vless" && p.SNI != "" {
		m["servername"] = p.SNI
		delete(m, "sni")
	}

	if p.Type == "wireguard" {
		if p.KeepAlive > 0 {
			m["keepalive"] = p.KeepAlive
			m["persistent-keepalive"] = p.KeepAlive
		}
		if p.PreSharedKey != "" {
			m["preshared-key"] = p.PreSharedKey
			m["pre-shared-key"] = p.PreSharedKey
		}
	}

	if p.RealityOpts != nil {
		if p.Type == "vless" || p.Type == "ss" {
			m["reality-opts"] = p.RealityOpts
		}
	}

	if p.TLSFingerprint != "" {
		m["client-fingerprint"] = p.ClientFingerprint
		m["fingerprint"] = p.TLSFingerprint
	}

	forceTLS := map[string]bool{"trojan": true, "tuic": true, "hysteria": true, "hysteria2": true, "juicity": true, "anytls": true, "trusttunnel": true, "naive": true}
	if forceTLS[p.Type] {
		delete(m, "tls")
	}

	delete(m, "subName")
	delete(m, "collectionName")
	delete(m, "id")
	delete(m, "resolved")
	delete(m, "no-resolve")
	delete(m, "ip-cidr")
	delete(m, "ipv6-cidr")

	for k, v := range m {
		if v == nil || strings.HasPrefix(k, "_") {
			delete(m, k)
		}
	}

	return m
}

type surgeProducer struct{}

func (s *surgeProducer) Type() string { return "SINGLE" }
func (s *surgeProducer) Produce(proxies []*model.Proxy) (string, error) {
	var lines []string
	for _, p := range proxies {
		if !IsProxySupported("surge", p.Type) {
			continue
		}
		if line, err := s.ProduceSingle(p); err == nil {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), nil
}
func (s *surgeProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	p := normalizer.NormalizeProxy(proxy)
	name := strings.ReplaceAll(p.Name, "=", "\\=")
	switch p.Type {
	case "ss":
		parts := []string{name + "=ss", p.Server, strconv.Itoa(p.Port), "encrypt-method=" + p.Cipher, "password=" + p.Password}
		if p.TLS {
			parts = append(parts, "tls=true")
		}
		if p.SNI != "" {
			parts = append(parts, "sni="+p.SNI)
		}
		if p.SkipCertVerify {
			parts = append(parts, "skip-cert-verify=true")
		}
		if p.TCPFastOpen {
			parts = append(parts, "tfo=true")
		}
		if p.UDP {
			parts = append(parts, "udp-relay=true")
		}
		return strings.Join(parts, ","), nil
	case "vmess":
		parts := []string{name + "=vmess", p.Server, strconv.Itoa(p.Port), "username=" + p.UUID}
		if p.TLS {
			parts = append(parts, "tls=true")
		}
		if p.SNI != "" {
			parts = append(parts, "sni="+p.SNI)
		}
		if p.Network == "ws" {
			parts = append(parts, "ws=true")
			if p.WSOpts != nil {
				if path := model.MapGetString(p.WSOpts, "path"); path != "" {
					parts = append(parts, "ws-path="+path)
				}
				if headers, ok := p.WSOpts["headers"].(map[string]interface{}); ok {
					if host := model.MapGetString(headers, "Host"); host != "" {
						parts = append(parts, "ws-headers=Host:"+host)
					}
				}
			}
		}
		return strings.Join(parts, ","), nil
	case "trojan":
		parts := []string{name + "=trojan", p.Server, strconv.Itoa(p.Port), "password=" + p.Password}
		if p.SNI != "" {
			parts = append(parts, "sni="+p.SNI)
		}
		if p.SkipCertVerify {
			parts = append(parts, "skip-cert-verify=true")
		}
		return strings.Join(parts, ","), nil
	case "hysteria2":
		parts := []string{name + "=hysteria2", p.Server, strconv.Itoa(p.Port), "password=" + p.Password}
		if p.SNI != "" {
			parts = append(parts, "sni="+p.SNI)
		}
		return strings.Join(parts, ","), nil
	case "tuic":
		parts := []string{name + "=tuic-v5", p.Server, strconv.Itoa(p.Port), "password=" + p.Password, "uuid=" + p.UUID}
		if p.SNI != "" {
			parts = append(parts, "sni="+p.SNI)
		}
		return strings.Join(parts, ","), nil
	case "wireguard":
		parts := []string{name + "=wireguard", "server=" + p.Server, "port=" + strconv.Itoa(p.Port)}
		if p.PrivateKey != "" {
			parts = append(parts, "private-key="+p.PrivateKey)
		}
		if p.PublicKey != "" {
			parts = append(parts, "public-key="+p.PublicKey)
		}
		return strings.Join(parts, ","), nil
	default:
		return "", fmt.Errorf("surge: unsupported type %s", p.Type)
	}
}

type loonProducer struct{}

func (l *loonProducer) Type() string { return "SINGLE" }
func (l *loonProducer) Produce(proxies []*model.Proxy) (string, error) {
	var lines []string
	for _, p := range proxies {
		if !IsProxySupported("loon", p.Type) {
			continue
		}
		if line, err := l.ProduceSingle(p); err == nil {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), nil
}
func (l *loonProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	p := normalizer.NormalizeProxy(proxy)
	name := strings.ReplaceAll(p.Name, "=", "\\=")
	switch p.Type {
	case "ss":
		parts := []string{name + "=shadowsocks", p.Server, strconv.Itoa(p.Port), p.Cipher, "password=" + p.Password}
		if p.TLS {
			parts = append(parts, "tls=true")
		}
		if p.SNI != "" {
			parts = append(parts, "sni="+p.SNI)
		}
		return strings.Join(parts, ","), nil
	case "vmess":
		parts := []string{name + "=vmess", p.Server, strconv.Itoa(p.Port), "password=" + p.UUID, "method=" + p.Cipher}
		if p.Network != "" {
			parts = append(parts, "transport="+p.Network)
		}
		if p.TLS {
			parts = append(parts, "tls=true")
		}
		if p.SNI != "" {
			parts = append(parts, "sni="+p.SNI)
		}
		return strings.Join(parts, ","), nil
	case "vless":
		parts := []string{name + "=vless", p.Server, strconv.Itoa(p.Port), "uuid=" + p.UUID}
		if p.Network != "" {
			parts = append(parts, "transport="+p.Network)
		}
		if p.TLS {
			parts = append(parts, "tls=true")
		}
		if p.SNI != "" {
			parts = append(parts, "sni="+p.SNI)
		}
		if p.Flow != "" {
			parts = append(parts, "flow="+p.Flow)
		}
		return strings.Join(parts, ","), nil
	case "trojan":
		parts := []string{name + "=trojan", p.Server, strconv.Itoa(p.Port), "password=" + p.Password}
		if p.TLS {
			parts = append(parts, "tls=true")
		}
		if p.SNI != "" {
			parts = append(parts, "sni="+p.SNI)
		}
		return strings.Join(parts, ","), nil
	case "hysteria2":
		parts := []string{name + "=hysteria2", p.Server, strconv.Itoa(p.Port), "password=" + p.Password}
		if p.SNI != "" {
			parts = append(parts, "sni="+p.SNI)
		}
		return strings.Join(parts, ","), nil
	case "tuic":
		parts := []string{name + "=tuic", p.Server, strconv.Itoa(p.Port), "password=" + p.Password, "uuid=" + p.UUID}
		if p.SNI != "" {
			parts = append(parts, "sni="+p.SNI)
		}
		return strings.Join(parts, ","), nil
	default:
		return "", fmt.Errorf("loon: unsupported type %s", p.Type)
	}
}

type qxProducer struct{}

func (q *qxProducer) Type() string { return "SINGLE" }
func (q *qxProducer) Produce(proxies []*model.Proxy) (string, error) {
	var lines []string
	for _, p := range proxies {
		if !IsProxySupported("qx", p.Type) {
			continue
		}
		if line, err := q.ProduceSingle(p); err == nil {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), nil
}
func (q *qxProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	p := normalizer.NormalizeProxy(proxy)
	switch p.Type {
	case "ss":
		parts := []string{"shadowsocks=" + p.Server + ":" + strconv.Itoa(p.Port), p.Cipher, p.Password}
		if p.TLS {
			parts = append(parts, "over-tls=true")
		}
		if p.SNI != "" {
			parts = append(parts, "tls-host="+p.SNI)
		}
		parts = append(parts, "tag="+p.Name)
		return strings.Join(parts, ","), nil
	case "vmess":
		parts := []string{"vmess=" + p.Server + ":" + strconv.Itoa(p.Port), "method=" + p.Cipher, "password=" + p.UUID}
		if p.Network == "ws" {
			parts = append(parts, "obfs=ws")
		}
		if p.TLS {
			parts = append(parts, "over-tls=true")
		}
		if p.SNI != "" {
			parts = append(parts, "tls-host="+p.SNI)
		}
		parts = append(parts, "tag="+p.Name)
		return strings.Join(parts, ","), nil
	case "vless":
		parts := []string{"vless=" + p.Server + ":" + strconv.Itoa(p.Port), "uuid=" + p.UUID}
		if p.TLS {
			parts = append(parts, "over-tls=true")
		}
		if p.SNI != "" {
			parts = append(parts, "tls-host="+p.SNI)
		}
		parts = append(parts, "tag="+p.Name)
		return strings.Join(parts, ","), nil
	case "trojan":
		parts := []string{"trojan=" + p.Server + ":" + strconv.Itoa(p.Port), "password=" + p.Password, "over-tls=true"}
		if p.SNI != "" {
			parts = append(parts, "tls-host="+p.SNI)
		}
		parts = append(parts, "tag="+p.Name)
		return strings.Join(parts, ","), nil
	case "hysteria2":
		parts := []string{"hysteria2=" + p.Server + ":" + strconv.Itoa(p.Port), "password=" + p.Password}
		if p.SNI != "" {
			parts = append(parts, "tls-host="+p.SNI)
		}
		parts = append(parts, "tag="+p.Name)
		return strings.Join(parts, ","), nil
	case "tuic":
		parts := []string{"tuic=" + p.Server + ":" + strconv.Itoa(p.Port), "password=" + p.Password, "uuid=" + p.UUID}
		parts = append(parts, "tag="+p.Name)
		return strings.Join(parts, ","), nil
	default:
		return "", fmt.Errorf("qx: unsupported type %s", p.Type)
	}
}

type singBoxProducer struct{}

func (sb *singBoxProducer) Type() string { return "ALL" }
func (sb *singBoxProducer) Produce(proxies []*model.Proxy) (string, error) {
	outbounds := make([]map[string]interface{}, 0)
	for _, p := range proxies {
		if !IsProxySupported("singbox", p.Type) {
			continue
		}
		if out := sb.produceSingleMap(p); out != nil {
			outbounds = append(outbounds, out)
		}
	}
	result := map[string]interface{}{"outbounds": outbounds}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
func (sb *singBoxProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	out := sb.produceSingleMap(proxy)
	if out == nil {
		return "", fmt.Errorf("sing-box: failed to produce proxy")
	}
	data, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
func (sb *singBoxProducer) produceSingleMap(p *model.Proxy) map[string]interface{} {
	p = normalizer.NormalizeProxy(p)
	out := map[string]interface{}{"type": sb.mapType(p.Type), "tag": p.Name, "server": p.Server, "server_port": p.Port}
	switch p.Type {
	case "ss":
		out["method"] = p.Cipher
		out["password"] = p.Password
	case "vmess":
		out["uuid"] = p.UUID
		out["alter_id"] = p.AlterID
		out["security"] = p.Cipher
	case "vless":
		out["uuid"] = p.UUID
		if p.Flow != "" {
			out["flow"] = p.Flow
		}
	case "trojan":
		out["password"] = p.Password
	case "hysteria2":
		out["password"] = p.Password
		if p.Obfs != "" {
			out["obfs"] = map[string]interface{}{"type": p.Obfs, "password": p.ObfsPassword}
		}
	case "tuic":
		out["uuid"] = p.UUID
		out["password"] = p.Password
	case "wireguard":
		out["private_key"] = p.PrivateKey
		delete(out, "server")
		delete(out, "server_port")
	case "socks5":
		out["username"] = p.Username
		out["password"] = p.Password
	case "http":
		out["username"] = p.Username
		out["password"] = p.Password
	}
	if p.TLS && p.Type != "wireguard" {
		tls := map[string]interface{}{}
		if p.SNI != "" {
			tls["server_name"] = p.SNI
		}
		if p.SkipCertVerify {
			tls["insecure"] = true
		}
		out["tls"] = tls
	}
	return out
}
func (sb *singBoxProducer) mapType(t string) string {
	switch t {
	case "ss":
		return "shadowsocks"
	case "socks5":
		return "socks"
	default:
		return t
	}
}

type v2rayProducer struct{}

func (v *v2rayProducer) Type() string { return "ALL" }
func (v *v2rayProducer) Produce(proxies []*model.Proxy) (string, error) {
	var lines []string
	for _, p := range proxies {
		if p.Type == "vmess" {
			if line, err := v.ProduceSingle(p); err == nil {
				lines = append(lines, line)
			}
		}
	}
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n"))), nil
}
func (v *v2rayProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	p := normalizer.NormalizeProxy(proxy)
	if p.Type != "vmess" {
		return "", fmt.Errorf("v2ray: only vmess supported")
	}
	obj := map[string]interface{}{"v": "2", "ps": p.Name, "add": p.Server, "port": p.Port, "id": p.UUID, "aid": p.AlterID, "scy": p.Cipher, "net": p.Network, "type": "none"}
	if p.TLS {
		obj["tls"] = "tls"
	}
	data, _ := json.Marshal(obj)
	return "vmess://" + base64.StdEncoding.EncodeToString(data), nil
}

type uriProducer struct{}

func (u *uriProducer) Type() string { return "SINGLE" }
func (u *uriProducer) Produce(proxies []*model.Proxy) (string, error) {
	var lines []string
	for _, p := range proxies {
		if line, err := u.ProduceSingle(p); err == nil {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), nil
}
func (u *uriProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	p := normalizer.NormalizeProxy(proxy)
	switch p.Type {
	case "ss":
		userinfo := base64.StdEncoding.EncodeToString([]byte(p.Cipher + ":" + p.Password))
		s := fmt.Sprintf("ss://%s@%s:%d", userinfo, p.Server, p.Port)
		if p.Name != "" {
			s += "#" + url.PathEscape(p.Name)
		}
		return s, nil
	case "vmess":
		obj := map[string]interface{}{"v": "2", "ps": p.Name, "add": p.Server, "port": p.Port, "id": p.UUID, "aid": p.AlterID, "scy": p.Cipher, "net": p.Network, "type": "none"}
		if p.TLS {
			obj["tls"] = "tls"
		}
		data, _ := json.Marshal(obj)
		return "vmess://" + base64.StdEncoding.EncodeToString(data), nil
	case "vless":
		s := fmt.Sprintf("vless://%s@%s:%d", p.UUID, p.Server, p.Port)
		params := url.Values{}
		if p.Network != "" {
			params.Set("type", p.Network)
		}
		if p.TLS {
			params.Set("security", "tls")
		}
		if p.SNI != "" {
			params.Set("sni", p.SNI)
		}
		if p.Flow != "" {
			params.Set("flow", p.Flow)
		}
		if len(params) > 0 {
			s += "?" + params.Encode()
		}
		if p.Name != "" {
			s += "#" + url.PathEscape(p.Name)
		}
		return s, nil
	case "trojan":
		s := fmt.Sprintf("trojan://%s@%s:%d", p.Password, p.Server, p.Port)
		params := url.Values{}
		if p.SNI != "" {
			params.Set("sni", p.SNI)
		}
		if len(params) > 0 {
			s += "?" + params.Encode()
		}
		if p.Name != "" {
			s += "#" + url.PathEscape(p.Name)
		}
		return s, nil
	case "hysteria2":
		s := fmt.Sprintf("hysteria2://%s@%s:%d", p.Password, p.Server, p.Port)
		params := url.Values{}
		if p.SNI != "" {
			params.Set("sni", p.SNI)
		}
		if len(params) > 0 {
			s += "?" + params.Encode()
		}
		if p.Name != "" {
			s += "#" + url.PathEscape(p.Name)
		}
		return s, nil
	default:
		return "", fmt.Errorf("uri: unsupported type %s", p.Type)
	}
}

type shadowrocketProducer struct {
	clashMetaProducer
}

func (s *shadowrocketProducer) isSupported(p *model.Proxy) bool {
	if p == nil {
		return false
	}
	filtered := map[string]bool{"tailscale": true, "sudoku": true, "naive": true, "openvpn": true, "gost-relay": true}
	if filtered[p.Type] {
		return false
	}
	support := PlatformSupport["shadowrocket"]
	return support[p.Type]
}

func (s *shadowrocketProducer) Produce(proxies []*model.Proxy) (string, error) {
	list := make([]map[string]interface{}, 0, len(proxies))
	for _, p := range proxies {
		if !s.isSupported(p) {
			continue
		}
		m := s.clashMetaProducer.produceSingleMap(p)
		if m != nil {
			list = append(list, m)
		}
	}
	if len(list) == 0 {
		return "proxies: []\n", nil
	}
	root := map[string]interface{}{"proxies": list}
	data, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *shadowrocketProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	if !s.isSupported(proxy) {
		return "", fmt.Errorf("proxy type %s not supported by Shadowrocket", proxy.Type)
	}
	m := s.clashMetaProducer.produceSingleMap(proxy)
	if m == nil {
		return "", fmt.Errorf("failed to produce proxy")
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type stashProducer struct {
	clashMetaProducer
}

func (s *stashProducer) isSupported(p *model.Proxy) bool {
	if p == nil {
		return false
	}
	support := PlatformSupport["stash"]
	return support[p.Type]
}

func (s *stashProducer) Produce(proxies []*model.Proxy) (string, error) {
	list := make([]map[string]interface{}, 0, len(proxies))
	for _, p := range proxies {
		if !s.isSupported(p) {
			continue
		}
		m := s.clashMetaProducer.produceSingleMap(p)
		if m != nil {
			list = append(list, m)
		}
	}
	if len(list) == 0 {
		return "proxies: []\n", nil
	}
	root := map[string]interface{}{"proxies": list}
	data, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *stashProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	if !s.isSupported(proxy) {
		return "", fmt.Errorf("proxy type %s not supported by Stash", proxy.Type)
	}
	m := s.clashMetaProducer.produceSingleMap(proxy)
	if m == nil {
		return "", fmt.Errorf("failed to produce proxy")
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type surfboardProducer struct{}

func (s *surfboardProducer) Type() string { return "SINGLE" }

func (s *surfboardProducer) Produce(proxies []*model.Proxy) (string, error) {
	surge := &surgeProducer{}
	var lines []string
	for _, p := range proxies {
		if !IsProxySupported("surfboard", p.Type) {
			continue
		}
		if line, err := surge.ProduceSingle(p); err == nil {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), nil
}

func (s *surfboardProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	if !IsProxySupported("surfboard", proxy.Type) {
		return "", fmt.Errorf("proxy type %s not supported by Surfboard", proxy.Type)
	}
	surge := &surgeProducer{}
	return surge.ProduceSingle(proxy)
}

type egernProducer struct{}

func (e *egernProducer) Type() string { return "ALL" }

func (e *egernProducer) Produce(proxies []*model.Proxy) (string, error) {
	list := make([]map[string]interface{}, 0, len(proxies))
	for _, p := range proxies {
		if !IsProxySupported("egern", p.Type) {
			continue
		}
		m := e.produceSingleMap(p)
		if m != nil {
			list = append(list, m)
		}
	}
	if len(list) == 0 {
		return "proxies: []\n", nil
	}
	root := map[string]interface{}{"proxies": list}
	data, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (e *egernProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	if !IsProxySupported("egern", proxy.Type) {
		return "", fmt.Errorf("proxy type %s not supported by Egern", proxy.Type)
	}
	m := e.produceSingleMap(proxy)
	if m == nil {
		return "", fmt.Errorf("failed to produce proxy")
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (e *egernProducer) produceSingleMap(p *model.Proxy) map[string]interface{} {
	m := p.ToMap()
	if p.Type == "vmess" && p.SNI != "" {
		m["servername"] = p.SNI
		delete(m, "sni")
	}
	if p.Type == "vless" && p.SNI != "" {
		m["servername"] = p.SNI
		delete(m, "sni")
	}
	forceTLS := map[string]bool{"trojan": true, "tuic": true, "hysteria2": true, "anytls": true}
	if forceTLS[p.Type] {
		delete(m, "tls")
	}
	delete(m, "subName")
	delete(m, "collectionName")
	delete(m, "id")
	delete(m, "resolved")
	delete(m, "no-resolve")
	for k, v := range m {
		if v == nil || strings.HasPrefix(k, "_") {
			delete(m, k)
		}
	}
	return m
}

type surgeMacProducer struct{}

func (s *surgeMacProducer) Type() string { return "SINGLE" }

func (s *surgeMacProducer) Produce(proxies []*model.Proxy) (string, error) {
	surge := &surgeProducer{}
	var lines []string
	for _, p := range proxies {
		if !IsProxySupported("surgemac", p.Type) {
			continue
		}
		if p.Type == "external" {
			continue
		}
		if line, err := surge.ProduceSingle(p); err == nil {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), nil
}

func (s *surgeMacProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	if !IsProxySupported("surgemac", proxy.Type) {
		return "", fmt.Errorf("proxy type %s not supported by SurgeMac", proxy.Type)
	}
	if proxy.Type == "external" {
		return "", fmt.Errorf("external proxy type requires special handling")
	}
	surge := &surgeProducer{}
	return surge.ProduceSingle(proxy)
}

type jsonProducer struct{}

func (j *jsonProducer) Type() string { return "ALL" }

func (j *jsonProducer) Produce(proxies []*model.Proxy) (string, error) {
	list := make([]map[string]interface{}, 0, len(proxies))
	for _, p := range proxies {
		m := p.ToMap()
		if m != nil {
			list = append(list, m)
		}
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (j *jsonProducer) ProduceSingle(proxy *model.Proxy) (string, error) {
	m := proxy.ToMap()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
