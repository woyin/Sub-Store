package main

import (
	"github.com/gin-gonic/gin"
)

func registerRoutes(r *gin.Engine, app *App) {
	// Health check
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "message": "Hello from Sub-Store Go"})
	})

	// Subscriptions
	subRoutes := r.Group("/api")
	{
		subRoutes.GET("/subs", GetAllSubscriptions(app))
		subRoutes.POST("/subs", CreateSubscription(app))
		subRoutes.PUT("/subs", ReplaceSubscriptions(app))
		subRoutes.GET("/sub/flow/:name", GetSubscriptionFlow(app))
		subRoutes.GET("/sub/:name", GetSubscription(app))
		subRoutes.PATCH("/sub/:name", UpdateSubscription(app))
		subRoutes.DELETE("/sub/:name", DeleteSubscription(app))
	}

	// Collections
	colRoutes := r.Group("/api")
	{
		colRoutes.GET("/collections", GetAllCollections(app))
		colRoutes.POST("/collections", CreateCollection(app))
		colRoutes.PUT("/collections", ReplaceCollections(app))
		colRoutes.GET("/collection/:name", GetCollection(app))
		colRoutes.PATCH("/collection/:name", UpdateCollection(app))
		colRoutes.DELETE("/collection/:name", DeleteCollection(app))
	}

	// Files
	fileRoutes := r.Group("/api")
	{
		fileRoutes.GET("/files", GetAllFiles(app))
		fileRoutes.POST("/files", CreateFile(app))
		fileRoutes.PUT("/files", ReplaceFiles(app))
		fileRoutes.GET("/file/:name", GetFile(app))
		fileRoutes.GET("/wholeFile/:name", GetWholeFile(app))
		fileRoutes.GET("/wholeFiles", GetAllWholeFiles(app))
		fileRoutes.PATCH("/file/:name", UpdateFile(app))
		fileRoutes.DELETE("/file/:name", DeleteFile(app))
	}

	// Artifacts
	artifactRoutes := r.Group("/api")
	{
		artifactRoutes.GET("/artifacts", GetAllArtifacts(app))
		artifactRoutes.POST("/artifacts", CreateArtifact(app))
		artifactRoutes.PUT("/artifacts", ReplaceArtifacts(app))
		artifactRoutes.GET("/artifacts/restore", RestoreArtifacts(app))
		artifactRoutes.GET("/artifact/:name", GetArtifact(app))
		artifactRoutes.PATCH("/artifact/:name", UpdateArtifact(app))
		artifactRoutes.DELETE("/artifact/:name", DeleteArtifact(app))
	}

	// Download & Share
	r.GET("/download/:name", DownloadSubscription(app))
	r.GET("/download/:name/:target", DownloadSubscription(app))
	r.GET("/download/collection/:name", DownloadCollection(app))
	r.GET("/download/collection/:name/:target", DownloadCollection(app))
	r.GET("/share/sub/:name", DownloadSubscription(app))
	r.GET("/share/sub/:name/:target", DownloadSubscription(app))
	r.GET("/share/col/:name", DownloadCollection(app))
	r.GET("/share/col/:name/:target", DownloadCollection(app))

	// Sync
	syncRoutes := r.Group("/api")
	{
		syncRoutes.GET("/sync/artifacts", SyncAllArtifacts(app))
		syncRoutes.GET("/sync/artifact/:name", SyncArtifact(app))
	}

	// Settings
	settingsRoutes := r.Group("/api")
	{
		settingsRoutes.GET("/settings", GetSettings(app))
		settingsRoutes.PATCH("/settings", UpdateSettings(app))
	}

	// Preview
	previewRoutes := r.Group("/api")
	{
		previewRoutes.POST("/preview/sub", PreviewSubscription(app))
		previewRoutes.POST("/preview/collection", PreviewCollection(app))
		previewRoutes.POST("/preview/file", PreviewFile(app))
	}

	// Sorting
	sortRoutes := r.Group("/api")
	{
		sortRoutes.POST("/sort/subs", SortSubs(app))
		sortRoutes.POST("/sort/collections", SortCollections(app))
		sortRoutes.POST("/sort/artifacts", SortArtifacts(app))
		sortRoutes.POST("/sort/files", SortFiles(app))
		sortRoutes.POST("/sort/tokens", SortTokens(app))
		sortRoutes.POST("/sort/archives", SortArchives(app))
	}

	// Tokens
	tokenRoutes := r.Group("/api")
	{
		tokenRoutes.GET("/tokens", GetAllTokens(app))
		tokenRoutes.POST("/token", CreateToken(app))
		tokenRoutes.DELETE("/token/:token", DeleteToken(app))
	}

	// Archives
	archiveRoutes := r.Group("/api")
	{
		archiveRoutes.GET("/archives", GetAllArchives(app))
		archiveRoutes.GET("/archives/:id", GetArchive(app))
		archiveRoutes.DELETE("/archives/:id", DeleteArchive(app))
		archiveRoutes.POST("/archives/:id/restore", RestoreArchive(app))
	}

	// Modules
	moduleRoutes := r.Group("/api")
	{
		moduleRoutes.GET("/modules", GetAllModules(app))
		moduleRoutes.POST("/modules", CreateModule(app))
		moduleRoutes.PUT("/modules", ReplaceModules(app))
		moduleRoutes.GET("/module/:name", GetModule(app))
		moduleRoutes.PATCH("/module/:name", UpdateModule(app))
		moduleRoutes.DELETE("/module/:name", DeleteModule(app))
	}

	// Utils
	utilsRoutes := r.Group("/api")
	{
		utilsRoutes.GET("/utils/env", GetEnv(app))
		utilsRoutes.GET("/utils/backup", GistBackup(app))
		utilsRoutes.GET("/utils/refresh", RefreshCache(app))
		utilsRoutes.POST("/utils/node-info", GetNodeInfo(app))
		utilsRoutes.POST("/utils/age/key-pair", GenerateAgeKeyPair(app))
		utilsRoutes.POST("/utils/age/public-key", DeriveAgePublicKey(app))
		utilsRoutes.POST("/proxy/parse", ParseProxy(app))
		utilsRoutes.POST("/rule/parse", ParseRule(app))
	}

	// Logs
	r.GET("/api/logs", GetLogs(app))
	r.DELETE("/api/logs", ClearLogs(app))

	// Storage
	storageRoutes := r.Group("/api")
	{
		storageRoutes.GET("/storage", ExportStorage(app))
		storageRoutes.POST("/storage", ImportStorage(app))
	}

	// 404 handler
	r.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"status": "failed", "message": "ERROR: 404 not found"})
	})
}
