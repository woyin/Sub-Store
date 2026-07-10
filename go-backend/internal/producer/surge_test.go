package producer

import (
	"strings"
	"testing"

	"sub-store/internal/model"
)

func TestSurgeVless(t *testing.T) {
	producer := GetProducer("surge")

	// basic vless
	p := &model.Proxy{
		Type:   "vless",
		Name:   "test-vless",
		Server: "s1.com",
		Port:   443,
		UUID:   "u1",
	}
	out, err := producer.ProduceSingle(p)
	if err != nil {
		t.Fatalf("produce failed: %v", err)
	}
	for _, s := range []string{"test-vless=vless", "s1.com", "443", "uuid=u1"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %s in output: %s", s, out)
		}
	}

	// vless with tls + sni + ws + skip-cert-verify
	p2 := &model.Proxy{
		Type:           "vless",
		Name:           "vl-ws",
		Server:         "s2.com",
		Port:           443,
		UUID:           "u2",
		TLS:            true,
		SNI:            "sni.com",
		SkipCertVerify: true,
		TCPFastOpen:    true,
		UDP:            true,
		Network:        "ws",
		WSOpts: map[string]interface{}{
			"path": "/ws",
			"headers": map[string]interface{}{
				"Host": "h.com",
			},
		},
	}
	out2, err := producer.ProduceSingle(p2)
	if err != nil {
		t.Fatalf("produce failed: %v", err)
	}
	for _, s := range []string{"tls=true", "sni=sni.com", "skip-cert-verify=true", "tfo=true", "udp-relay=true", "ws=true", "ws-path=/ws", "ws-headers=Host:h.com"} {
		if !strings.Contains(out2, s) {
			t.Errorf("missing %s in output: %s", s, out2)
		}
	}

	// vless with reality-opts + flow + tls-fingerprint + alpn
	p3 := &model.Proxy{
		Type:           "vless",
		Name:           "vl-reality",
		Server:         "s3.com",
		Port:           443,
		UUID:           "u3",
		TLS:            true,
		Flow:           "xtls-rprx-vision",
		TLSFingerprint: "fp123",
		ALPN:           []string{"h2", "http/1.1"},
		RealityOpts: map[string]interface{}{
			"public-key": "pk123",
			"short-id":   "sid",
		},
	}
	out3, err := producer.ProduceSingle(p3)
	if err != nil {
		t.Fatalf("produce failed: %v", err)
	}
	for _, s := range []string{"flow=xtls-rprx-vision", `public-key="pk123"`, "short-id=sid", "server-cert-fingerprint-sha256=fp123", `alpn="h2,http/1.1"`} {
		if !strings.Contains(out3, s) {
			t.Errorf("missing %s in output: %s", s, out3)
		}
	}
}

func TestSurgeSsr(t *testing.T) {
	producer := GetProducer("surge")
	p := &model.Proxy{
		Type:          "ssr",
		Name:          "test-ssr",
		Server:        "s1.com",
		Port:          443,
		Cipher:        "aes-256-cfb",
		Password:      "pass",
		Protocol:      "auth_aes128_md5",
		ProtocolParam: "param1",
		Obfs:          "tls1.2_ticket_auth",
		ObfsParam:     "obfs.com",
	}
	out, err := producer.ProduceSingle(p)
	if err != nil {
		t.Fatalf("produce failed: %v", err)
	}
	for _, s := range []string{"test-ssr=ssr", "s1.com", "443", "encrypt-method=aes-256-cfb", "password=pass", "protocol=auth_aes128_md5", "protocol-param=param1", "obfs=tls1.2_ticket_auth", "obfs-param=obfs.com"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %s in output: %s", s, out)
		}
	}
}

func TestSurgeSocks5(t *testing.T) {
	producer := GetProducer("surge")

	// basic socks5
	p := &model.Proxy{
		Type:     "socks5",
		Name:     "test-socks5",
		Server:   "s1.com",
		Port:     1080,
		Username: "user",
		Password: "pass",
		UDP:      true,
	}
	out, err := producer.ProduceSingle(p)
	if err != nil {
		t.Fatalf("produce failed: %v", err)
	}
	for _, s := range []string{"test-socks5=socks5", "s1.com", "1080", `username="user"`, `password="pass"`, "udp-relay=true"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %s in output: %s", s, out)
		}
	}

	// socks5-tls
	p2 := &model.Proxy{
		Type:           "socks5",
		Name:           "socks5-tls",
		Server:         "s2.com",
		Port:           443,
		TLS:            true,
		SNI:            "sni.com",
		SkipCertVerify: true,
		TLSFingerprint: "fp",
	}
	out2, err := producer.ProduceSingle(p2)
	if err != nil {
		t.Fatalf("produce failed: %v", err)
	}
	if !strings.Contains(out2, "socks5-tls") {
		t.Errorf("expected socks5-tls in output: %s", out2)
	}
	for _, s := range []string{"sni=sni.com", "skip-cert-verify=true", "server-cert-fingerprint-sha256=fp"} {
		if !strings.Contains(out2, s) {
			t.Errorf("missing %s in output: %s", s, out2)
		}
	}
}

func TestSurgeHttp(t *testing.T) {
	producer := GetProducer("surge")

	// basic http
	p := &model.Proxy{
		Type:     "http",
		Name:     "test-http",
		Server:   "s1.com",
		Port:     8080,
		Username: "user",
		Password: "pass",
		UDP:      true,
	}
	out, err := producer.ProduceSingle(p)
	if err != nil {
		t.Fatalf("produce failed: %v", err)
	}
	for _, s := range []string{"test-http=http", "s1.com", "8080", `username="user"`, `password="pass"`, "udp-relay=true"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %s in output: %s", s, out)
		}
	}

	// https
	p2 := &model.Proxy{
		Type:           "http",
		Name:           "test-https",
		Server:         "s2.com",
		Port:           443,
		TLS:            true,
		SNI:            "sni.com",
		SkipCertVerify: true,
		TLSFingerprint: "fp",
	}
	out2, err := producer.ProduceSingle(p2)
	if err != nil {
		t.Fatalf("produce failed: %v", err)
	}
	if !strings.Contains(out2, "test-https=https") {
		t.Errorf("expected https in output: %s", out2)
	}
	for _, s := range []string{"sni=sni.com", "skip-cert-verify=true", "server-cert-fingerprint-sha256=fp"} {
		if !strings.Contains(out2, s) {
			t.Errorf("missing %s in output: %s", s, out2)
		}
	}
}

func TestSurgeSnell(t *testing.T) {
	producer := GetProducer("surge")
	p := &model.Proxy{
		Type:        "snell",
		Name:        "test-snell",
		Server:      "s1.com",
		Port:        443,
		Version:     4,
		PSK:         "psk123",
		Mode:        "http",
		Obfs:        "tls",
		Host:        "obfs.com",
		Path:        "/path",
		TCPFastOpen: true,
		UDP:         true,
	}
	out, err := producer.ProduceSingle(p)
	if err != nil {
		t.Fatalf("produce failed: %v", err)
	}
	for _, s := range []string{"test-snell=snell", "s1.com", "443", "version=4", `psk="psk123"`, "mode=http", "obfs=tls", "obfs-host=obfs.com", "obfs-uri=/path", "tfo=true", "udp-relay=true"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %s in output: %s", s, out)
		}
	}
}

func TestSurgeH2Connect(t *testing.T) {
	producer := GetProducer("surge")
	p := &model.Proxy{
		Type:             "h2-connect",
		Name:             "test-h2",
		Server:           "s1.com",
		Port:             443,
		Username:         "user",
		Password:         "pass",
		SNI:              "sni.com",
		SkipCertVerify:   true,
		TLSFingerprint:   "fp",
		ALPN:             []string{"h2"},
		ClientFingerprint: "firefox",
		UDP:              true,
	}
	out, err := producer.ProduceSingle(p)
	if err != nil {
		t.Fatalf("produce failed: %v", err)
	}
	for _, s := range []string{"test-h2=h2-connect", "s1.com", "443", `username="user"`, `password="pass"`, "sni=sni.com", "skip-cert-verify=true", "server-cert-fingerprint-sha256=fp", `alpn="h2"`, "tls-profile=firefox", "udp-relay=true"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %s in output: %s", s, out)
		}
	}
}

func TestSurgeUnsupportedType(t *testing.T) {
	producer := GetProducer("surge")
	p := &model.Proxy{
		Type:   "ssh",
		Name:   "test-ssh",
		Server: "s1.com",
		Port:   22,
	}
	_, err := producer.ProduceSingle(p)
	if err == nil {
		t.Fatal("expected error for unsupported type ssh")
	}
	if !strings.Contains(err.Error(), "unsupported type ssh") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSurgeNameEscaping(t *testing.T) {
	producer := GetProducer("surge")
	p := &model.Proxy{
		Type:   "vless",
		Name:   "a=b",
		Server: "s1.com",
		Port:   443,
		UUID:   "u1",
	}
	out, err := producer.ProduceSingle(p)
	if err != nil {
		t.Fatalf("produce failed: %v", err)
	}
	if !strings.HasPrefix(out, `a\=b=vless`) {
		t.Errorf("expected escaped name prefix, got: %s", out)
	}
}
