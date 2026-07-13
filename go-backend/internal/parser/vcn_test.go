package parser

import "testing"

func TestURIParsesVCN(t *testing.T) {
	for _, uri := range []string{
		"vless://id@example.com:443?security=tls&vcn=one.example%2Ctwo.example#test",
		"trojan://password@example.com:443?vcn=one.example%2Ctwo.example#test",
	} {
		proxy, err := NewURIParser().Parse(uri)
		if err != nil {
			t.Fatal(err)
		}
		if proxy.NameCertVerify != "one.example" || len(proxy.VCN) != 2 || proxy.VCN[1] != "two.example" {
			t.Fatalf("VCN mismatch for %s: %#v", uri, proxy.VCN)
		}
	}
}
