package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Settings handlers
func GetSettings(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		if data := app.Store.Read(SETTINGS_KEY); data != nil {
			if settings, ok := data.(map[string]interface{}); ok {
				success(c, settings)
				return
			}
		}
		// Return default settings
		defaultSettings := map[string]interface{}{
			"defaultTimeout":         8000,
			"githubApiTimeout":       10000,
			"artifactSyncBatchSize":  10,
			"cacheThreshold":         1024,
			"backendRequestConcurrency": 10,
		}
		success(c, defaultSettings)
	}
}

func UpdateSettings(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var settings map[string]interface{}
		if err := c.ShouldBindJSON(&settings); err != nil {
			failed(c, err)
			return
		}
		app.Store.Write(SETTINGS_KEY, settings)
		success(c, settings)
	}
}

// Preview handlers
func PreviewSubscription(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name    string     `json:"name"`
			Raw     string     `json:"raw,omitempty"`
			Process []Operator `json:"process,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		app.Info("Previewing subscription: " + req.Name)

		// Simplified preview
		result := map[string]interface{}{
			"original":   []map[string]interface{}{},
			"processed":  []map[string]interface{}{},
		}
		success(c, result)
	}
}

func PreviewCollection(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name    string     `json:"name"`
			Process []Operator `json:"process,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		app.Info("Previewing collection: " + req.Name)
		result := map[string]interface{}{
			"original":  []map[string]interface{}{},
			"processed": []map[string]interface{}{},
		}
		success(c, result)
	}
}

func PreviewFile(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name    string     `json:"name"`
			Process []Operator `json:"process,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		app.Info("Previewing file: " + req.Name)
		result := map[string]interface{}{
			"original":  "",
			"processed": "",
		}
		success(c, result)
	}
}

// Sorting handlers
func SortSubs(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var names []string
		if err := c.ShouldBindJSON(&names); err != nil {
			failed(c, err)
			return
		}
		// Reorder subscriptions based on names
		allSubs := getList[Subscription](app.Store, SUBS_KEY)
		var newSubs []Subscription
		for _, name := range names {
			if sub := findByName(allSubs, name); sub != nil {
				newSubs = append(newSubs, *sub)
			}
		}
		saveList(app.Store, SUBS_KEY, newSubs)
		success(c, nil)
	}
}

func SortCollections(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var names []string
		if err := c.ShouldBindJSON(&names); err != nil {
			failed(c, err)
			return
		}
		allCols := getList[Collection](app.Store, COLLECTIONS_KEY)
		var newCols []Collection
		for _, name := range names {
			if col := findByName(allCols, name); col != nil {
				newCols = append(newCols, *col)
			}
		}
		saveList(app.Store, COLLECTIONS_KEY, newCols)
		success(c, nil)
	}
}

func SortArtifacts(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var names []string
		if err := c.ShouldBindJSON(&names); err != nil {
			failed(c, err)
			return
		}
		allArtifacts := getList[Artifact](app.Store, ARTIFACTS_KEY)
		var newArtifacts []Artifact
		for _, name := range names {
			if artifact := findByName(allArtifacts, name); artifact != nil {
				newArtifacts = append(newArtifacts, *artifact)
			}
		}
		saveList(app.Store, ARTIFACTS_KEY, newArtifacts)
		success(c, nil)
	}
}

func SortFiles(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var names []string
		if err := c.ShouldBindJSON(&names); err != nil {
			failed(c, err)
			return
		}
		allFiles := getList[File](app.Store, FILES_KEY)
		var newFiles []File
		for _, name := range names {
			if file := findByName(allFiles, name); file != nil {
				newFiles = append(newFiles, *file)
			}
		}
		saveList(app.Store, FILES_KEY, newFiles)
		success(c, nil)
	}
}

func SortTokens(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Sort tokens by type-name-token
		var names []string
		if err := c.ShouldBindJSON(&names); err != nil {
			failed(c, err)
			return
		}
		success(c, nil)
	}
}

func SortArchives(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var names []string
		if err := c.ShouldBindJSON(&names); err != nil {
			failed(c, err)
			return
		}
		success(c, nil)
	}
}

// Token handlers
func GetAllTokens(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokens := getList[Token](app.Store, TOKENS_KEY)
		// Apply filters if provided
		tokenType := c.Query("type")
		name := c.Query("name")

		var filtered []Token
		for _, token := range tokens {
			if (tokenType == "" || token.Type == tokenType) && (name == "" || token.Name == name) {
				filtered = append(filtered, token)
			}
		}
		success(c, filtered)
	}
}

func CreateToken(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var token Token
		if err := c.ShouldBindJSON(&token); err != nil {
			failed(c, err)
			return
		}
		if token.Token == "" {
			token.Token = randomString(16)
		}

		tokens := getList[Token](app.Store, TOKENS_KEY)
		insertByPosition(&tokens, token, "bottom")
		saveList(app.Store, TOKENS_KEY, tokens)
		success(c, token, http.StatusCreated)
	}
}

func DeleteToken(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		tokens := getList[Token](app.Store, TOKENS_KEY)
		for i, t := range tokens {
			if t.Token == token {
				tokens = append(tokens[:i], tokens[i+1:]...)
				break
			}
		}
		saveList(app.Store, TOKENS_KEY, tokens)
		success(c, nil)
	}
}

// Archive handlers
func GetAllArchives(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		archives := getList[Archive](app.Store, ARCHIVES_KEY)
		success(c, archives)
	}
}

func GetArchive(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		archives := getList[Archive](app.Store, ARCHIVES_KEY)
		for _, archive := range archives {
			if archive.ID == id {
				success(c, archive)
				return
			}
		}
		failed(c, fmt.Errorf("archive %s not found"), http.StatusNotFound)
	}
}

func DeleteArchive(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		archives := getList[Archive](app.Store, ARCHIVES_KEY)
		for i, archive := range archives {
			if archive.ID == id {
				archives = append(archives[:i], archives[i+1:]...)
				break
			}
		}
		saveList(app.Store, ARCHIVES_KEY, archives)
		success(c, nil)
	}
}

func RestoreArchive(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		archives := getList[Archive](app.Store, ARCHIVES_KEY)
		for _, archive := range archives {
			if archive.ID == id {
				// Restore based on type
				switch archive.Type {
				case "sub":
					var sub Subscription
					if data, err := json.Marshal(archive.Data); err == nil {
						json.Unmarshal(data, &sub)
						// Insert into subs
						// (simplified)
					}
				case "col":
					var col Collection
					if data, err := json.Marshal(archive.Data); err == nil {
						json.Unmarshal(data, &col)
					}
				case "file":
					var file File
					if data, err := json.Marshal(archive.Data); err == nil {
						json.Unmarshal(data, &file)
					}
				case "artifact":
					var artifact Artifact
					if data, err := json.Marshal(archive.Data); err == nil {
						json.Unmarshal(data, &artifact)
					}
				}
				success(c, nil)
				return
			}
		}
		failed(c, fmt.Errorf("archive %s not found"), http.StatusNotFound)
	}
}

// Module handlers
func GetAllModules(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		modules := getList[Module](app.Store, MODULES_KEY)
		// Mask content
		for i := range modules {
			modules[i].Content = ""
		}
		success(c, modules)
	}
}

func CreateModule(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var module Module
		if err := c.ShouldBindJSON(&module); err != nil {
			failed(c, err)
			return
		}
		modules := getList[Module](app.Store, MODULES_KEY)
		if findByName(modules, module.Name) != nil {
			failed(c, fmt.Errorf("module %s already exists"), http.StatusConflict)
			return
		}
		module.UpdatedAt = time.Now().Unix()
		insertByPosition(&modules, module, "bottom")
		saveList(app.Store, MODULES_KEY, modules)
		success(c, module, http.StatusCreated)
	}
}

func ReplaceModules(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var modules []Module
		if err := c.ShouldBindJSON(&modules); err != nil {
			failed(c, err)
			return
		}
		saveList(app.Store, MODULES_KEY, modules)
		success(c, modules)
	}
}

func GetModule(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		modules := getList[Module](app.Store, MODULES_KEY)
		module := findByName(modules, name)
		if module == nil {
			failed(c, fmt.Errorf("module %s not found"), http.StatusNotFound)
			return
		}
		c.String(200, module.Content)
	}
}

func UpdateModule(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var module Module
		if err := c.ShouldBindJSON(&module); err != nil {
			failed(c, err)
			return
		}
		modules := getList[Module](app.Store, MODULES_KEY)
		if findByName(modules, name) == nil {
			failed(c, fmt.Errorf("module %s not found"), http.StatusNotFound)
			return
		}
		module.UpdatedAt = time.Now().Unix()
		updateByName(modules, name, module)
		saveList(app.Store, MODULES_KEY, modules)
		success(c, module)
	}
}

func DeleteModule(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		modules := getList[Module](app.Store, MODULES_KEY)
		if findByName(modules, name) == nil {
			failed(c, fmt.Errorf("module %s not found"), http.StatusNotFound)
			return
		}
		deleteByName(&modules, name)
		saveList(app.Store, MODULES_KEY, modules)
		success(c, nil)
	}
}

// Utils handlers
func GetEnv(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		env := map[string]interface{}{
			"isNode":  true,
			"version": "2.36.0-go",
		}
		success(c, env)
	}
}

func GistBackup(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		action := c.Query("action")
		app.Info("Gist backup action: " + action)
		c.JSON(200, gin.H{"status": "success"})
	}
}

func RefreshCache(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		app.Info("Refreshing cache")
		c.JSON(200, gin.H{"status": "success"})
	}
}

func GetNodeInfo(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Server string `json:"server"`
			Port   int    `json:"port"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		// Fetch node info from ip-api.com (simplified)
		result := map[string]interface{}{
			"server": req.Server,
			"info":   "Node info placeholder",
		}
		success(c, result)
	}
}

func GenerateAgeKeyPair(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate age key pair (placeholder)
		result := map[string]interface{}{
			"publicKey":  "age1...",
			"secretKey":  "AGE-SECRET-KEY-1...",
		}
		success(c, result)
	}
}

func DeriveAgePublicKey(app *App) gin.HandlerFunc {
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

func ParseProxy(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		// Parse proxy content (placeholder)
		result := map[string]interface{}{
			"parsed": []map[string]interface{}{},
		}
		success(c, result)
	}
}

func ParseRule(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Rules []string `json:"rules"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		result := map[string]interface{}{
			"parsed": req.Rules,
		}
		success(c, result)
	}
}

// Logs handlers
func GetLogs(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		keyword := c.Query("keyword")
		if keyword != "" {
			app.Info("Filtering logs with keyword: " + keyword)
		}
		logs := []map[string]interface{}{}
		if data := app.Store.Read(LOGS_KEY); data != nil {
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

func ClearLogs(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		app.Store.Write(LOGS_KEY, []interface{}{})
		success(c, nil)
	}
}

// Storage handlers
func ExportStorage(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		data := map[string]interface{}{
			"settings":    app.Store.Read(SETTINGS_KEY),
			"subs":        app.Store.Read(SUBS_KEY),
			"collections": app.Store.Read(COLLECTIONS_KEY),
			"files":       app.Store.Read(FILES_KEY),
			"artifacts":   app.Store.Read(ARTIFACTS_KEY),
			"rules":       app.Store.Read(RULES_KEY),
			"tokens":      app.Store.Read(TOKENS_KEY),
			"modules":     app.Store.Read(MODULES_KEY),
		}
		jsonData, _ := json.Marshal(data)
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", `attachment; filename="sub-store_backup_`+formatDateTime(time.Now())+`.json"`)
		c.String(200, base64.StdEncoding.EncodeToString(jsonData))
	}
}

func ImportStorage(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Data string `json:"data"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			failed(c, err)
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(req.Data)
		if err != nil {
			failed(c, err)
			return
		}
		var data map[string]interface{}
		if err := json.Unmarshal(decoded, &data); err != nil {
			failed(c, err)
			return
		}
		if settings, ok := data["settings"]; ok {
			app.Store.Write(SETTINGS_KEY, settings)
		}
		if subs, ok := data["subs"]; ok {
			app.Store.Write(SUBS_KEY, subs)
		}
		if collections, ok := data["collections"]; ok {
			app.Store.Write(COLLECTIONS_KEY, collections)
		}
		if files, ok := data["files"]; ok {
			app.Store.Write(FILES_KEY, files)
		}
		if artifacts, ok := data["artifacts"]; ok {
			app.Store.Write(ARTIFACTS_KEY, artifacts)
		}
		if rules, ok := data["rules"]; ok {
			app.Store.Write(RULES_KEY, rules)
		}
		if tokens, ok := data["tokens"]; ok {
			app.Store.Write(TOKENS_KEY, tokens)
		}
		if modules, ok := data["modules"]; ok {
			app.Store.Write(MODULES_KEY, modules)
		}
		app.Store.Migrate()
		success(c, nil)
	}
}
