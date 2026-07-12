package app

import (
	"fmt"
	"strings"
	"time"

	"sub-store/internal/gist"
	"sub-store/internal/model"
	"sub-store/internal/normalizer"
	"sub-store/internal/parser"
	"sub-store/internal/processor"
	"sub-store/internal/producer"
	"sub-store/internal/store"
)

// shouldSyncArtifact 判断 artifact 是否应参与同步。
func shouldSyncArtifact(artifact model.Artifact) bool {
	return artifact.Sync && artifact.Source != ""
}

// shouldUploadArtifact 判断 artifact 是否应上传到 Gist。
func shouldUploadArtifact(artifact model.Artifact) bool {
	return artifact.Upload
}

// formatArtifactLogName 格式化 artifact 日志名称。
func formatArtifactLogName(artifact model.Artifact) string {
	if artifact.DisplayName != "" {
		return fmt.Sprintf("%s (%s)", artifact.DisplayName, artifact.Name)
	}
	return artifact.Name
}

// SyncArtifacts 同步所有启用了 sync 的 artifact 到 Gist。
// 对应 Node.js 版本的 syncArtifacts 实现。
func (a *App) SyncArtifacts() error {
	artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
	files := make(map[string]map[string]string)

	valid := []string{}
	invalid := []string{}
	producedWithoutUpload := []string{}

	allSubs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
	allCols := store.GetList[model.Collection](a.Store, model.COLLECTIONS_KEY)

	// 收集所有需要预下载的订阅名称
	subNames := []string{}
	enabledCount := 0
	for i := range artifacts {
		art := &artifacts[i]
		if !shouldSyncArtifact(*art) {
			continue
		}
		enabledCount++
		artifactType := strings.ToLower(art.Type)
		if artifactType == "subscription" || artifactType == "sub" {
			subName := art.Source
			sub := store.FindByName(allSubs, subName)
			if sub != nil && sub.URL != "" && !containsString(subNames, subName) {
				subNames = append(subNames, subName)
			}
		} else if artifactType == "collection" || artifactType == "col" {
			collection := store.FindByName(allCols, art.Source)
			if collection != nil && len(collection.Subscriptions) > 0 {
				for _, sName := range collection.Subscriptions {
					sub := store.FindByName(allSubs, sName)
					if sub != nil && sub.URL != "" && !containsString(subNames, sName) {
						subNames = append(subNames, sName)
					}
				}
			}
		}
	}

	if enabledCount == 0 {
		a.Info(fmt.Sprintf("需同步的配置: %d, 总数: %d", enabledCount, len(artifacts)))
		return nil
	}

	// 预下载所有相关订阅（触发缓存）
	if len(subNames) > 0 {
		a.Info(fmt.Sprintf("预下载 %d 个相关订阅...", len(subNames)))
		for _, subName := range subNames {
			a.Info(fmt.Sprintf("预拉取订阅: %s", subName))
			_, _ = a.produceArtifact("subscription", subName, "JSON")
		}
	}

	// 为每个 artifact 生成产物
	for i := range artifacts {
		art := &artifacts[i]
		if !shouldSyncArtifact(*art) {
			continue
		}

		a.Info(fmt.Sprintf("正在同步云配置：%s...", formatArtifactLogName(*art)))

		output, err := a.produceSyncArtifactOutput(art)
		if err != nil {
			a.Error(fmt.Sprintf("生成同步配置 %s 发生错误: %v", formatArtifactLogName(*art), err))
			invalid = append(invalid, art.Name)
			continue
		}

		if shouldUploadArtifact(*art) {
			encodedName := encodeURIComponent(art.Name)
			files[encodedName] = map[string]string{"content": output}
			valid = append(valid, art.Name)
		} else {
			art.Updated = time.Now().UnixMilli()
			art.URL = ""
			producedWithoutUpload = append(producedWithoutUpload, art.Name)
		}
	}

	producedCount := len(valid) + len(producedWithoutUpload)
	a.Info(fmt.Sprintf("%d 个同步配置生成成功: %s", producedCount, strings.Join(append(valid, producedWithoutUpload...), ", ")))
	if len(invalid) > 0 {
		a.Info(fmt.Sprintf("%d 个同步配置生成失败: %s", len(invalid), strings.Join(invalid, ", ")))
	}
	if len(producedWithoutUpload) > 0 {
		a.Info(fmt.Sprintf("%d 个同步配置仅生成未上传: %s", len(producedWithoutUpload), strings.Join(producedWithoutUpload, ", ")))
	}

	if producedCount == 0 {
		return fmt.Errorf("同步配置 %s 生成失败 详情请查看日志", strings.Join(invalid, ", "))
	}

	// 上传到 Gist
	if len(valid) > 0 {
		err := a.uploadArtifactsToGist(files, valid, &invalid)
		if err != nil {
			return err
		}
	}

	// 保存更新后的 artifacts
	store.SaveList(a.Store, model.ARTIFACTS_KEY, artifacts)
	a.Info("同步配置执行完成")

	if len(invalid) > 0 {
		return fmt.Errorf("同步配置成功 %d 个, 失败 %d 个, 详情请查看日志", len(valid)+len(producedWithoutUpload), len(invalid))
	}
	a.Info(fmt.Sprintf("同步配置成功 %d 个", len(valid)+len(producedWithoutUpload)))
	return nil
}

// SyncArtifact 同步单个 artifact。
func (a *App) SyncArtifact(name string) (*model.Artifact, error) {
	artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
	artifact := store.FindByName(artifacts, name)
	if artifact == nil {
		return nil, fmt.Errorf("找不到远程配置 %s", name)
	}
	if artifact.Source == "" {
		return nil, fmt.Errorf("远程配置 %s 未设置来源", formatArtifactLogName(*artifact))
	}

	a.Info(fmt.Sprintf("开始同步远程配置 %s...", formatArtifactLogName(*artifact)))

	output, err := a.produceSyncArtifactOutput(artifact)
	if err != nil {
		return nil, fmt.Errorf("生成同步配置 %s 失败: %w", formatArtifactLogName(*artifact), err)
	}

	if !shouldUploadArtifact(*artifact) {
		a.Info(fmt.Sprintf("配置 %s 已关闭上传, 仅更新执行时间", formatArtifactLogName(*artifact)))
		artifact.Updated = time.Now().UnixMilli()
		artifact.URL = ""
		store.UpdateByName(artifacts, name, *artifact)
		store.SaveList(a.Store, model.ARTIFACTS_KEY, artifacts)
		return artifact, nil
	}

	// 上传到 Gist
	files := map[string]map[string]string{
		encodeURIComponent(artifact.Name): {"content": output},
	}

	valid := []string{artifact.Name}
	invalid := []string{}
	err = a.uploadArtifactsToGist(files, valid, &invalid)
	if err != nil {
		return nil, err
	}

	// 重新读取更新后的 artifact
	artifacts = store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
	artifact = store.FindByName(artifacts, name)
	return artifact, nil
}

// produceSyncArtifactOutput 生成 artifact 的同步输出内容。
func (a *App) produceSyncArtifactOutput(artifact *model.Artifact) (string, error) {
	output, err := a.produceArtifact(artifact.Type, artifact.Source, artifact.Platform)
	if err != nil {
		return "", err
	}

	// TODO: 集成 AGE 输出加密
	// 参考 Node.js 的 applyAgeOutputEncryption

	return output, nil
}

// produceArtifact 生成指定类型和名称的产物。
// 对应 handler.go 中的 produceArtifact 函数逻辑。
func (a *App) produceArtifact(artifactType, name, target string) (string, error) {
	platform := strings.ToLower(target)
	if platform == "" {
		platform = "json"
	}

	prod := producer.GetProducer(platform)
	if prod == nil {
		return "", fmt.Errorf("unsupported target platform: %s", target)
	}

	var proxies []*model.Proxy
	var err error

	lowercaseType := strings.ToLower(artifactType)
	switch lowercaseType {
	case "subscription", "sub":
		proxies, err = a.processSubscription(name)
	case "collection", "col":
		proxies, err = a.processCollection(name)
	default:
		return "", fmt.Errorf("unsupported artifact type: %s", artifactType)
	}
	if err != nil {
		return "", err
	}

	for i, p := range proxies {
		proxies[i] = normalizer.NormalizeProxy(p)
	}

	output, err := prod.Produce(proxies)
	if err != nil {
		return "", fmt.Errorf("produce failed: %w", err)
	}
	return output, nil
}

// processSubscription 处理订阅并返回代理列表。
func (a *App) processSubscription(name string) ([]*model.Proxy, error) {
	subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
	sub := store.FindByName(subs, name)
	if sub == nil {
		return nil, fmt.Errorf("subscription %s not found", name)
	}

	var rawContent string
	if sub.Content != "" {
		rawContent = sub.Content
	} else if sub.URL != "" {
		// 使用第一个 URL 的内容
		urls := strings.Split(sub.URL, "\n")
		var contents []string
		for _, u := range urls {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			// 由于避免循环依赖的限制，直接使用 app 层的 HTTP 获取
			content, err := a.downloadContent(u, sub.UA)
			if err != nil {
				return nil, fmt.Errorf("fetch subscription URL failed: %w", err)
			}
			contents = append(contents, content)
		}
		rawContent = strings.Join(contents, "\n")
	} else {
		return nil, fmt.Errorf("subscription has no URL or content")
	}

	if rawContent == "" {
		return nil, fmt.Errorf("subscription %s has empty content", name)
	}

	if sub.MergeSources != "" {
		var localContent string
		if sub.Content != "" {
			localContent = sub.Content
		}
		var remoteContent string
		if sub.URL != "" && rawContent != localContent {
			remoteContent = rawContent
		}
		switch sub.MergeSources {
		case "localFirst":
			if localContent != "" && remoteContent != "" {
				rawContent = localContent + "\n" + remoteContent
			}
		case "remoteFirst":
			if localContent != "" && remoteContent != "" {
				rawContent = remoteContent + "\n" + localContent
			}
		}
	}

	proxies, err := parser.ParseContent(rawContent)
	if err != nil {
		return nil, fmt.Errorf("parse subscription failed: %w", err)
	}

	proxies, err = applyProcess(proxies, sub.Process)
	if err != nil {
		return nil, fmt.Errorf("process subscription failed: %w", err)
	}

	return proxies, nil
}

// processCollection 处理组合订阅并返回合并后的代理列表。
func (a *App) processCollection(name string) ([]*model.Proxy, error) {
	cols := store.GetList[model.Collection](a.Store, model.COLLECTIONS_KEY)
	col := store.FindByName(cols, name)
	if col == nil {
		return nil, fmt.Errorf("collection %s not found", name)
	}

	subNames := make([]string, len(col.Subscriptions))
	copy(subNames, col.Subscriptions)

	if len(col.SubscriptionTags) > 0 {
		allSubs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
		tagSet := make(map[string]bool, len(col.SubscriptionTags))
		for _, t := range col.SubscriptionTags {
			tagSet[t] = true
		}
		existing := make(map[string]bool, len(subNames))
		for _, n := range subNames {
			existing[n] = true
		}
		for _, sub := range allSubs {
			if existing[sub.Name] || len(sub.Tag) == 0 {
				continue
			}
			for _, t := range sub.Tag {
				if tagSet[t] {
					subNames = append(subNames, sub.Name)
					existing[sub.Name] = true
					break
				}
			}
		}
	}

	var allProxies []*model.Proxy
	for _, subName := range subNames {
		proxies, err := a.processSubscription(subName)
		if err != nil {
			return nil, fmt.Errorf("process subscription %s in collection %s failed: %w", subName, name, err)
		}
		allProxies = append(allProxies, proxies...)
	}

	allProxies, err := applyProcess(allProxies, col.Process)
	if err != nil {
		return nil, fmt.Errorf("process collection %s failed: %w", name, err)
	}

	return allProxies, nil
}

// applyProcess 应用处理器管道。
// 相当于 handler.go 中的 applyProcess。
func applyProcess(proxies []*model.Proxy, ops []model.Operator) ([]*model.Proxy, error) {
	var procs []processor.Processor
	for _, op := range ops {
		p, err := processor.BuildProcessor(op)
		if err != nil {
			continue
		}
		if p != nil {
			procs = append(procs, p)
		}
	}
	if len(procs) == 0 {
		return proxies, nil
	}
	return processor.Pipeline(proxies, procs)
}

// downloadContent 从 URL 下载内容。
// 简化的下载实现，避免与 download 包产生循环依赖。
func (a *App) downloadContent(urlStr, ua string) (string, error) {
	// TODO: 使用全局 download client 或传递 client 引用
	// 目前作为占位实现，实际应使用 download.Client
	if ua == "" {
		ua = a.Config.DefaultUserAgent
	}
	if ua == "" {
		ua = "clash.meta/v1.19.23"
	}
	// 由于避免循环依赖，这里暂时返回空
	// 后续可以通过接口注入 download client
	return "", nil
}

// uploadArtifactsToGist 上传 artifact 文件到 Gist。
func (a *App) uploadArtifactsToGist(files map[string]map[string]string, valid []string, invalid *[]string) error {
	settingsData := a.Store.Read(model.SETTINGS_KEY)
	if settingsData == nil {
		return fmt.Errorf("settings not found")
	}
	settings, ok := settingsData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid settings format")
	}

	gistToken := ""
	if v, ok := settings["gistToken"].(string); ok {
		gistToken = v
	}
	if gistToken == "" {
		return fmt.Errorf("gist token is not configured")
	}

	githubProxy := ""
	if v, ok := settings["githubProxy"].(string); ok {
		githubProxy = v
	}
	githubAPIURL := ""
	if v, ok := settings["githubApiUrl"].(string); ok {
		githubAPIURL = v
	}

	ageSecretKey := ""
	if v, ok := settings["ageSecretKey"].(string); ok {
		ageSecretKey = v
	}

	client := gist.NewClient(gist.Config{
		GistToken:    gistToken,
		GitHubProxy:  githubProxy,
		GitHubAPIURL: githubAPIURL,
		AgeSecretKey: ageSecretKey,
	})

	// 分批上传
	batchSize := 10
	if v, ok := settings["artifactSyncBatchSize"].(float64); ok && v > 0 {
		batchSize = int(v)
	}

	batches := createArtifactUploadBatches(valid, batchSize)

	for batchIndex, batchNames := range batches {
		batchFiles := make(map[string]interface{})
		for _, name := range batchNames {
			encodedName := encodeURIComponent(name)
			if fileData, ok := files[encodedName]; ok {
				batchFiles[encodedName] = fileData
			}
		}

		a.Info(fmt.Sprintf("正在上传第 %d/%d 批同步配置: %s", batchIndex+1, len(batches), strings.Join(batchNames, ", ")))

		result, err := client.Upload(batchFiles)
		if err != nil {
			a.Error(fmt.Sprintf("第 %d/%d 批同步配置上传失败: %s, 原因: %v", batchIndex+1, len(batches), strings.Join(batchNames, ", "), err))
			*invalid = append(*invalid, batchNames...)
			continue
		}

		a.Info(fmt.Sprintf("上传成功: %s", result.URL))

		// 更新 artifact URL
		artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
		for i := range artifacts {
			art := &artifacts[i]
			if art.Sync && art.Source != "" && containsString(batchNames, art.Name) {
				art.Updated = time.Now().UnixMilli()
				art.URL = result.URL
			}
		}
		store.SaveList(a.Store, model.ARTIFACTS_KEY, artifacts)
	}

	return nil
}

// createArtifactUploadBatches 将 artifact 名称分批。
func createArtifactUploadBatches(names []string, batchSize int) [][]string {
	var batches [][]string
	for i := 0; i < len(names); i += batchSize {
		end := i + batchSize
		if end > len(names) {
			end = len(names)
		}
		batches = append(batches, names[i:end])
	}
	return batches
}

// encodeURIComponent 对字符串进行 URL 编码（近似实现）。
func encodeURIComponent(s string) string {
	result := s
	result = strings.ReplaceAll(result, " ", "%20")
	result = strings.ReplaceAll(result, "!", "%21")
	result = strings.ReplaceAll(result, "#", "%23")
	result = strings.ReplaceAll(result, "$", "%24")
	result = strings.ReplaceAll(result, "&", "%26")
	result = strings.ReplaceAll(result, "'", "%27")
	result = strings.ReplaceAll(result, "(", "%28")
	result = strings.ReplaceAll(result, ")", "%29")
	result = strings.ReplaceAll(result, "*", "%2A")
	result = strings.ReplaceAll(result, "+", "%2B")
	result = strings.ReplaceAll(result, ",", "%2C")
	result = strings.ReplaceAll(result, "/", "%2F")
	result = strings.ReplaceAll(result, ":", "%3A")
	result = strings.ReplaceAll(result, ";", "%3B")
	result = strings.ReplaceAll(result, "=", "%3D")
	result = strings.ReplaceAll(result, "?", "%3F")
	result = strings.ReplaceAll(result, "@", "%40")
	result = strings.ReplaceAll(result, "[", "%5B")
	result = strings.ReplaceAll(result, "]", "%5D")
	return result
}

// containsString 检查字符串切片是否包含指定字符串。
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ProduceAllArtifacts 生成所有 artifact 的产物（不上传到 Gist）。
func (a *App) ProduceAllArtifacts() {
	artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
	for _, art := range artifacts {
		if art.Source == "" {
			continue
		}
		a.Info(fmt.Sprintf("Producing artifact: %s (type=%s, platform=%s)", art.Name, art.Type, art.Platform))
		_, err := a.produceArtifact(art.Type, art.Source, art.Platform)
		if err != nil {
			a.Error(fmt.Sprintf("Failed to produce artifact %s: %v", art.Name, err))
		}
	}
}

// PreFetchSubscriptions 预拉取所有订阅内容。
func (a *App) PreFetchSubscriptions() {
	subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
	for _, sub := range subs {
		if sub.URL == "" {
			continue
		}
		a.Info(fmt.Sprintf("Pre-fetching subscription: %s", sub.Name))
		_, _ = a.processSubscription(sub.Name)
	}
}
