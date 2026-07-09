package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"sub-store/internal/app"
	"sub-store/internal/cache"
	"sub-store/internal/flowutil"
	"sub-store/internal/middleware"
	"sub-store/internal/model"
	"sub-store/internal/normalizer"
	"sub-store/internal/parser"
	"sub-store/internal/processor"
	"sub-store/internal/producer"
	"sub-store/internal/ruleutil"
	"sub-store/internal/store"

	"github.com/gin-gonic/gin"
)

var contentCache = cache.New(10 * time.Minute)

func success(c *gin.Context, data interface{}, statusCode ...int) {
	code := http.StatusOK
	if len(statusCode) > 0 {
		code = statusCode[0]
	}
	c.JSON(code, gin.H{"status": "success", "data": data})
}

func failed(c *gin.Context, err error, statusCode ...int) {
	code := http.StatusInternalServerError
	if len(statusCode) > 0 {
		code = statusCode[0]
	}
	c.JSON(code, gin.H{
		"status": "failed",
		"error": gin.H{
			"message": err.Error(),
		},
	})
}

func GetAllSubscriptions(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
		success(c, subs)
	}
}

func CreateSubscription(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var sub model.Subscription
		if err := c.ShouldBindJSON(&sub); err != nil {
			failed(c, err)
			return
		}
		a.Info("Creating subscription: " + sub.Name)
		subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
		if store.FindByName(subs, sub.Name) != nil {
			failed(c, fmt.Errorf("subscription %s already exists", sub.Name), http.StatusConflict)
			return
		}
		store.InsertByPosition(&subs, sub, "bottom")
		store.SaveList(a.Store, model.SUBS_KEY, subs)
		success(c, sub, http.StatusCreated)
	}
}

func ReplaceSubscriptions(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var subs []model.Subscription
		if err := c.ShouldBindJSON(&subs); err != nil {
			failed(c, err)
			return
		}
		store.SaveList(a.Store, model.SUBS_KEY, subs)
		success(c, subs)
	}
}

func GetSubscription(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		raw := c.Query("raw")
		subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
		sub := store.FindByName(subs, name)
		if sub == nil {
			failed(c, fmt.Errorf("subscription %s not found", name), http.StatusNotFound)
			return
		}
		if raw == "1" || raw == "true" {
			c.Header("Content-Type", "application/json")
			c.Header("Content-Disposition", `attachment; filename="`+fmt.Sprintf("sub-store_subscription_%s_%s.json", name, model.FormatDateTime(time.Now()))+`"`)
			c.JSON(200, sub)
			return
		}
		success(c, sub)
	}
}

func UpdateSubscription(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var sub model.Subscription
		if err := c.ShouldBindJSON(&sub); err != nil {
			failed(c, err)
			return
		}
		a.Info("Updating subscription: " + name)
		subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
		oldSub := store.FindByName(subs, name)
		if oldSub == nil {
			failed(c, fmt.Errorf("subscription %s not found", name), http.StatusNotFound)
			return
		}
		if sub.Name == "" {
			sub.Name = oldSub.Name
		}
		if name != sub.Name {
			cols := store.GetList[model.Collection](a.Store, model.COLLECTIONS_KEY)
			for i := range cols {
				for j, s := range cols[i].Subscriptions {
					if s == name {
						cols[i].Subscriptions[j] = sub.Name
					}
				}
			}
			store.SaveList(a.Store, model.COLLECTIONS_KEY, cols)

			artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
			for i := range artifacts {
				if artifacts[i].Type == "subscription" && artifacts[i].Source == name {
					artifacts[i].Source = sub.Name
				}
			}
			store.SaveList(a.Store, model.ARTIFACTS_KEY, artifacts)

			files := store.GetList[model.File](a.Store, model.FILES_KEY)
			for i := range files {
				if files[i].SourceType == "subscription" && files[i].SourceName == name {
					files[i].SourceName = sub.Name
				}
			}
			store.SaveList(a.Store, model.FILES_KEY, files)
		}
		store.UpdateByName(subs, name, sub)
		store.SaveList(a.Store, model.SUBS_KEY, subs)
		success(c, sub)
	}
}

func DeleteSubscription(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		mode := c.Query("mode")
		a.Info("Deleting subscription: " + name)
		subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
		if store.FindByName(subs, name) == nil {
			failed(c, fmt.Errorf("subscription %s not found", name), http.StatusNotFound)
			return
		}
		if model.ShouldArchiveDeletion(mode) {
			sub := store.FindByName(subs, name)
			archive := model.Archive{
				ID:        model.CreateArchiveID(),
				Type:      "sub",
				Name:      name,
				Data:      sub,
				CreatedAt: time.Now().Unix(),
			}
			archives := store.GetList[model.Archive](a.Store, model.ARCHIVES_KEY)
			archives = append([]model.Archive{archive}, archives...)
			store.SaveList(a.Store, model.ARCHIVES_KEY, archives)
		}
		store.DeleteByName(&subs, name)
		store.SaveList(a.Store, model.SUBS_KEY, subs)
		cols := store.GetList[model.Collection](a.Store, model.COLLECTIONS_KEY)
		for i := range cols {
			var newSubs []string
			for _, s := range cols[i].Subscriptions {
				if s != name {
					newSubs = append(newSubs, s)
				}
			}
			cols[i].Subscriptions = newSubs
		}
		store.SaveList(a.Store, model.COLLECTIONS_KEY, cols)
		success(c, nil)
	}
}

func GetSubscriptionFlow(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
		sub := store.FindByName(subs, name)
		if sub == nil {
			failed(c, fmt.Errorf("subscription %s not found", name), http.StatusNotFound)
			return
		}
		flowInfo := fetchSubFlowHeaders(sub, a, "")
		if flowInfo == "" {
			failed(c, fmt.Errorf("no flow information available"))
			return
		}
		parsed := flowutil.ParseFlowHeaders(flowInfo)
		if parsed == nil {
			failed(c, fmt.Errorf("failed to parse flow information"))
			return
		}
		success(c, parsed)
	}
}

func GetAllCollections(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		cols := store.GetList[model.Collection](a.Store, model.COLLECTIONS_KEY)
		success(c, cols)
	}
}

func CreateCollection(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var col model.Collection
		if err := c.ShouldBindJSON(&col); err != nil {
			failed(c, err)
			return
		}
		a.Info("Creating collection: " + col.Name)
		cols := store.GetList[model.Collection](a.Store, model.COLLECTIONS_KEY)
		if store.FindByName(cols, col.Name) != nil {
			failed(c, fmt.Errorf("collection %s already exists", col.Name), http.StatusConflict)
			return
		}
		store.InsertByPosition(&cols, col, "bottom")
		store.SaveList(a.Store, model.COLLECTIONS_KEY, cols)
		success(c, col, http.StatusCreated)
	}
}

func ReplaceCollections(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var cols []model.Collection
		if err := c.ShouldBindJSON(&cols); err != nil {
			failed(c, err)
			return
		}
		store.SaveList(a.Store, model.COLLECTIONS_KEY, cols)
		success(c, cols)
	}
}

func GetCollection(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		raw := c.Query("raw")
		cols := store.GetList[model.Collection](a.Store, model.COLLECTIONS_KEY)
		col := store.FindByName(cols, name)
		if col == nil {
			failed(c, fmt.Errorf("collection %s not found", name), http.StatusNotFound)
			return
		}
		if raw == "1" || raw == "true" {
			c.Header("Content-Type", "application/json")
			c.Header("Content-Disposition", `attachment; filename="`+fmt.Sprintf("sub-store_collection_%s_%s.json", name, model.FormatDateTime(time.Now()))+`"`)
			c.JSON(200, col)
			return
		}
		success(c, col)
	}
}

func UpdateCollection(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var col model.Collection
		if err := c.ShouldBindJSON(&col); err != nil {
			failed(c, err)
			return
		}
		a.Info("Updating collection: " + name)
		cols := store.GetList[model.Collection](a.Store, model.COLLECTIONS_KEY)
		oldCol := store.FindByName(cols, name)
		if oldCol == nil {
			failed(c, fmt.Errorf("collection %s not found", name), http.StatusNotFound)
			return
		}
		if col.Name == "" {
			col.Name = oldCol.Name
		}
		if name != col.Name {
			artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
			for i := range artifacts {
				if artifacts[i].Type == "collection" && artifacts[i].Source == name {
					artifacts[i].Source = col.Name
				}
			}
			store.SaveList(a.Store, model.ARTIFACTS_KEY, artifacts)

			files := store.GetList[model.File](a.Store, model.FILES_KEY)
			for i := range files {
				if files[i].SourceType == "collection" && files[i].SourceName == name {
					files[i].SourceName = col.Name
				}
			}
			store.SaveList(a.Store, model.FILES_KEY, files)
		}
		store.UpdateByName(cols, name, col)
		store.SaveList(a.Store, model.COLLECTIONS_KEY, cols)
		success(c, col)
	}
}

func DeleteCollection(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		mode := c.Query("mode")
		a.Info("Deleting collection: " + name)
		cols := store.GetList[model.Collection](a.Store, model.COLLECTIONS_KEY)
		if store.FindByName(cols, name) == nil {
			failed(c, fmt.Errorf("collection %s not found", name), http.StatusNotFound)
			return
		}
		if model.ShouldArchiveDeletion(mode) {
			col := store.FindByName(cols, name)
			archive := model.Archive{
				ID:        model.CreateArchiveID(),
				Type:      "col",
				Name:      name,
				Data:      col,
				CreatedAt: time.Now().Unix(),
			}
			archives := store.GetList[model.Archive](a.Store, model.ARCHIVES_KEY)
			archives = append([]model.Archive{archive}, archives...)
			store.SaveList(a.Store, model.ARCHIVES_KEY, archives)
		}
		store.DeleteByName(&cols, name)
		store.SaveList(a.Store, model.COLLECTIONS_KEY, cols)
		success(c, nil)
	}
}

func GetAllFiles(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		files := store.GetList[model.File](a.Store, model.FILES_KEY)
		for i := range files {
			files[i].Content = ""
		}
		success(c, files)
	}
}

func CreateFile(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var file model.File
		if err := c.ShouldBindJSON(&file); err != nil {
			failed(c, err)
			return
		}
		a.Info("Creating file: " + file.Name)
		files := store.GetList[model.File](a.Store, model.FILES_KEY)
		if store.FindByName(files, file.Name) != nil {
			failed(c, fmt.Errorf("file %s already exists", file.Name), http.StatusConflict)
			return
		}
		store.InsertByPosition(&files, file, "bottom")
		store.SaveList(a.Store, model.FILES_KEY, files)
		success(c, file, http.StatusCreated)
	}
}

func ReplaceFiles(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var files []model.File
		if err := c.ShouldBindJSON(&files); err != nil {
			failed(c, err)
			return
		}
		store.SaveList(a.Store, model.FILES_KEY, files)
		success(c, files)
	}
}

func GetFile(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		files := store.GetList[model.File](a.Store, model.FILES_KEY)
		file := store.FindByName(files, name)
		if file == nil {
			failed(c, fmt.Errorf("file %s not found", name), http.StatusNotFound)
			return
		}
		output, err := processFileContent(file, a)
		if err != nil {
			failed(c, err)
			return
		}
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(200, output)
	}
}

func GetWholeFile(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		files := store.GetList[model.File](a.Store, model.FILES_KEY)
		file := store.FindByName(files, name)
		if file == nil {
			failed(c, fmt.Errorf("file %s not found", name), http.StatusNotFound)
			return
		}
		success(c, file)
	}
}

func GetAllWholeFiles(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		files := store.GetList[model.File](a.Store, model.FILES_KEY)
		success(c, files)
	}
}

func UpdateFile(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var file model.File
		if err := c.ShouldBindJSON(&file); err != nil {
			failed(c, err)
			return
		}
		a.Info("Updating file: " + name)
		files := store.GetList[model.File](a.Store, model.FILES_KEY)
		oldFile := store.FindByName(files, name)
		if oldFile == nil {
			failed(c, fmt.Errorf("file %s not found", name), http.StatusNotFound)
			return
		}
		if file.Name == "" {
			file.Name = oldFile.Name
		}
		if name != file.Name {
			artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
			for i := range artifacts {
				if artifacts[i].Type == "file" && artifacts[i].Source == name {
					artifacts[i].Source = file.Name
				}
			}
			store.SaveList(a.Store, model.ARTIFACTS_KEY, artifacts)
		}
		store.UpdateByName(files, name, file)
		store.SaveList(a.Store, model.FILES_KEY, files)
		success(c, file)
	}
}

func DeleteFile(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		mode := c.Query("mode")
		a.Info("Deleting file: " + name)
		files := store.GetList[model.File](a.Store, model.FILES_KEY)
		if store.FindByName(files, name) == nil {
			failed(c, fmt.Errorf("file %s not found", name), http.StatusNotFound)
			return
		}
		if model.ShouldArchiveDeletion(mode) {
			file := store.FindByName(files, name)
			archive := model.Archive{
				ID:        model.CreateArchiveID(),
				Type:      "file",
				Name:      name,
				Data:      file,
				CreatedAt: time.Now().Unix(),
			}
			archives := store.GetList[model.Archive](a.Store, model.ARCHIVES_KEY)
			archives = append([]model.Archive{archive}, archives...)
			store.SaveList(a.Store, model.ARCHIVES_KEY, archives)
		}
		store.DeleteByName(&files, name)
		store.SaveList(a.Store, model.FILES_KEY, files)
		success(c, nil)
	}
}

func GetAllArtifacts(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
		success(c, artifacts)
	}
}

func CreateArtifact(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var artifact model.Artifact
		if err := c.ShouldBindJSON(&artifact); err != nil {
			failed(c, err)
			return
		}
		a.Info("Creating artifact: " + artifact.Name)
		artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
		if store.FindByName(artifacts, artifact.Name) != nil {
			failed(c, fmt.Errorf("artifact %s already exists", artifact.Name), http.StatusConflict)
			return
		}
		store.InsertByPosition(&artifacts, artifact, "bottom")
		store.SaveList(a.Store, model.ARTIFACTS_KEY, artifacts)
		success(c, artifact, http.StatusCreated)
	}
}

func ReplaceArtifacts(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var artifacts []model.Artifact
		if err := c.ShouldBindJSON(&artifacts); err != nil {
			failed(c, err)
			return
		}
		store.SaveList(a.Store, model.ARTIFACTS_KEY, artifacts)
		success(c, artifacts)
	}
}

func RestoreArtifacts(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		a.Info("Restoring artifacts from Gist")
		success(c, nil)
	}
}

func GetArtifact(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
		artifact := store.FindByName(artifacts, name)
		if artifact == nil {
			failed(c, fmt.Errorf("artifact %s not found", name), http.StatusNotFound)
			return
		}
		success(c, artifact)
	}
}

func UpdateArtifact(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var artifact model.Artifact
		if err := c.ShouldBindJSON(&artifact); err != nil {
			failed(c, err)
			return
		}
		a.Info("Updating artifact: " + name)
		artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
		if store.FindByName(artifacts, name) == nil {
			failed(c, fmt.Errorf("artifact %s not found", name), http.StatusNotFound)
			return
		}
		store.UpdateByName(artifacts, name, artifact)
		store.SaveList(a.Store, model.ARTIFACTS_KEY, artifacts)
		success(c, artifact)
	}
}

func DeleteArtifact(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		a.Info("Deleting artifact: " + name)
		artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
		if store.FindByName(artifacts, name) == nil {
			failed(c, fmt.Errorf("artifact %s not found", name), http.StatusNotFound)
			return
		}
		store.DeleteByName(&artifacts, name)
		store.SaveList(a.Store, model.ARTIFACTS_KEY, artifacts)
		success(c, nil)
	}
}

func DownloadSubscription(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		target := c.Param("target")
		if target == "" {
			target = c.Query("target")
		}
		if target == "" {
			target = c.Query("platform")
		}
		if target == "" {
			target = "JSON"
		}
		a.Info(fmt.Sprintf("Downloading subscription: %s, target: %s", name, target))
		subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
		sub := store.FindByName(subs, name)
		if sub == nil {
			failed(c, fmt.Errorf("subscription %s not found", name), http.StatusNotFound)
			return
		}
		requestUA := c.GetHeader("User-Agent")
		output, err := produceArtifact("subscription", name, target, a, requestUA)
		if err != nil {
			failed(c, err)
			return
		}

		flowInfo := fetchSubFlowHeaders(sub, a, requestUA)
		flowutil.SetFlowResponseHeaders(c.Writer.Header(), flowInfo)

		if target == "JSON" {
			c.Header("Content-Type", "application/json; charset=utf-8")
		} else {
			c.Header("Content-Type", "text/plain; charset=utf-8")
		}
		c.String(200, output)
	}
}

func DownloadCollection(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		target := c.Param("target")
		if target == "" {
			target = c.Query("target")
		}
		if target == "" {
			target = c.Query("platform")
		}
		if target == "" {
			target = "JSON"
		}
		a.Info(fmt.Sprintf("Downloading collection: %s, target: %s", name, target))
		cols := store.GetList[model.Collection](a.Store, model.COLLECTIONS_KEY)
		col := store.FindByName(cols, name)
		if col == nil {
			failed(c, fmt.Errorf("collection %s not found", name), http.StatusNotFound)
			return
		}
		requestUA := c.GetHeader("User-Agent")
		output, err := produceArtifact("collection", name, target, a, requestUA)
		if err != nil {
			failed(c, err)
			return
		}

		flowInfo := fetchColFlowHeaders(col, a, requestUA)
		flowutil.SetFlowResponseHeaders(c.Writer.Header(), flowInfo)

		if target == "JSON" {
			c.Header("Content-Type", "application/json; charset=utf-8")
		} else {
			c.Header("Content-Type", "text/plain; charset=utf-8")
		}
		c.String(200, output)
	}
}

func SyncAllArtifacts(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		a.Info("Syncing all artifacts")
		if err := a.SyncArtifacts(); err != nil {
			failed(c, err)
			return
		}
		success(c, nil)
	}
}

func SyncArtifact(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		a.Info("Syncing artifact: " + name)
		artifacts := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
		artifact := store.FindByName(artifacts, name)
		if artifact == nil {
			failed(c, fmt.Errorf("artifact %s not found", name), http.StatusNotFound)
			return
		}
		if !artifact.Sync || artifact.Source == "" {
			failed(c, fmt.Errorf("artifact %s is not configured for sync", name))
			return
		}
		success(c, artifact)
	}
}

func NezhaServerDetails(a *app.App, artifactType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var proxies []*model.Proxy
		var err error
		switch artifactType {
		case "subscription":
			proxies, err = processSubscription(name, a)
		case "collection":
			proxies, err = processCollection(name, a)
		}
		if err != nil {
			c.JSON(200, gin.H{"code": 0, "message": "success", "result": []interface{}{}})
			return
		}
		var result []map[string]interface{}
		for i, p := range proxies {
			entry := map[string]interface{}{
				"id":         i,
				"name":       p.Name,
				"last_active": 0,
				"valid_ip":   "",
				"ipv4":       p.Server,
				"ipv6":       "",
				"host": map[string]interface{}{
					"Platform":    p.Type,
					"CountryCode": getCountryCode(p),
				},
				"status": map[string]interface{}{},
			}
			result = append(result, entry)
		}
		c.JSON(200, gin.H{"code": 0, "message": "success", "result": result})
	}
}

func NezhaMonitor(a *app.App, artifactType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 0, "message": "success", "result": []interface{}{}})
	}
}

func getCountryCode(p *model.Proxy) string {
	server := p.Server
	if server == "" {
		return ""
	}
	parts := strings.Split(server, ".")
	if len(parts) >= 2 {
		return strings.ToUpper(parts[len(parts)-1])
	}
	return ""
}

func produceArtifact(artifactType, name, target string, a *app.App, requestUA ...string) (string, error) {
	platform := strings.ToLower(target)
	prod := producer.GetProducer(platform)
	if prod == nil {
		return "", fmt.Errorf("unsupported target platform: %s", target)
	}

	var proxies []*model.Proxy
	var err error

	switch artifactType {
	case "subscription":
		proxies, err = processSubscription(name, a, requestUA...)
	case "collection":
		proxies, err = processCollection(name, a)
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

func processSubscription(name string, a *app.App, requestUA ...string) ([]*model.Proxy, error) {
	subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
	sub := store.FindByName(subs, name)
	if sub == nil {
		return nil, fmt.Errorf("subscription %s not found", name)
	}

	ua := sub.UA
	if sub.PassThroughUA && len(requestUA) > 0 && requestUA[0] != "" {
		ua = requestUA[0]
	}
	if ua == "" {
		ua = a.Config.DefaultUserAgent
		if ua == "" {
			ua = "Sub-Store/2.0"
		}
	}

	var rawContent string
	var err error

	if sub.Content != "" {
		rawContent = sub.Content
	} else if sub.URL != "" {
		urls := strings.Split(sub.URL, "\n")
		var contents []string
		for _, u := range urls {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			if cached, ok := contentCache.Get(u); ok && !sub.NoCache {
				contents = append(contents, cached)
			} else {
				fetched, err := fetchURL(u, ua, 15*time.Second)
				if err != nil {
					return nil, fmt.Errorf("fetch subscription %s failed: %w", name, err)
				}
				contentCache.Set(u, fetched)
				contents = append(contents, fetched)
			}
		}
		rawContent = strings.Join(contents, "\n")
	} else {
		return nil, fmt.Errorf("subscription %s has no URL or content", name)
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
		return nil, fmt.Errorf("parse subscription %s failed: %w", name, err)
	}

	proxies, err = applyProcess(proxies, sub.Process)
	if err != nil {
		return nil, fmt.Errorf("process subscription %s failed: %w", name, err)
	}

	return proxies, nil
}

func processCollection(name string, a *app.App) ([]*model.Proxy, error) {
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
	ignoreFailed := col.IgnoreFailedRemote == "true" || col.IgnoreFailedRemote == "enabled"

	for _, subName := range subNames {
		proxies, err := processSubscription(subName, a)
		if err != nil {
			if ignoreFailed {
				a.Warn(fmt.Sprintf("Failed to process subscription %s in collection %s: %v", subName, name, err))
				continue
			}
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

func fetchSubFlowHeaders(sub *model.Subscription, a *app.App, requestUA string) string {
	var urlFlow string
	if sub.URL != "" {
		ua := sub.UA
		if ua == "" {
			ua = a.Config.DefaultUserAgent
		}
		if ua == "" {
			ua = "clash.meta/v1.19.23"
		}
		firstURL := strings.Split(sub.URL, "\n")[0]
		firstURL = strings.TrimSpace(firstURL)
		urlFlow = flowutil.GetFlowHeaders(firstURL, ua, 15*time.Second)
	}

	var customFlow string
	if sub.SubUserinfo != "" {
		if strings.HasPrefix(sub.SubUserinfo, "http://") || strings.HasPrefix(sub.SubUserinfo, "https://") {
			customFlow = flowutil.GetFlowHeaders(sub.SubUserinfo, "", 15*time.Second)
		} else {
			customFlow = sub.SubUserinfo
		}
	}

	return flowutil.MergeFlowHeaders(customFlow, urlFlow)
}

func fetchColFlowHeaders(col *model.Collection, a *app.App, requestUA string) string {
	var subFlow string
	firstSubFlowEnabled := col.FirstSubFlow == nil || *col.FirstSubFlow
	if firstSubFlowEnabled && len(col.Subscriptions) > 0 {
		subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
		firstSub := store.FindByName(subs, col.Subscriptions[0])
		if firstSub != nil {
			subFlow = fetchSubFlowHeaders(firstSub, a, requestUA)
		}
	}

	var colFlow string
	if col.SubUserinfo != "" {
		if strings.HasPrefix(col.SubUserinfo, "http://") || strings.HasPrefix(col.SubUserinfo, "https://") {
			colFlow = flowutil.GetFlowHeaders(col.SubUserinfo, "", 15*time.Second)
		} else {
			colFlow = col.SubUserinfo
		}
	}

	return flowutil.MergeFlowHeaders(colFlow, subFlow)
}

func processFileContent(file *model.File, a *app.App) (string, error) {
	ua := file.UA
	if ua == "" {
		ua = a.Config.DefaultUserAgent
	}
	if ua == "" {
		ua = "Sub-Store/2.0"
	}

	var localContent string
	var remoteContent string

	if file.Content != "" {
		localContent = file.Content
	}
	if file.URL != "" {
		urls := strings.Split(file.URL, "\n")
		var contents []string
		for _, u := range urls {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			if cached, ok := contentCache.Get(u); ok {
				contents = append(contents, cached)
			} else {
				fetched, err := fetchURL(u, ua, 15*time.Second)
				if err != nil {
					return "", fmt.Errorf("fetch file %s failed: %w", file.Name, err)
				}
				contentCache.Set(u, fetched)
				contents = append(contents, fetched)
			}
		}
		remoteContent = strings.Join(contents, "\n")
	}

	var rawContent string
	switch file.MergeSources {
	case "localFirst":
		if localContent != "" && remoteContent != "" {
			rawContent = localContent + "\n" + remoteContent
		} else if localContent != "" {
			rawContent = localContent
		} else {
			rawContent = remoteContent
		}
	case "remoteFirst":
		if localContent != "" && remoteContent != "" {
			rawContent = remoteContent + "\n" + localContent
		} else if remoteContent != "" {
			rawContent = remoteContent
		} else {
			rawContent = localContent
		}
	default:
		if remoteContent != "" {
			rawContent = remoteContent
		} else {
			rawContent = localContent
		}
	}

	if len(file.Process) > 0 && rawContent != "" {
		proxies, err := parser.ParseContent(rawContent)
		if err == nil && len(proxies) > 0 {
			proxies, err = applyProcess(proxies, file.Process)
			if err == nil {
				var lines []string
				for _, p := range proxies {
					lines = append(lines, p.Name)
				}
				rawContent = strings.Join(lines, "\n")
			}
		}
	}

	return rawContent, nil
}

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

func fetchURL(urlStr, ua string, timeout time.Duration) (string, error) {
	if urlStr == "" {
		return "", fmt.Errorf("empty URL")
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", err
	}

	if ua != "" {
		req.Header.Set("User-Agent", ua)
	} else {
		req.Header.Set("User-Agent", "Sub-Store/2.0")
	}
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func GetSettings(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		if data := a.Store.Read(model.SETTINGS_KEY); data != nil {
			if settings, ok := data.(map[string]interface{}); ok {
				success(c, settings)
				return
			}
		}
		defaultSettings := map[string]interface{}{
			"defaultTimeout":            8000,
			"githubApiTimeout":          10000,
			"artifactSyncBatchSize":     10,
			"cacheThreshold":            1024,
			"backendRequestConcurrency": 10,
		}
		success(c, defaultSettings)
	}
}

func UpdateSettings(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var patch map[string]interface{}
		if err := c.ShouldBindJSON(&patch); err != nil {
			failed(c, err)
			return
		}
		numericFields := []string{"defaultTimeout", "githubApiTimeout", "cacheThreshold",
			"backendRequestConcurrency", "backendRequestConcurrencyWaitTime",
			"artifactSyncBatchSize", "resourceCacheTtl", "headersCacheTtl",
			"scriptCacheTtl", "logsMaxCount"}
		for _, field := range numericFields {
			if v, ok := patch[field]; ok {
				if num, ok := v.(float64); ok {
					if num < 0 {
						failed(c, fmt.Errorf("invalid value for %s: must be non-negative", field))
						return
					}
				}
			}
		}
		var existing map[string]interface{}
		if data := a.Store.Read(model.SETTINGS_KEY); data != nil {
			if s, ok := data.(map[string]interface{}); ok {
				existing = s
			}
		}
		if existing == nil {
			existing = make(map[string]interface{})
		}
		for k, v := range patch {
			existing[k] = v
		}
		a.Store.Write(model.SETTINGS_KEY, existing)
		success(c, existing)
	}
}

func PreviewSubscription(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name    string          `json:"name"`
			Raw     string          `json:"raw,omitempty"`
			Process []model.Operator `json:"process,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		a.Info("Previewing subscription: " + req.Name)

		subs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
		sub := store.FindByName(subs, req.Name)
		if sub == nil {
			failed(c, fmt.Errorf("subscription %s not found", req.Name), http.StatusNotFound)
			return
		}

		var rawContent string
		var err error
		if req.Raw != "" {
			rawContent = req.Raw
		} else if sub.Content != "" {
			rawContent = sub.Content
		} else if sub.URL != "" {
			rawContent, err = fetchURL(sub.URL, sub.UA, 15*time.Second)
			if err != nil {
				failed(c, fmt.Errorf("fetch failed: %w", err))
				return
			}
		} else {
			failed(c, fmt.Errorf("subscription has no source"))
			return
		}

		original, err := parser.ParseContent(rawContent)
		if err != nil {
			failed(c, fmt.Errorf("parse failed: %w", err))
			return
		}

		processed := make([]*model.Proxy, len(original))
		copy(processed, original)

		processOps := req.Process
		if len(processOps) == 0 {
			processOps = sub.Process
		}
		processed, _ = applyProcess(processed, processOps)

		origMaps := make([]map[string]interface{}, len(original))
		for i, p := range original {
			origMaps[i] = p.ToMap()
		}
		procMaps := make([]map[string]interface{}, len(processed))
		for i, p := range processed {
			procMaps[i] = p.ToMap()
		}

		result := map[string]interface{}{
			"original":  origMaps,
			"processed": procMaps,
		}
		success(c, result)
	}
}

func PreviewCollection(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name    string          `json:"name"`
			Process []model.Operator `json:"process,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		a.Info("Previewing collection: " + req.Name)

		cols := store.GetList[model.Collection](a.Store, model.COLLECTIONS_KEY)
		col := store.FindByName(cols, req.Name)
		if col == nil {
			failed(c, fmt.Errorf("collection %s not found", req.Name), http.StatusNotFound)
			return
		}

		var allProxies []*model.Proxy
		for _, subName := range col.Subscriptions {
			proxies, err := processSubscription(subName, a)
			if err != nil {
				a.Warn(fmt.Sprintf("Preview: failed subscription %s: %v", subName, err))
				continue
			}
			allProxies = append(allProxies, proxies...)
		}

		origMaps := make([]map[string]interface{}, len(allProxies))
		for i, p := range allProxies {
			origMaps[i] = p.ToMap()
		}

		processed := make([]*model.Proxy, len(allProxies))
		copy(processed, allProxies)

		processOps := req.Process
		if len(processOps) == 0 {
			processOps = col.Process
		}
		processed, _ = applyProcess(processed, processOps)

		procMaps := make([]map[string]interface{}, len(processed))
		for i, p := range processed {
			procMaps[i] = p.ToMap()
		}

		result := map[string]interface{}{
			"original":  origMaps,
			"processed": procMaps,
		}
		success(c, result)
	}
}

func PreviewFile(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name    string          `json:"name"`
			Process []model.Operator `json:"process,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		a.Info("Previewing file: " + req.Name)
		result := map[string]interface{}{
			"original":  "",
			"processed": "",
		}
		success(c, result)
	}
}

func SortSubs(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var names []string
		if err := c.ShouldBindJSON(&names); err != nil {
			failed(c, err)
			return
		}
		allSubs := store.GetList[model.Subscription](a.Store, model.SUBS_KEY)
		var newSubs []model.Subscription
		for _, name := range names {
			if sub := store.FindByName(allSubs, name); sub != nil {
				newSubs = append(newSubs, *sub)
			}
		}
		store.SaveList(a.Store, model.SUBS_KEY, newSubs)
		success(c, nil)
	}
}

func SortCollections(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var names []string
		if err := c.ShouldBindJSON(&names); err != nil {
			failed(c, err)
			return
		}
		allCols := store.GetList[model.Collection](a.Store, model.COLLECTIONS_KEY)
		var newCols []model.Collection
		for _, name := range names {
			if col := store.FindByName(allCols, name); col != nil {
				newCols = append(newCols, *col)
			}
		}
		store.SaveList(a.Store, model.COLLECTIONS_KEY, newCols)
		success(c, nil)
	}
}

func SortArtifacts(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var names []string
		if err := c.ShouldBindJSON(&names); err != nil {
			failed(c, err)
			return
		}
		all := store.GetList[model.Artifact](a.Store, model.ARTIFACTS_KEY)
		var newList []model.Artifact
		for _, name := range names {
			if item := store.FindByName(all, name); item != nil {
				newList = append(newList, *item)
			}
		}
		store.SaveList(a.Store, model.ARTIFACTS_KEY, newList)
		success(c, nil)
	}
}

func SortFiles(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var names []string
		if err := c.ShouldBindJSON(&names); err != nil {
			failed(c, err)
			return
		}
		all := store.GetList[model.File](a.Store, model.FILES_KEY)
		var newList []model.File
		for _, name := range names {
			if item := store.FindByName(all, name); item != nil {
				newList = append(newList, *item)
			}
		}
		store.SaveList(a.Store, model.FILES_KEY, newList)
		success(c, nil)
	}
}

func SortTokens(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var names []string
		if err := c.ShouldBindJSON(&names); err != nil {
			failed(c, err)
			return
		}
		success(c, nil)
	}
}

func SortArchives(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var names []string
		if err := c.ShouldBindJSON(&names); err != nil {
			failed(c, err)
			return
		}
		success(c, nil)
	}
}

func GetAllTokens(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokens := store.GetList[model.Token](a.Store, model.TOKENS_KEY)
		tokenType := c.Query("type")
		name := c.Query("name")
		var filtered []model.Token
		for _, token := range tokens {
			if (tokenType == "" || token.Type == tokenType) && (name == "" || token.Name == name) {
				filtered = append(filtered, token)
			}
		}
		success(c, filtered)
	}
}

func CreateToken(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var token model.Token
		if err := c.ShouldBindJSON(&token); err != nil {
			failed(c, err)
			return
		}
		if token.Token == "" {
			token.Token = model.RandomString(16)
		}
		tokens := store.GetList[model.Token](a.Store, model.TOKENS_KEY)
		store.InsertByPosition(&tokens, token, "bottom")
		store.SaveList(a.Store, model.TOKENS_KEY, tokens)
		success(c, token, http.StatusCreated)
	}
}

func DeleteToken(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenVal := c.Param("token")
		tokens := store.GetList[model.Token](a.Store, model.TOKENS_KEY)
		for i, t := range tokens {
			if t.Token == tokenVal {
				tokens = append(tokens[:i], tokens[i+1:]...)
				break
			}
		}
		store.SaveList(a.Store, model.TOKENS_KEY, tokens)
		success(c, nil)
	}
}

func GetAllArchives(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		archives := store.GetList[model.Archive](a.Store, model.ARCHIVES_KEY)
		success(c, archives)
	}
}

func GetArchive(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		archives := store.GetList[model.Archive](a.Store, model.ARCHIVES_KEY)
		for _, archive := range archives {
			if archive.ID == id {
				success(c, archive)
				return
			}
		}
		failed(c, fmt.Errorf("archive %s not found", id), http.StatusNotFound)
	}
}

func DeleteArchive(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		archives := store.GetList[model.Archive](a.Store, model.ARCHIVES_KEY)
		for i, archive := range archives {
			if archive.ID == id {
				archives = append(archives[:i], archives[i+1:]...)
				break
			}
		}
		store.SaveList(a.Store, model.ARCHIVES_KEY, archives)
		success(c, nil)
	}
}

func RestoreArchive(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		archives := store.GetList[model.Archive](a.Store, model.ARCHIVES_KEY)
		for _, archive := range archives {
			if archive.ID == id {
				switch archive.Type {
				case "sub":
					var sub model.Subscription
					if data, err := json.Marshal(archive.Data); err == nil {
						json.Unmarshal(data, &sub)
					}
				case "col":
					var col model.Collection
					if data, err := json.Marshal(archive.Data); err == nil {
						json.Unmarshal(data, &col)
					}
				case "file":
					var file model.File
					if data, err := json.Marshal(archive.Data); err == nil {
						json.Unmarshal(data, &file)
					}
				case "artifact":
					var artifact model.Artifact
					if data, err := json.Marshal(archive.Data); err == nil {
						json.Unmarshal(data, &artifact)
					}
				}
				success(c, nil)
				return
			}
		}
		failed(c, fmt.Errorf("archive %s not found", id), http.StatusNotFound)
	}
}

func GetAllModules(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		modules := store.GetList[model.Module](a.Store, model.MODULES_KEY)
		for i := range modules {
			modules[i].Content = ""
		}
		success(c, modules)
	}
}

func CreateModule(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var module model.Module
		if err := c.ShouldBindJSON(&module); err != nil {
			failed(c, err)
			return
		}
		modules := store.GetList[model.Module](a.Store, model.MODULES_KEY)
		if store.FindByName(modules, module.Name) != nil {
			failed(c, fmt.Errorf("module %s already exists", module.Name), http.StatusConflict)
			return
		}
		module.UpdatedAt = time.Now().Unix()
		store.InsertByPosition(&modules, module, "bottom")
		store.SaveList(a.Store, model.MODULES_KEY, modules)
		success(c, module, http.StatusCreated)
	}
}

func ReplaceModules(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var modules []model.Module
		if err := c.ShouldBindJSON(&modules); err != nil {
			failed(c, err)
			return
		}
		store.SaveList(a.Store, model.MODULES_KEY, modules)
		success(c, modules)
	}
}

func GetModule(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		modules := store.GetList[model.Module](a.Store, model.MODULES_KEY)
		module := store.FindByName(modules, name)
		if module == nil {
			failed(c, fmt.Errorf("module %s not found", name), http.StatusNotFound)
			return
		}
		c.String(200, module.Content)
	}
}

func UpdateModule(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var module model.Module
		if err := c.ShouldBindJSON(&module); err != nil {
			failed(c, err)
			return
		}
		modules := store.GetList[model.Module](a.Store, model.MODULES_KEY)
		if store.FindByName(modules, name) == nil {
			failed(c, fmt.Errorf("module %s not found", name), http.StatusNotFound)
			return
		}
		module.UpdatedAt = time.Now().Unix()
		store.UpdateByName(modules, name, module)
		store.SaveList(a.Store, model.MODULES_KEY, modules)
		success(c, module)
	}
}

func DeleteModule(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		modules := store.GetList[model.Module](a.Store, model.MODULES_KEY)
		if store.FindByName(modules, name) == nil {
			failed(c, fmt.Errorf("module %s not found", name), http.StatusNotFound)
			return
		}
		store.DeleteByName(&modules, name)
		store.SaveList(a.Store, model.MODULES_KEY, modules)
		success(c, nil)
	}
}

func GetEnv(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		subStoreEnv := make(map[string]string)
		for _, e := range os.Environ() {
			if strings.HasPrefix(e, "SUB_STORE_") {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) == 2 {
					subStoreEnv[parts[0]] = parts[1]
				}
			}
		}
		feature := map[string]interface{}{
			"archive": true,
		}
		if c.Query("share") != "" {
			feature["share"] = true
		}
		env := map[string]interface{}{
			"backend": "Node",
			"version": "2.36.0-go",
			"feature": feature,
			"meta": map[string]interface{}{
				"node": map[string]interface{}{
					"version": runtime.Version(),
					"env":     subStoreEnv,
				},
			},
		}
		success(c, env)
	}
}

func GistBackup(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		action := c.Query("action")
		a.Info("Gist backup action: " + action)
		c.JSON(200, gin.H{"status": "success"})
	}
}

func RefreshCache(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		a.Info("Refreshing cache")
		contentCache.Clear()
		c.JSON(200, gin.H{"status": "success"})
	}
}

func GetNodeInfo(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Server string `json:"server"`
			Port   int    `json:"port"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		result := map[string]interface{}{
			"server": req.Server,
			"info":   "Node info placeholder",
		}
		success(c, result)
	}
}

func GenerateAgeKeyPair(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := map[string]interface{}{
			"publicKey": "age1...",
			"secretKey": "AGE-SECRET-KEY-1...",
		}
		success(c, result)
	}
}

func DeriveAgePublicKey(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SecretKey string `json:"secretKey"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		result := map[string]interface{}{
			"publicKey": "age1...",
		}
		success(c, result)
	}
}

func ParseProxy(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		proxies, err := parser.ParseContent(req.Content)
		if err != nil {
			failed(c, err)
			return
		}
		maps := make([]map[string]interface{}, len(proxies))
		for i, p := range proxies {
			maps[i] = normalizer.NormalizeProxy(p).ToMap()
		}
		result := map[string]interface{}{
			"parsed": maps,
		}
		success(c, result)
	}
}

func ParseRule(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Rules  []string `json:"rules"`
			Target string   `json:"target,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		rawContent := strings.Join(req.Rules, "\n")
		rules := ruleutil.ParseRules(rawContent)
		var output []string
		target := strings.ToLower(req.Target)
		for _, r := range rules {
			switch target {
			case "qx", "quantumultx":
				output = append(output, ruleutil.ProduceQXRule(r))
			case "surge", "loon":
				output = append(output, ruleutil.ProduceSurgeRule(r))
			default:
				output = append(output, ruleutil.ProduceClashRule(r))
			}
		}
		result := map[string]interface{}{
			"parsed": output,
		}
		success(c, result)
	}
}

func GetLogs(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		keyword := c.Query("keyword")
		if keyword != "" {
			a.Info("Filtering logs with keyword: " + keyword)
		}
		logs := []map[string]interface{}{}
		if data := a.Store.Read(model.LOGS_KEY); data != nil {
			if logData, ok := data.([]interface{}); ok {
				for _, item := range logData {
					if logItem, ok := item.(map[string]interface{}); ok {
						logs = append(logs, logItem)
					}
				}
			}
		}
		success(c, logs)
	}
}

func ClearLogs(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		a.Store.Write(model.LOGS_KEY, []interface{}{})
		success(c, nil)
	}
}

func ExportStorage(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		data := map[string]interface{}{
			"settings":    a.Store.Read(model.SETTINGS_KEY),
			"subs":        a.Store.Read(model.SUBS_KEY),
			"collections": a.Store.Read(model.COLLECTIONS_KEY),
			"files":       a.Store.Read(model.FILES_KEY),
			"artifacts":   a.Store.Read(model.ARTIFACTS_KEY),
			"rules":       a.Store.Read(model.RULES_KEY),
			"tokens":      a.Store.Read(model.TOKENS_KEY),
			"modules":     a.Store.Read(model.MODULES_KEY),
		}
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", `attachment; filename="`+fmt.Sprintf("sub-store_data_%s.json", model.FormatDateTime(time.Now()))+`"`)
		c.JSON(200, data)
	}
}

func ImportStorage(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		if req.Content == "" {
			failed(c, fmt.Errorf("content field is required"))
			return
		}

		var data map[string]interface{}
		decoded, err := base64.StdEncoding.DecodeString(req.Content)
		if err == nil {
			json.Unmarshal(decoded, &data)
		}
		if data == nil {
			if err := json.Unmarshal([]byte(req.Content), &data); err != nil {
				failed(c, fmt.Errorf("failed to parse backup data: %w", err))
				return
			}
		}
		if data == nil {
			failed(c, fmt.Errorf("invalid backup data"))
			return
		}
		if _, hasSettings := data["settings"]; !hasSettings {
			failed(c, fmt.Errorf("backup data must contain settings field"))
			return
		}
		for _, key := range []string{"settings", "subs", "collections", "files", "artifacts", "rules", "tokens", "modules"} {
			if v, ok := data[key]; ok {
				a.Store.Write(key, v)
			}
		}
		a.Store.Migrate()
		success(c, nil)
	}
}

func RegisterRoutes(r *gin.Engine, a *app.App) {
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "message": "Hello from Sub-Store Go"})
	})

	api := r.Group("/api")
	{
		api.GET("/subs", GetAllSubscriptions(a))
		api.POST("/subs", CreateSubscription(a))
		api.PUT("/subs", ReplaceSubscriptions(a))
		api.GET("/sub/flow/:name", GetSubscriptionFlow(a))
		api.GET("/sub/:name", GetSubscription(a))
		api.PATCH("/sub/:name", UpdateSubscription(a))
		api.DELETE("/sub/:name", DeleteSubscription(a))
	}
	{
		api.GET("/collections", GetAllCollections(a))
		api.POST("/collections", CreateCollection(a))
		api.PUT("/collections", ReplaceCollections(a))
		api.GET("/collection/:name", GetCollection(a))
		api.PATCH("/collection/:name", UpdateCollection(a))
		api.DELETE("/collection/:name", DeleteCollection(a))
	}
	{
		api.GET("/files", GetAllFiles(a))
		api.POST("/files", CreateFile(a))
		api.PUT("/files", ReplaceFiles(a))
		api.GET("/file/:name", GetFile(a))
		api.GET("/wholeFile/:name", GetWholeFile(a))
		api.GET("/wholeFiles", GetAllWholeFiles(a))
		api.PATCH("/file/:name", UpdateFile(a))
		api.DELETE("/file/:name", DeleteFile(a))
	}
	{
		api.GET("/artifacts", GetAllArtifacts(a))
		api.POST("/artifacts", CreateArtifact(a))
		api.PUT("/artifacts", ReplaceArtifacts(a))
		api.GET("/artifacts/restore", RestoreArtifacts(a))
		api.GET("/artifact/:name", GetArtifact(a))
		api.PATCH("/artifact/:name", UpdateArtifact(a))
		api.DELETE("/artifact/:name", DeleteArtifact(a))
	}

	r.GET("/download/:name", DownloadSubscription(a))
	r.GET("/download/:name/:target", DownloadSubscription(a))
	r.GET("/download/collection/:name", DownloadCollection(a))
	r.GET("/download/collection/:name/:target", DownloadCollection(a))
	r.GET("/download/:name/api/v1/server/details", NezhaServerDetails(a, "subscription"))
	r.GET("/download/collection/:name/api/v1/server/details", NezhaServerDetails(a, "collection"))
	r.GET("/download/:name/api/v1/monitor/:nezhaIndex", NezhaMonitor(a, "subscription"))
	r.GET("/download/collection/:name/api/v1/monitor/:nezhaIndex", NezhaMonitor(a, "collection"))

	share := r.Group("/", middleware.ShareTokenAuth(a.Store))
	{
		share.GET("/share/sub/:name", DownloadSubscription(a))
		share.GET("/share/sub/:name/:target", DownloadSubscription(a))
		share.GET("/share/col/:name", DownloadCollection(a))
		share.GET("/share/col/:name/:target", DownloadCollection(a))
		share.GET("/share/file/:name", GetFile(a))
	}

	{
		api.GET("/sync/artifacts", SyncAllArtifacts(a))
		api.GET("/sync/artifact/:name", SyncArtifact(a))
	}
	{
		api.GET("/settings", GetSettings(a))
		api.PATCH("/settings", UpdateSettings(a))
	}
	{
		api.POST("/preview/sub", PreviewSubscription(a))
		api.POST("/preview/collection", PreviewCollection(a))
		api.POST("/preview/file", PreviewFile(a))
	}
	{
		api.POST("/sort/subs", SortSubs(a))
		api.POST("/sort/collections", SortCollections(a))
		api.POST("/sort/artifacts", SortArtifacts(a))
		api.POST("/sort/files", SortFiles(a))
		api.POST("/sort/tokens", SortTokens(a))
		api.POST("/sort/archives", SortArchives(a))
	}
	{
		api.GET("/tokens", GetAllTokens(a))
		api.POST("/token", CreateToken(a))
		api.DELETE("/token/:token", DeleteToken(a))
	}
	{
		api.GET("/archives", GetAllArchives(a))
		api.GET("/archives/:id", GetArchive(a))
		api.DELETE("/archives/:id", DeleteArchive(a))
		api.POST("/archives/:id/restore", RestoreArchive(a))
	}
	{
		api.GET("/modules", GetAllModules(a))
		api.POST("/modules", CreateModule(a))
		api.PUT("/modules", ReplaceModules(a))
		api.GET("/module/:name", GetModule(a))
		api.PATCH("/module/:name", UpdateModule(a))
		api.DELETE("/module/:name", DeleteModule(a))
	}
	{
		api.GET("/utils/env", GetEnv(a))
		api.GET("/utils/backup", GistBackup(a))
		api.GET("/utils/refresh", RefreshCache(a))
		api.POST("/utils/node-info", GetNodeInfo(a))
		api.POST("/utils/age/key-pair", GenerateAgeKeyPair(a))
		api.POST("/utils/age/public-key", DeriveAgePublicKey(a))
		api.POST("/proxy/parse", ParseProxy(a))
		api.POST("/rule/parse", ParseRule(a))
	}

	r.GET("/api/logs", GetLogs(a))
	r.DELETE("/api/logs", ClearLogs(a))

	{
		api.GET("/storage", ExportStorage(a))
		api.POST("/storage", ImportStorage(a))
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"status": "failed", "message": "ERROR: 404 not found"})
	})
}
