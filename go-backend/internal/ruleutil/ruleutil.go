package ruleutil

import (
	"strings"
)

type Rule struct {
	Type    string
	Content string
	Payload string
	Proxy   string
	NoResolve bool
}

func ParseRules(raw string) []Rule {
	var rules []Rule
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//") {
			continue
		}
		r := parseRuleLine(line)
		if r != nil {
			rules = append(rules, *r)
		}
	}
	return rules
}

func parseRuleLine(line string) *Rule {
	parts := strings.SplitN(line, ",", -1)
	if len(parts) < 2 {
		return nil
	}

	ruleType := strings.TrimSpace(parts[0])
	r := &Rule{Type: ruleType}

	switch strings.ToUpper(ruleType) {
	case "DOMAIN", "DOMAIN-SUFFIX", "DOMAIN-KEYWORD", "DOMAIN-REGEX",
		"IP-CIDR", "IP-CIDR6", "GEOIP", "GEOIP-CN",
		"SRC-IP-CIDR", "SRC-GEOIP",
		"GEOSITE", "IP-ASN", "SRC-IP-ASN",
		"PROCESS-NAME", "PROCESS-PATH", "PROCESS-NAME-REGEX", "PROCESS-PATH-REGEX",
		"DEST-PORT", "SRC-PORT",
		"IN-PORT", "IN-NAME", "IN-TYPE", "IN-USER",
		"SUB-RULE", "RULE-SET", "AND", "OR", "NOT",
		"FINAL", "MATCH":
		if len(parts) >= 2 {
			r.Payload = strings.TrimSpace(parts[1])
		}
		if len(parts) >= 3 {
			r.Proxy = strings.TrimSpace(parts[2])
		}
		for _, p := range parts[3:] {
			if strings.TrimSpace(p) == "no-resolve" {
				r.NoResolve = true
			}
		}
	default:
		r.Content = line
	}

	return r
}

func ProduceSurgeRule(r Rule) string {
	parts := []string{r.Type}
	if r.Payload != "" {
		parts = append(parts, r.Payload)
	}
	if r.Proxy != "" {
		parts = append(parts, r.Proxy)
	}
	if r.NoResolve {
		parts = append(parts, "no-resolve")
	}
	return strings.Join(parts, ",")
}

func ProduceClashRule(r Rule) string {
	parts := []string{r.Type}
	if r.Payload != "" {
		parts = append(parts, r.Payload)
	}
	if r.Proxy != "" {
		parts = append(parts, r.Proxy)
	}
	if r.NoResolve {
		parts = append(parts, "no-resolve")
	}
	return strings.Join(parts, ",")
}

func ProduceLoonRule(r Rule) string {
	return ProduceSurgeRule(r)
}

func ProduceQXRule(r Rule) string {
	switch strings.ToUpper(r.Type) {
	case "DOMAIN":
		return "host = " + r.Payload
	case "DOMAIN-SUFFIX":
		return "host-suffix = " + r.Payload
	case "DOMAIN-KEYWORD":
		return "host-keyword = " + r.Payload
	case "IP-CIDR":
		return "ip-cidr = " + r.Payload
	case "IP-CIDR6":
		return "ip6-cidr = " + r.Payload
	case "GEOIP":
		return "geoip = " + r.Payload
	case "FINAL":
		return "final = " + r.Proxy
	default:
		return ProduceSurgeRule(r)
	}
}
