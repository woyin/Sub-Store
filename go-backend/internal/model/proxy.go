package model

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Proxy struct {
	Type                  string                   `json:"type"`
	Name                  string                   `json:"name"`
	Server                string                   `json:"server"`
	Port                  int                      `json:"port"`
	Password              string                   `json:"password,omitempty"`
	UUID                  string                   `json:"uuid,omitempty"`
	Cipher                string                   `json:"cipher,omitempty"`
	AlterID               int                      `json:"alterId,omitempty"`
	Network               string                   `json:"network,omitempty"`
	TLS                   bool                     `json:"tls,omitempty"`
	SNI                   string                   `json:"sni,omitempty"`
	NameCertVerify        string                   `json:"name-cert-verify,omitempty"`
	VCN                   []string                 `json:"_vcn,omitempty"`
	SkipCertVerify        bool                     `json:"skip-cert-verify,omitempty"`
	WSOpts                map[string]interface{}   `json:"ws-opts,omitempty"`
	HTTPOpts              map[string]interface{}   `json:"http-opts,omitempty"`
	H2Opts                map[string]interface{}   `json:"h2-opts,omitempty"`
	GRPCOpts              map[string]interface{}   `json:"grpc-opts,omitempty"`
	TCPFastOpen           bool                     `json:"tfo,omitempty"`
	UDP                   bool                     `json:"udp,omitempty"`
	Mux                   bool                     `json:"mux,omitempty"`
	Obfs                  string                   `json:"obfs,omitempty"`
	ObfsParam             string                   `json:"obfs-param,omitempty"`
	Protocol              string                   `json:"protocol,omitempty"`
	ProtocolParam         string                   `json:"protocol-param,omitempty"`
	Extra                 map[string]interface{}   `json:"extra,omitempty"`
	Username              string                   `json:"username,omitempty"`
	Host                  string                   `json:"host,omitempty"`
	Path                  string                   `json:"path,omitempty"`
	ServiceName           string                   `json:"grpc-service-name,omitempty"`
	Flow                  string                   `json:"flow,omitempty"`
	RealityOpts           map[string]interface{}   `json:"reality-opts,omitempty"`
	ClientFingerprint     string                   `json:"client-fingerprint,omitempty"`
	TLSFingerprint        string                   `json:"tls-fingerprint,omitempty"`
	ALPN                  []string                 `json:"alpn,omitempty"`
	Plugin                string                   `json:"plugin,omitempty"`
	PluginOpts            interface{}              `json:"plugin-opts,omitempty"`
	PSK                   string                   `json:"psk,omitempty"`
	Version               int                      `json:"version,omitempty"`
	Mode                  string                   `json:"mode,omitempty"`
	PrivateKey            string                   `json:"private-key,omitempty"`
	PublicKey             string                   `json:"public-key,omitempty"`
	PreSharedKey          string                   `json:"pre-shared-key,omitempty"`
	Reserved              string                   `json:"reserved,omitempty"`
	Peers                 []map[string]interface{} `json:"peers,omitempty"`
	IP                    string                   `json:"ip,omitempty"`
	IPv6                  string                   `json:"ipv6,omitempty"`
	DNS                   []string                 `json:"dns,omitempty"`
	MTU                   int                      `json:"mtu,omitempty"`
	KeepAlive             int                      `json:"keepalive,omitempty"`
	PersistentKeepalive   int                      `json:"persistent-keepalive,omitempty"`
	AllowedIPs            string                   `json:"allowed-ips,omitempty"`
	InterfaceName         string                   `json:"interface-name,omitempty"`
	UnderlyingProxy       string                   `json:"underlying-proxy,omitempty"`
	DialerProxy           string                   `json:"dialer-proxy,omitempty"`
	Token                 string                   `json:"token,omitempty"`
	CongestionController  string                   `json:"congestion-controller,omitempty"`
	UDPRelayMode          string                   `json:"udp-relay-mode,omitempty"`
	FastOpen              bool                     `json:"fast-open,omitempty"`
	Ports                 string                   `json:"ports,omitempty"`
	HopInterval           int                      `json:"hop-interval,omitempty"`
	HopIntervalMax        int                      `json:"hop-interval-max,omitempty"`
	ObfsPassword          string                   `json:"obfs-password,omitempty"`
	Down                  string                   `json:"down,omitempty"`
	Up                    string                   `json:"up,omitempty"`
	Headers               map[string]string        `json:"headers,omitempty"`
	AEAD                  bool                     `json:"aead,omitempty"`
	IPVersion             string                   `json:"ip-version,omitempty"`
	ECN                   bool                     `json:"ecn,omitempty"`
	BlockQUIC             string                   `json:"block-quic,omitempty"`
	IdleSessionTimeout    int                      `json:"idle-session-timeout,omitempty"`
	MaxStreamCount        int                      `json:"max-stream-count,omitempty"`
	UDPOverTCP            bool                     `json:"udp-over-tcp,omitempty"`
	DisableSNI            bool                     `json:"disable-sni,omitempty"`
	Insecure              bool                     `json:"insecure,omitempty"`
	CaStr                 string                   `json:"ca-str,omitempty"`
	ECH                   map[string]interface{}   `json:"ech-opts,omitempty"`
	Smux                  map[string]interface{}   `json:"smux,omitempty"`
	XUDPPacketAddr        bool                     `json:"packet-addr,omitempty"`
	PacketEncoding        string                   `json:"packet-encoding,omitempty"`
	RenameDev             string                   `json:"rename-dev,omitempty"`
	MPTCP                 bool                     `json:"mptcp,omitempty"`
	Override              map[string]interface{}   `json:"override,omitempty"`
	Interface             string                   `json:"interface,omitempty"`
	CA                    string                   `json:"ca,omitempty"`
	BrutalOpts            map[string]interface{}   `json:"brutal-opts,omitempty"`
	Workers               int                      `json:"workers,omitempty"`
	LocalDNS              []string                 `json:"local-dns,omitempty"`
	MaxUDPRelayPacketSize int                      `json:"max-udp-relay-packet-size,omitempty"`
	MaxOpenStreams        int                      `json:"max-open-streams,omitempty"`
	HeartbeatInterval     int                      `json:"heartbeat-interval,omitempty"`
	ReduceRTT             bool                     `json:"reduce-rtt,omitempty"`
	RequestTimeout        int                      `json:"request-timeout,omitempty"`
	ServerKey             string                   `json:"server-key,omitempty"`
	ServerKeyAlgorithm    string                   `json:"server-key-algorithm,omitempty"`
	ObfsOpts              map[string]interface{}   `json:"obfs-opts,omitempty"`
	UDPOverTCPVersion     int                      `json:"udp-over-tcp-version,omitempty"`
	Raw                   map[string]interface{}   `json:"-"`
}

func MapGetString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func MapGetBool(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func MapGetInt(m map[string]interface{}, key string) int {
	if m == nil {
		return 0
	}
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case float64:
			return int(val)
		case string:
			if i, err := strconv.Atoi(val); err == nil {
				return i
			}
		}
	}
	return 0
}

func MapGetStringSlice(m map[string]interface{}, key string) []string {
	if m == nil {
		return nil
	}
	if v, ok := m[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			res := make([]string, 0, len(arr))
			for _, item := range arr {
				res = append(res, fmt.Sprintf("%v", item))
			}
			return res
		}
		if arr, ok := v.([]string); ok {
			return arr
		}
	}
	return nil
}

func MapGetIntSlice(m map[string]interface{}, key string) []int {
	if m == nil {
		return nil
	}
	if v, ok := m[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			res := make([]int, 0, len(arr))
			for _, item := range arr {
				switch val := item.(type) {
				case int:
					res = append(res, val)
				case float64:
					res = append(res, int(val))
				}
			}
			return res
		}
		if arr, ok := v.([]int); ok {
			return arr
		}
	}
	return nil
}

func CloneMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case map[string]interface{}:
			dst[k] = CloneMap(val)
		case []interface{}:
			arr := make([]interface{}, len(val))
			copy(arr, val)
			dst[k] = arr
		default:
			dst[k] = val
		}
	}
	return dst
}

func ProxyFromMap(m map[string]interface{}) *Proxy {
	data, _ := json.Marshal(m)
	var p Proxy
	_ = json.Unmarshal(data, &p)
	return &p
}

func (p *Proxy) ToMap() map[string]interface{} {
	data, _ := json.Marshal(p)
	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	return m
}

func (p *Proxy) HasNonBlankValue(field string) bool {
	m := p.ToMap()
	if v, ok := m[field]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s) != ""
		}
		return v != nil
	}
	return false
}
