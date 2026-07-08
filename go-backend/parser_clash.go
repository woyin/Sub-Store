package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ClashParser parses Clash YAML/JSON proxy configurations into Proxy structs.
type ClashParser struct{}

// NewClashParser creates a new ClashParser.
func NewClashParser() *ClashParser {
	return &ClashParser{}
}

// Parse parses raw Clash YAML/JSON content and returns a list of Proxy nodes.
func (c *ClashParser) Parse(raw string) ([]*Proxy, error) {
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

	proxies := make([]*Proxy, 0, len(proxiesList))
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

// parseProxy converts a single Clash proxy map to our internal Proxy struct.
func (c *ClashParser) parseProxy(m map[string]interface{}) *Proxy {
	proxyType := strings.ToLower(MapGetString(m, "type"))
	if proxyType == "" {
		return nil
	}

	p := &Proxy{
		Type:   proxyType,
		Name:   MapGetString(m, "name"),
		Server: MapGetString(m, "server"),
		Port:   MapGetInt(m, "port"),
	}

	// Common fields
	p.Password = MapGetString(m, "password")
	p.UUID = MapGetString(m, "uuid")
	p.Cipher = MapGetString(m, "cipher")
	p.AlterID = MapGetInt(m, "alterId")
	p.Network = MapGetString(m, "network")
	p.TLS = MapGetBool(m, "tls")
	p.TCPFastOpen = MapGetBool(m, "tfo")
	p.UDP = MapGetBool(m, "udp")
	p.Mux = MapGetBool(m, "mux")
	p.Obfs = MapGetString(m, "obfs")
	p.ObfsParam = MapGetString(m, "obfs-param")
	p.Protocol = MapGetString(m, "protocol")
	p.ProtocolParam = MapGetString(m, "protocol-param")
	p.Username = MapGetString(m, "username")
	p.Host = MapGetString(m, "host")
	p.Path = MapGetString(m, "path")
	p.Flow = MapGetString(m, "flow")
	p.Plugin = MapGetString(m, "plugin")
	p.PSK = MapGetString(m, "psk")
	p.Version = MapGetInt(m, "version")
	p.Mode = MapGetString(m, "mode")
	p.PrivateKey = MapGetString(m, "private-key")
	p.PublicKey = MapGetString(m, "public-key")
	p.PreSharedKey = MapGetString(m, "pre-shared-key")
	p.IP = MapGetString(m, "ip")
	p.IPv6 = MapGetString(m, "ipv6")
	p.MTU = MapGetInt(m, "mtu")
	p.KeepAlive = MapGetInt(m, "keepalive")
	p.PersistentKeepalive = MapGetInt(m, "persistent-keepalive")
	p.InterfaceName = MapGetString(m, "interface")
	p.UnderlyingProxy = MapGetString(m, "dialer-proxy")
	p.Token = MapGetString(m, "token")
	p.CongestionController = MapGetString(m, "congestion-controller")
	p.UDPRelayMode = MapGetString(m, "udp-relay-mode")
	p.HopInterval = MapGetInt(m, "hop-interval")
	p.ObfsPassword = MapGetString(m, "obfs-password")
	p.Down = MapGetString(m, "down")
	p.Up = MapGetString(m, "up")
	p.AEAD = MapGetBool(m, "aead")
	p.SkipCertVerify = MapGetBool(m, "skip-cert-verify")
	p.Insecure = MapGetBool(m, "insecure")
	p.CaStr = MapGetString(m, "ca-str")
	p.PacketEncoding = MapGetString(m, "packet-encoding")

	// Field mappings
	if sni := MapGetString(m, "servername"); sni != "" {
		p.SNI = sni
	}
	if fp := MapGetString(m, "fingerprint"); fp != "" {
		p.TLSFingerprint = fp
	}
	if fp := MapGetString(m, "tls-fingerprint"); fp != "" {
		p.TLSFingerprint = fp
	}
	if cf := MapGetString(m, "client-fingerprint"); cf != "" {
		p.ClientFingerprint = cf
	}
	if dp := MapGetString(m, "dialer-proxy"); dp != "" {
		p.DialerProxy = dp
	}
	if dp := MapGetString(m, "underlying-proxy"); dp != "" {
		p.UnderlyingProxy = dp
	}

	// Arrays
	p.ALPN = MapGetStringSlice(m, "alpn")
	p.Reserved = MapGetIntSlice(m, "reserved")
	p.AllowedIPs = MapGetStringSlice(m, "allowed-ips")
	p.DNS = MapGetStringSlice(m, "dns")

	// Maps
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
	if v, ok := m["extra"]; ok {
		p.Extra = normalizeMap(v)
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
