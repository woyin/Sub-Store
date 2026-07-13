package producer

import (
	"strings"
	"testing"

	"sub-store/internal/model"
)

func TestURIProducerWritesVCN(t *testing.T) {
	p := &model.Proxy{Type: "vless", UUID: "id", Server: "example.com", Port: 443, VCN: []string{"one.example", "two.example"}}
	got, err := (&uriProducer{}).ProduceSingle(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "vcn=one.example%2Ctwo.example") {
		t.Fatalf("missing VCN: %s", got)
	}
}
