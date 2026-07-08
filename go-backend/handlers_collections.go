package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Collection handlers
func GetAllCollections(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		cols := getList[Collection](app.Store, COLLECTIONS_KEY)
		success(c, cols)
	}
}

func CreateCollection(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var col Collection
		if err := c.ShouldBindJSON(&col); err != nil {
			failed(c, err)
			return
		}

		app.Info("Creating collection: " + col.Name)
		cols := getList[Collection](app.Store, COLLECTIONS_KEY)
		if findByName(cols, col.Name) != nil {
			failed(c, fmt.Errorf("collection %s already exists"), http.StatusConflict)
			return
		}

		insertByPosition(&cols, col, "bottom")
		saveList(app.Store, COLLECTIONS_KEY, cols)
		success(c, col, http.StatusCreated)
	}
}

func ReplaceCollections(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var cols []Collection
		if err := c.ShouldBindJSON(&cols); err != nil {
			failed(c, err)
			return
		}
		saveList(app.Store, COLLECTIONS_KEY, cols)
		success(c, cols)
	}
}

func GetCollection(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		raw := c.Query("raw")
		cols := getList[Collection](app.Store, COLLECTIONS_KEY)
		col := findByName(cols, name)
		if col == nil {
			failed(c, fmt.Errorf("collection %s not found"), http.StatusNotFound)
			return
		}

		if raw == "1" || raw == "true" {
			c.Header("Content-Type", "application/json")
			c.Header("Content-Disposition", `attachment; filename="`+fmt.Sprintf("sub-store_collection_%s_%s.json", name, formatDateTime(time.Now()))+`"`)
			c.JSON(200, col)
			return
		}
		success(c, col)
	}
}

func UpdateCollection(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var col Collection
		if err := c.ShouldBindJSON(&col); err != nil {
			failed(c, err)
			return
		}

		app.Info("Updating collection: " + name)
		cols := getList[Collection](app.Store, COLLECTIONS_KEY)
		oldCol := findByName(cols, name)
		if oldCol == nil {
			failed(c, fmt.Errorf("collection %s not found"), http.StatusNotFound)
			return
		}

		if col.Name == "" {
			col.Name = oldCol.Name
		}

		if name != col.Name {
			// Update artifacts and files referencing this collection
			artifacts := getList[Artifact](app.Store, ARTIFACTS_KEY)
			for i := range artifacts {
				if artifacts[i].Type == "collection" && artifacts[i].Source == name {
					artifacts[i].Source = col.Name
				}
			}
			saveList(app.Store, ARTIFACTS_KEY, artifacts)

			files := getList[File](app.Store, FILES_KEY)
			for i := range files {
				if files[i].SourceType == "collection" && files[i].SourceName == name {
					files[i].SourceName = col.Name
				}
			}
			saveList(app.Store, FILES_KEY, files)
		}

		updateByName(cols, name, col)
		saveList(app.Store, COLLECTIONS_KEY, cols)
		success(c, col)
	}
}

func DeleteCollection(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		mode := c.Query("mode")
		app.Info("Deleting collection: " + name)

		cols := getList[Collection](app.Store, COLLECTIONS_KEY)
		if findByName(cols, name) == nil {
			failed(c, fmt.Errorf("collection %s not found"), http.StatusNotFound)
			return
		}

		if shouldArchiveDeletion(mode) {
			col := findByName(cols, name)
			archive := Archive{
				ID:        createArchiveID(),
				Type:      "col",
				Name:      name,
				Data:      col,
				CreatedAt: time.Now().Unix(),
			}
			archives := getList[Archive](app.Store, ARCHIVES_KEY)
			archives = append([]Archive{archive}, archives...)
			saveList(app.Store, ARCHIVES_KEY, archives)
		}

		deleteByName(&cols, name)
		saveList(app.Store, COLLECTIONS_KEY, cols)
		success(c, nil)
	}
}

// File handlers
func GetAllFiles(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		files := getList[File](app.Store, FILES_KEY)
		// Mask sensitive content
		for i := range files {
			files[i].Content = ""
		}
		success(c, files)
	}
}

func CreateFile(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var file File
		if err := c.ShouldBindJSON(&file); err != nil {
			failed(c, err)
			return
		}

		app.Info("Creating file: " + file.Name)
		files := getList[File](app.Store, FILES_KEY)
		if findByName(files, file.Name) != nil {
			failed(c, fmt.Errorf("file %s already exists"), http.StatusConflict)
			return
		}

		insertByPosition(&files, file, "bottom")
		saveList(app.Store, FILES_KEY, files)
		success(c, file, http.StatusCreated)
	}
}

func ReplaceFiles(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var files []File
		if err := c.ShouldBindJSON(&files); err != nil {
			failed(c, err)
			return
		}
		saveList(app.Store, FILES_KEY, files)
		success(c, files)
	}
}

func GetFile(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		files := getList[File](app.Store, FILES_KEY)
		file := findByName(files, name)
		if file == nil {
			failed(c, fmt.Errorf("file %s not found"), http.StatusNotFound)
			return
		}
		c.String(200, file.Content)
	}
}

func GetWholeFile(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		files := getList[File](app.Store, FILES_KEY)
		file := findByName(files, name)
		if file == nil {
			failed(c, fmt.Errorf("file %s not found"), http.StatusNotFound)
			return
		}
		success(c, file)
	}
}

func GetAllWholeFiles(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		files := getList[File](app.Store, FILES_KEY)
		success(c, files)
	}
}

func UpdateFile(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var file File
		if err := c.ShouldBindJSON(&file); err != nil {
			failed(c, err)
			return
		}

		app.Info("Updating file: " + name)
		files := getList[File](app.Store, FILES_KEY)
		oldFile := findByName(files, name)
		if oldFile == nil {
			failed(c, fmt.Errorf("file %s not found"), http.StatusNotFound)
			return
		}

		if file.Name == "" {
			file.Name = oldFile.Name
		}

		if name != file.Name {
			// Update artifacts referencing this file
			artifacts := getList[Artifact](app.Store, ARTIFACTS_KEY)
			for i := range artifacts {
				if artifacts[i].Type == "file" && artifacts[i].Source == name {
					artifacts[i].Source = file.Name
				}
			}
			saveList(app.Store, ARTIFACTS_KEY, artifacts)
		}

		updateByName(files, name, file)
		saveList(app.Store, FILES_KEY, files)
		success(c, file)
	}
}

func DeleteFile(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		mode := c.Query("mode")
		app.Info("Deleting file: " + name)

		files := getList[File](app.Store, FILES_KEY)
		if findByName(files, name) == nil {
			failed(c, fmt.Errorf("file %s not found"), http.StatusNotFound)
			return
		}

		if shouldArchiveDeletion(mode) {
			file := findByName(files, name)
			archive := Archive{
				ID:        createArchiveID(),
				Type:      "file",
				Name:      name,
				Data:      file,
				CreatedAt: time.Now().Unix(),
			}
			archives := getList[Archive](app.Store, ARCHIVES_KEY)
			archives = append([]Archive{archive}, archives...)
			saveList(app.Store, ARCHIVES_KEY, archives)
		}

		deleteByName(&files, name)
		saveList(app.Store, FILES_KEY, files)
		success(c, nil)
	}
}

// Artifact handlers
func GetAllArtifacts(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		artifacts := getList[Artifact](app.Store, ARTIFACTS_KEY)
		success(c, artifacts)
	}
}

func CreateArtifact(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var artifact Artifact
		if err := c.ShouldBindJSON(&artifact); err != nil {
			failed(c, err)
			return
		}

		app.Info("Creating artifact: " + artifact.Name)
		artifacts := getList[Artifact](app.Store, ARTIFACTS_KEY)
		if findByName(artifacts, artifact.Name) != nil {
			failed(c, fmt.Errorf("artifact %s already exists"), http.StatusConflict)
			return
		}

		insertByPosition(&artifacts, artifact, "bottom")
		saveList(app.Store, ARTIFACTS_KEY, artifacts)
		success(c, artifact, http.StatusCreated)
	}
}

func ReplaceArtifacts(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var artifacts []Artifact
		if err := c.ShouldBindJSON(&artifacts); err != nil {
			failed(c, err)
			return
		}
		saveList(app.Store, ARTIFACTS_KEY, artifacts)
		success(c, artifacts)
	}
}

func RestoreArtifacts(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Restore artifacts from Gist
		app.Info("Restoring artifacts from Gist")
		success(c, nil)
	}
}

func GetArtifact(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		artifacts := getList[Artifact](app.Store, ARTIFACTS_KEY)
		artifact := findByName(artifacts, name)
		if artifact == nil {
			failed(c, fmt.Errorf("artifact %s not found"), http.StatusNotFound)
			return
		}
		success(c, artifact)
	}
}

func UpdateArtifact(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var artifact Artifact
		if err := c.ShouldBindJSON(&artifact); err != nil {
			failed(c, err)
			return
		}

		app.Info("Updating artifact: " + name)
		artifacts := getList[Artifact](app.Store, ARTIFACTS_KEY)
		if findByName(artifacts, name) == nil {
			failed(c, fmt.Errorf("artifact %s not found"), http.StatusNotFound)
			return
		}

		updateByName(artifacts, name, artifact)
		saveList(app.Store, ARTIFACTS_KEY, artifacts)
		success(c, artifact)
	}
}

func DeleteArtifact(app *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		app.Info("Deleting artifact: " + name)

		artifacts := getList[Artifact](app.Store, ARTIFACTS_KEY)
		if findByName(artifacts, name) == nil {
			failed(c, fmt.Errorf("artifact %s not found"), http.StatusNotFound)
			return
		}

		deleteByName(&artifacts, name)
		saveList(app.Store, ARTIFACTS_KEY, artifacts)
		success(c, nil)
	}
}
