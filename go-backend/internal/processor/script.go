package processor

import (
	"encoding/json"
	"fmt"

	"sub-store/internal/model"

	"github.com/dop251/goja"
)

type scriptFilter struct {
	script string
}

func NewScriptFilter(script string) (Processor, error) {
	if script == "" {
		return nil, fmt.Errorf("script filter: empty script")
	}
	return &scriptFilter{script: script}, nil
}

func (f *scriptFilter) Name() string { return "Script Filter" }

func (f *scriptFilter) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	vm := goja.New()
	vm.SetFieldNameMapper(goja.UncapFieldNameMapper())

	var result []*model.Proxy
	for _, p := range proxies {
		m := p.ToMap()
		vm.Set("$server", m)
		val, err := vm.RunString(f.script)
		if err != nil {
			continue
		}
		if val.ToBoolean() {
			result = append(result, p)
		}
	}
	return result, nil
}

type scriptOperator struct {
	script string
}

func NewScriptOperator(script string) (Processor, error) {
	if script == "" {
		return nil, fmt.Errorf("script operator: empty script")
	}
	return &scriptOperator{script: script}, nil
}

func (o *scriptOperator) Name() string { return "Script Operator" }

func (o *scriptOperator) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	vm := goja.New()
	vm.SetFieldNameMapper(goja.UncapFieldNameMapper())

	maps := make([]map[string]interface{}, len(proxies))
	for i, p := range proxies {
		maps[i] = p.ToMap()
	}

	data, _ := json.Marshal(maps)
	vm.Set("$servers", string(data))

	var result []*model.Proxy
	for _, p := range proxies {
		m := p.ToMap()
		vm.Set("$server", m)

		_, err := vm.RunString(o.script)
		if err != nil {
			result = append(result, p)
			continue
		}

		updatedVal := vm.Get("$server")
		if updatedVal != nil && !goja.IsUndefined(updatedVal) {
			updatedMap := make(map[string]interface{})
			if obj := updatedVal.ToObject(vm); obj != nil {
				for _, key := range obj.Keys() {
					updatedMap[key] = obj.Get(key).Export()
				}
			}
			if t, ok := updatedMap["type"].(string); ok {
				p.Type = t
			}
			if n, ok := updatedMap["name"].(string); ok {
				p.Name = n
			}
			if s, ok := updatedMap["server"].(string); ok {
				p.Server = s
			}
		}
		result = append(result, p)
	}
	return result, nil
}
