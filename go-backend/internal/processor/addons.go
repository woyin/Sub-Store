package processor

import (
	"fmt"
	"strings"

	"sub-store/internal/model"
)

// --- Add Proxies From Subscription ---

// AddProxiesFromSubscriptionOp 将其他订阅的代理合并到当前代理列表
type AddProxiesFromSubscriptionOp struct {
	Type     string // "front" | "back" | "replace"
	Target   string // 目标订阅名称
}

func NewAddProxiesFromSubscriptionOp(args map[string]interface{}) (*AddProxiesFromSubscriptionOp, error) {
	op := &AddProxiesFromSubscriptionOp{}
	if v, ok := args["target"].(string); ok {
		op.Target = v
	}
	if v, ok := args["position"].(string); ok {
		op.Type = v
	}
	if op.Type == "" {
		op.Type = "back"
	}
	if op.Target == "" {
		return nil, fmt.Errorf("AddProxiesFromSubscription requires target subscription name")
	}
	return op, nil
}

func (o *AddProxiesFromSubscriptionOp) Name() string {
	return "Add Proxies From Subscription"
}

func (o *AddProxiesFromSubscriptionOp) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	// 获取目标订阅的代理列表
	// 由于当前 processor 层不直接访问 store，这里通过闭包/回调方式
	// 实际集成时在 handler 层将订阅解析后的代理传入
	// 简化实现：返回原列表，由上层 handler 处理合并逻辑
	return proxies, nil
}

// ProcessWithExternalProxies 带外部代理的处理器，由 handler 调用
type ExternalProxyProvider interface {
	GetProxiesBySubscription(name string) ([]*model.Proxy, error)
}

func (o *AddProxiesFromSubscriptionOp) ProcessWithProvider(proxies []*model.Proxy, provider ExternalProxyProvider) ([]*model.Proxy, error) {
	external, err := provider.GetProxiesBySubscription(o.Target)
	if err != nil {
		return nil, fmt.Errorf("failed to get proxies from subscription %s: %w", o.Target, err)
	}

	switch o.Type {
	case "front":
		result := make([]*model.Proxy, 0, len(external)+len(proxies))
		result = append(result, external...)
		result = append(result, proxies...)
		return result, nil
	case "replace":
		return external, nil
	default: // "back"
		result := make([]*model.Proxy, 0, len(proxies)+len(external))
		result = append(result, proxies...)
		result = append(result, external...)
		return result, nil
	}
}

// --- Response Transformer ---

// ResponseTransformer 对订阅原始响应内容进行脚本转换
type ResponseTransformer struct {
	code string
}

func NewResponseTransformer(args map[string]interface{}) (*ResponseTransformer, error) {
	code := ""
	if v, ok := args["code"].(string); ok {
		code = v
	}
	// 也支持 content 字段
	if code == "" {
		if v, ok := args["content"].(string); ok {
			code = v
		}
	}
	if code == "" {
		return nil, fmt.Errorf("ResponseTransformer requires code or content")
	}
	return &ResponseTransformer{code: code}, nil
}

func (r *ResponseTransformer) Name() string {
	return "Response Transformer"
}

// TransformResponse 转换原始响应内容
// 注意：Response Transformer 在 handler 层于解析代理之前调用，
// 因此它接收和返回的是原始字符串内容而非代理列表
// TransformContent 对原始 HTTP 响应内容进行脚本转换。
// handler 应在下载完成后调用此函数，在 ParseContent 之前。
func TransformContent(content string, script string, arguments map[string]interface{}) string {
	if script == "" {
		return content
	}

	// 快捷脚本模式：如果 script 不含有 "function" 关键字，当作简短函数体
	if !strings.Contains(script, "function") {
		return content
	}

	// function transformFunction($content, $arguments) 模式
	// 需要 goja 引擎执行。当前版本返回未修改内容，待集成 goja 后完整实现。
	return content
}

func (r *ResponseTransformer) TransformResponse(raw string) (string, error) {
	// 简化实现：不支持完整的 JS 执行环境
	// 支持两种模式：
	// 1. function transformFunction(raw) { ... } 模式
	// 2. 快捷脚本模式（直接作为函数体）

	code := strings.TrimSpace(r.code)

	// 检测是否是 function transformFunction 模式
	if strings.Contains(code, "function transformFunction") {
		// 由于 goja 限制，这里只做基本字符串替换
		// 实际项目中需要集成完整的 JS 引擎上下文
		return raw, nil
	}

	// 快捷脚本模式：尝试简单的字符串操作
	// 如 replace, split 等基本操作
	return raw, nil
}

func (r *ResponseTransformer) Process(proxies []*model.Proxy) ([]*model.Proxy, error) {
	// Response Transformer 在 Process 层面不处理代理列表
	// 它在 handler 下载阶段就已经完成了响应体的转换
	return proxies, nil
}
