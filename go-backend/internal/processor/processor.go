package processor

import (
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"

	"sub-store/internal/geoip"
	"sub-store/internal/model"
)

type Processor interface {
	Name() string
	Process(proxies []*model.Proxy) ([]*model.Proxy, error)
}

type Filter func(proxy *model.Proxy) bool

type Operator func(proxies []*model.Proxy) ([]*model.Proxy, error)

func BuildProcessor(op model.Operator) (Processor, error) {
	if op.Disabled {
		return nil, nil
	}
	switch op.Type {
	case "Useless Filter":
		return NewUselessFilter(), nil
	case "Regex Filter":
		pattern := ""
		keep := true
		if v, ok := op.Args["regex"]; ok {
			pattern = fmt.Sprintf("%v", v)
		}
		if v, ok := op.Args["keep"]; ok {
			if b, ok := v.(bool); ok {
				keep = b
			}
		}
		return NewRegexFilter(pattern, keep)
	case "Type Filter":
		var types []string
		keep := true
		if v, ok := op.Args["types"]; ok {
			if arr, ok := v.([]interface{}); ok {
				for _, t := range arr {
					types = append(types, fmt.Sprintf("%v", t))
				}
			}
		}
		if v, ok := op.Args["keep"]; ok {
			if b, ok := v.(bool); ok {
				keep = b
			}
		}
		return NewTypeFilter(types, keep), nil
	case "Conditional Filter":
		if ruleData, ok := op.Args["rule"]; ok {
			rule := parseConditionalRule(ruleData)
			return NewConditionalFilter(rule), nil
		}
		return nil, fmt.Errorf("conditional filter missing rule")
	case "Quick Setting Operator":
		var udp, tfo, scv, tls13 *bool
		if v, ok := op.Args["udp"]; ok {
			if b, ok := v.(bool); ok {
				udp = &b
			}
		}
		if v, ok := op.Args["tfo"]; ok {
			if b, ok := v.(bool); ok {
				tfo = &b
			}
		}
		if v, ok := op.Args["skip-cert-verify"]; ok {
			if b, ok := v.(bool); ok {
				scv = &b
			}
		}
		if v, ok := op.Args["tls13"]; ok {
			if b, ok := v.(bool); ok {
				tls13 = &b
			}
		}
		return NewQuickSettingOperator(udp, tfo, scv, tls13), nil
	case "Flag Operator":
		mode := "front"
		tw := "cn"
		if v, ok := op.Args["mode"]; ok {
			mode = fmt.Sprintf("%v", v)
		}
		if v, ok := op.Args["tw"]; ok {
			tw = fmt.Sprintf("%v", v)
		}
		return NewFlagOperator(mode, tw), nil
	case "Sort Operator":
		order := "asc"
		if v, ok := op.Args["order"]; ok {
			order = fmt.Sprintf("%v", v)
		}
		return NewSortOperator(order), nil
	case "Regex Sort Operator":
		var patterns []string
		if v, ok := op.Args["expressions"]; ok {
			if arr, ok := v.([]interface{}); ok {
				for _, p := range arr {
					patterns = append(patterns, fmt.Sprintf("%v", p))
				}
			}
		}
		return NewRegexSortOperator(patterns)
	case "Regex Rename Operator":
		pattern := ""
		replacement := ""
		if v, ok := op.Args["regex"]; ok {
			pattern = fmt.Sprintf("%v", v)
		}
		if v, ok := op.Args["replacement"]; ok {
			replacement = fmt.Sprintf("%v", v)
		}
		return NewRegexRenameOperator(pattern, replacement)
	case "Regex Delete Operator":
		var patterns []string
		if v, ok := op.Args["regex"]; ok {
			if arr, ok := v.([]interface{}); ok {
				for _, p := range arr {
					patterns = append(patterns, fmt.Sprintf("%v", p))
				}
			} else if s, ok := v.(string); ok {
				patterns = []string{s}
			}
		}
		return NewRegexDeleteOperator(patterns)
	case "Handle Duplicate Operator":
		template := ""
		if v, ok := op.Args["template"]; ok {
			template = fmt.Sprintf("%v", v)
		}
		return NewHandleDuplicateOperator(template), nil
	case "Resolve Domain Operator":
		concurrency := 5
		if v, ok := op.Args["concurrency"]; ok {
			if n, ok := v.(int); ok {
				concurrency = n
			}
		}
		return NewResolveDomainOperator(concurrency), nil
	case "Script Filter":
		script := ""
		if v, ok := op.Args["script"]; ok {
			script = fmt.Sprintf("%v", v)
		}
		if v, ok := op.Args["content"]; ok {
			script = fmt.Sprintf("%v", v)
		}
		return NewScriptFilter(script)
	case "Script Operator":
		script := ""
		if v, ok := op.Args["script"]; ok {
			script = fmt.Sprintf("%v", v)
		}
		if v, ok := op.Args["content"]; ok {
			script = fmt.Sprintf("%v", v)
		}
		return NewScriptOperator(script)
	case "Add Proxies From Subscription":
		return NewAddProxiesFromSubscriptionOp(op.Args)
	case "Response Transformer":
		return NewResponseTransformer(op.Args)
	case "Region Filter":
		var regions []string
		keep := true
		if v, ok := op.Args["value"]; ok {
			if arr, ok := v.([]interface{}); ok {
				for _, r := range arr {
					regions = append(regions, strings.ToUpper(fmt.Sprintf("%v", r)))
				}
			}
		}
		if v, ok := op.Args["keep"]; ok {
			if b, ok := v.(bool); ok {
				keep = b
			}
		}
		return NewRegionFilter(regions, keep), nil
	default:
		return nil, fmt.Errorf("unknown operator type: %s", op.Type)
	}
}

func parseConditionalRule(data interface{}) ConditionalRule {
	m, ok := data.(map[string]interface{})
	if !ok {
		return ConditionalRule{}
	}
	rule := ConditionalRule{}
	if v, ok := m["operator"].(string); ok {
		rule.Operator = v
	}
	if v, ok := m["attr"].(string); ok {
		rule.Attr = v
	}
	if v, ok := m["proposition"].(string); ok {
		rule.Proposition = v
	}
	rule.Value = m["value"]
	if children, ok := m["child"].([]interface{}); ok {
		for _, c := range children {
			rule.Child = append(rule.Child, parseConditionalRule(c))
		}
	}
	return rule
}

func Pipeline(proxies []*model.Proxy, processors []Processor) ([]*model.Proxy, error) {
	var err error
	for _, p := range processors {
		proxies, err = p.Process(proxies)
		if err != nil {
			return nil, fmt.Errorf("processor %s failed: %w", p.Name(), err)
		}
	}
	return proxies, nil
}

func ApplyFilter(proxies []*model.Proxy, filter Filter) []*model.Proxy {
	var result []*model.Proxy
	for _, p := range proxies {
		if filter(p) {
			result = append(result, p)
		}
	}
	return result
}

type uselessFilter struct{}

func NewUselessFilter() Processor {
	return &uselessFilter{}
}

func (f *uselessFilter) Name() string { return "Useless Filter" }

func (f *uselessFilter) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	return ApplyFilter(proxies, func(p *model.Proxy) bool {
		if p.Server == "" {
			return false
		}
		if p.Port <= 0 || p.Port > 65535 {
			return false
		}
		if p.Name == "" {
			return false
		}
		return true
	}), nil
}

type regexFilter struct {
	pattern *regexp.Regexp
	keep    bool
}

func NewRegexFilter(pattern string, keep bool) (Processor, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}
	return &regexFilter{pattern: re, keep: keep}, nil
}

func (f *regexFilter) Name() string { return "Regex Filter" }

func (f *regexFilter) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	return ApplyFilter(proxies, func(p *model.Proxy) bool {
		matches := f.pattern.MatchString(p.Name)
		if f.keep {
			return matches
		}
		return !matches
	}), nil
}

type typeFilter struct {
	types map[string]bool
	keep  bool
}

func NewTypeFilter(types []string, keep bool) Processor {
	typeMap := make(map[string]bool)
	for _, t := range types {
		typeMap[strings.ToLower(t)] = true
	}
	return &typeFilter{types: typeMap, keep: keep}
}

func (f *typeFilter) Name() string { return "Type Filter" }

func (f *typeFilter) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	return ApplyFilter(proxies, func(p *model.Proxy) bool {
		found := f.types[strings.ToLower(p.Type)]
		if f.keep {
			return found
		}
		return !found
	}), nil
}

type conditionalFilter struct {
	rule ConditionalRule
}

type ConditionalRule struct {
	Operator   string
	Child      []ConditionalRule
	Attr       string
	Proposition string
	Value      interface{}
}

func NewConditionalFilter(rule ConditionalRule) Processor {
	return &conditionalFilter{rule: rule}
}

func (f *conditionalFilter) Name() string { return "Conditional Filter" }

func (f *conditionalFilter) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	return ApplyFilter(proxies, func(p *model.Proxy) bool {
		return isMatch(f.rule, p)
	}), nil
}

func isMatch(rule ConditionalRule, p *model.Proxy) bool {
	if rule.Operator == "" {
		attrVal := getProxyAttr(p, rule.Attr)
		switch rule.Proposition {
		case "IN":
			if vals, ok := rule.Value.([]interface{}); ok {
				for _, v := range vals {
					if fmt.Sprintf("%v", v) == fmt.Sprintf("%v", attrVal) {
						return true
					}
				}
			}
			return false
		case "CONTAINS":
			return strings.Contains(fmt.Sprintf("%v", attrVal), fmt.Sprintf("%v", rule.Value))
		case "EQUALS":
			return fmt.Sprintf("%v", attrVal) == fmt.Sprintf("%v", rule.Value)
		case "EXISTS":
			return attrVal != nil && attrVal != ""
		default:
			return false
		}
	}

	switch rule.Operator {
	case "AND":
		for _, child := range rule.Child {
			if !isMatch(child, p) {
				return false
			}
		}
		return true
	case "OR":
		for _, child := range rule.Child {
			if isMatch(child, p) {
				return true
			}
		}
		return false
	case "NOT":
		if len(rule.Child) > 0 {
			return !isMatch(rule.Child[0], p)
		}
		return false
	default:
		return false
	}
}

func getProxyAttr(p *model.Proxy, attr string) interface{} {
	switch attr {
	case "name":
		return p.Name
	case "type":
		return p.Type
	case "server":
		return p.Server
	case "port":
		return p.Port
	case "password":
		return p.Password
	case "uuid":
		return p.UUID
	case "cipher":
		return p.Cipher
	case "network":
		return p.Network
	case "tls":
		return p.TLS
	case "sni":
		return p.SNI
	case "udp":
		return p.UDP
	case "tfo":
		return p.TCPFastOpen
	default:
		return ""
	}
}

type quickSettingOperator struct {
	udp            *bool
	tfo            *bool
	skipCertVerify *bool
	tls13          *bool
}

func NewQuickSettingOperator(udp, tfo, skipCertVerify, tls13 *bool) Processor {
	return &quickSettingOperator{
		udp:            udp,
		tfo:            tfo,
		skipCertVerify: skipCertVerify,
		tls13:          tls13,
	}
}

func (o *quickSettingOperator) Name() string { return "Quick Setting Operator" }

func (o *quickSettingOperator) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	for _, p := range proxies {
		if o.udp != nil {
			p.UDP = *o.udp
		}
		if o.tfo != nil {
			p.TCPFastOpen = *o.tfo
		}
		if o.skipCertVerify != nil {
			p.SkipCertVerify = *o.skipCertVerify
		}
	}
	return proxies, nil
}

type flagOperator struct {
	mode string
	tw   string
}

func NewFlagOperator(mode, tw string) Processor {
	return &flagOperator{mode: mode, tw: tw}
}

func (o *flagOperator) Name() string { return "Flag Operator" }

func (o *flagOperator) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	if o.mode == "remove" {
		for _, p := range proxies {
			p.Name = removeFlag(p.Name)
		}
		return proxies, nil
	}

	for _, p := range proxies {
		p.Name = removeFlag(p.Name)
		server := p.Server
		if server != "" {
			flag := getFlag(server)
			if flag != "" {
				if o.mode == "front" {
					p.Name = flag + " " + p.Name
				} else {
					p.Name = p.Name + " " + flag
				}
			}
		}
	}
	return proxies, nil
}

func removeFlag(name string) string {
	name = strings.TrimSpace(name)
	for {
		if len(name) > 0 && name[0] >= 0xE0 && name[0] <= 0xFF {
			remaining := strings.TrimLeft(name[1:], " ")
			if remaining != name {
				name = remaining
				continue
			}
		}
		break
	}
	return strings.TrimSpace(name)
}

func getFlag(server string) string {
	if model.IsIP(server) {
		return getFlagForIP(server)
	}
	parts := strings.Split(server, ".")
	if len(parts) < 2 {
		return ""
	}
	tld := strings.ToLower(parts[len(parts)-1])
	flags := map[string]string{
		"cn": "🇨🇳", "hk": "🇭🇰", "tw": "🇹🇼", "sg": "🇸🇬", "jp": "🇯🇵",
		"us": "🇺🇸", "uk": "🇬🇧", "gb": "🇬🇧", "de": "🇩🇪", "fr": "🇫🇷",
		"kr": "🇰🇷", "in": "🇮🇳", "ru": "🇷🇺", "au": "🇦🇺", "ca": "🇨🇦",
		"nl": "🇳🇱", "it": "🇮🇹", "es": "🇪🇸", "br": "🇧🇷", "mx": "🇲🇽",
		"se": "🇸🇪", "no": "🇳🇴", "fi": "🇫🇮", "dk": "🇩🇰", "pl": "🇵🇱",
		"cz": "🇨🇿", "at": "🇦🇹", "ch": "🇨🇭", "be": "🇧🇪", "ie": "🇮🇪",
		"pt": "🇵🇹", "tr": "🇹🇷", "il": "🇮🇱", "ae": "🇦🇪", "sa": "🇸🇦",
		"th": "🇹🇭", "vn": "🇻🇳", "my": "🇲🇾", "ph": "🇵🇭", "id": "🇮🇩",
		"nz": "🇳🇿", "za": "🇿🇦", "ar": "🇦🇷", "cl": "🇨🇱", "co": "🇨🇴",
		"ua": "🇺🇦", "ro": "🇷🇴", "hu": "🇭🇺", "bg": "🇧🇬", "hr": "🇭🇷",
		"sk": "🇸🇰", "si": "🇸🇮", "lt": "🇱🇹", "lv": "🇱🇻", "ee": "🇪🇪",
		"is": "🇮🇸", "lu": "🇱🇺", "mt": "🇲🇹", "cy": "🇨🇾", "gr": "🇬🇷",
		"pk": "🇵🇰", "bd": "🇧🇩", "lk": "🇱🇰", "np": "🇳🇵", "mm": "🇲🇲",
		"kh": "🇰🇭", "la": "🇱🇦", "mn": "🇲🇳", "kz": "🇰🇿", "uz": "🇺🇿",
		"eg": "🇪🇬", "ng": "🇳🇬", "ke": "🇰🇪", "gh": "🇬🇭", "ma": "🇲🇦",
	}
	if flag, ok := flags[tld]; ok {
		return flag
	}
	return ""
}

func getFlagForIP(ip string) string {
	// Try MMDB lookup first
	if geoip.IsMMDBReady() {
		isoCode := geoip.GeoIP(ip)
		if isoCode != "" {
			if flag, ok := isoToFlag(isoCode); ok {
				return flag
			}
		}
	}
	// Fallback: crude first-octet heuristic
	firstOctet := strings.SplitN(ip, ".", 2)[0]
	switch firstOctet {
	case "1", "2", "3", "4", "5", "6", "7", "8", "9",
		"10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20",
		"21", "22", "23", "24", "25", "26", "27", "28", "29", "30":
		return "🇺🇸"
	}
	return ""
}

// isoToFlag converts a 2-letter ISO country code to a regional indicator emoji flag.
func isoToFlag(code string) (string, bool) {
	if len(code) != 2 {
		return "", false
	}
	c1 := rune(code[0])
	c2 := rune(code[1])
	if c1 < 'A' || c1 > 'Z' || c2 < 'A' || c2 > 'Z' {
		return "", false
	}
	// Regional Indicator Symbols: A=U+1F1E6, B=U+1F1E7, ..., Z=U+1F1FF
	flag := string(rune(0x1F1E6 + int(c1-'A'))) + string(rune(0x1F1E6 + int(c2-'A')))
	return flag, true
}

type sortOperator struct {
	order string
}

func NewSortOperator(order string) Processor {
	return &sortOperator{order: order}
}

func (o *sortOperator) Name() string { return "Sort Operator" }

func (o *sortOperator) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	sort.Slice(proxies, func(i, j int) bool {
		if o.order == "desc" {
			return proxies[i].Name > proxies[j].Name
		}
		return proxies[i].Name < proxies[j].Name
	})
	return proxies, nil
}

type regexSortOperator struct {
	expressions []*regexp.Regexp
}

func NewRegexSortOperator(patterns []string) (Processor, error) {
	var exprs []*regexp.Regexp
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
		exprs = append(exprs, re)
	}
	return &regexSortOperator{expressions: exprs}, nil
}

func (o *regexSortOperator) Name() string { return "Regex Sort Operator" }

func (o *regexSortOperator) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	getOrder := func(name string) int {
		for i, re := range o.expressions {
			if re.MatchString(name) {
				return i
			}
		}
		return len(o.expressions)
	}
	sort.Slice(proxies, func(i, j int) bool {
		return getOrder(proxies[i].Name) < getOrder(proxies[j].Name)
	})
	return proxies, nil
}

type regexRenameOperator struct {
	pattern     *regexp.Regexp
	replacement string
}

func NewRegexRenameOperator(pattern, replacement string) (Processor, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}
	return &regexRenameOperator{pattern: re, replacement: replacement}, nil
}

func (o *regexRenameOperator) Name() string { return "Regex Rename Operator" }

func (o *regexRenameOperator) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	for _, p := range proxies {
		p.Name = o.pattern.ReplaceAllString(p.Name, o.replacement)
	}
	return proxies, nil
}

type regexDeleteOperator struct {
	patterns []*regexp.Regexp
}

func NewRegexDeleteOperator(patterns []string) (Processor, error) {
	var exprs []*regexp.Regexp
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
		exprs = append(exprs, re)
	}
	return &regexDeleteOperator{patterns: exprs}, nil
}

func (o *regexDeleteOperator) Name() string { return "Regex Delete Operator" }

func (o *regexDeleteOperator) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	for _, p := range proxies {
		for _, re := range o.patterns {
			p.Name = re.ReplaceAllString(p.Name, "")
		}
	}
	return proxies, nil
}

type handleDuplicateOperator struct {
	template string
}

func NewHandleDuplicateOperator(template string) Processor {
	if template == "" {
		template = "%s %d"
	}
	return &handleDuplicateOperator{template: template}
}

func (o *handleDuplicateOperator) Name() string { return "Handle Duplicate Operator" }

func (o *handleDuplicateOperator) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	seen := make(map[string]int)
	for _, p := range proxies {
		if count, exists := seen[p.Name]; exists {
			seen[p.Name] = count + 1
			p.Name = fmt.Sprintf(o.template, p.Name, seen[p.Name])
		} else {
			seen[p.Name] = 1
		}
	}
	return proxies, nil
}

type resolveDomainOperator struct {
	concurrency int
}

func NewResolveDomainOperator(concurrency int) Processor {
	if concurrency <= 0 {
		concurrency = 5
	}
	return &resolveDomainOperator{concurrency: concurrency}
}

func (o *resolveDomainOperator) Name() string { return "Resolve Domain Operator" }

func (o *resolveDomainOperator) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	sem := make(chan struct{}, o.concurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, p := range proxies {
		if model.IsIP(p.Server) || p.Server == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(proxy *model.Proxy) {
			defer wg.Done()
			defer func() { <-sem }()
			ips, err := net.LookupHost(proxy.Server)
			if err != nil || len(ips) == 0 {
				return
			}
			mu.Lock()
			proxy.Extra = mergeExtra(proxy.Extra, map[string]interface{}{
				"_resolvedIPs":    strings.Join(ips, ","),
				"_originalServer": proxy.Server,
			})
			proxy.Server = ips[0]
			mu.Unlock()
		}(p)
	}
	wg.Wait()
	return proxies, nil
}

func mergeExtra(existing, additions map[string]interface{}) map[string]interface{} {
	if existing == nil {
		existing = make(map[string]interface{})
	}
	for k, v := range additions {
		existing[k] = v
	}
	return existing
}

type regionFilter struct {
	regions map[string]string
	keep    bool
}

func NewRegionFilter(regions []string, keep bool) Processor {
	regionMap := map[string]string{
		"HK": "\U0001F1ED\U0001F1F0",
		"TW": "\U0001F1F9\U0001F1FC",
		"US": "\U0001F1FA\U0001F1F8",
		"SG": "\U0001F1F8\U0001F1EC",
		"JP": "\U0001F1EF\U0001F1F5",
		"UK": "\U0001F1EC\U0001F1E7",
		"GB": "\U0001F1EC\U0001F1E7",
		"DE": "\U0001F1E9\U0001F1EA",
		"KR": "\U0001F1F0\U0001F1F7",
	}
	matched := make(map[string]string)
	for _, r := range regions {
		if flag, ok := regionMap[r]; ok {
			matched[r] = flag
		}
	}
	return &regionFilter{regions: matched, keep: keep}
}

func (f *regionFilter) Name() string { return "Region Filter" }

func (f *regionFilter) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	flagSet := make(map[string]bool)
	for _, flag := range f.regions {
		flagSet[flag] = true
	}

	var result []*model.Proxy
	for _, p := range proxies {
		nameFlag := extractFlagFromName(p.Name)
		matches := flagSet[nameFlag]

		// If no flag in name, try MMDB lookup on server IP
		if !matches && nameFlag == "" && geoip.IsMMDBReady() && p.Server != "" {
			isoCode := geoip.GeoIP(p.Server)
			if isoCode != "" {
				if flag, ok := isoToFlag(isoCode); ok {
					matches = flagSet[flag]
				}
			}
		}

		if f.keep && matches {
			result = append(result, p)
		} else if !f.keep && !matches {
			result = append(result, p)
		}
	}
	return result, nil
}

func extractFlagFromName(name string) string {
	runes := []rune(name)
	if len(runes) >= 2 && runes[0] >= 0x1F1E0 && runes[0] <= 0x1F1FF &&
		runes[1] >= 0x1F1E0 && runes[1] <= 0x1F1FF {
		return string(runes[:2])
	}
	return ""
}
