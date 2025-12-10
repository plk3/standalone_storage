package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"standalone_storage/db"
	"standalone_storage/handlers"
	"standalone_storage/models"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

//go:embed frontend/*
var frontendEmbed embed.FS

func main() {
	// Initialize key components
	db.Init()
	handlers.InitStorage()

	// Run Migrations
	if err := models.Migrate(db.DB); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	r := gin.Default()

	// CORS Setup
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API Routes
	api := r.Group("/api")
	{
		api.POST("/upload", handlers.UploadFile)
		api.GET("/files", handlers.SearchFiles)
		api.GET("/files/:id/download", handlers.DownloadFile)
		api.DELETE("/files/:id", handlers.DeleteFile)
		api.PUT("/files/:id", handlers.UpdateFile)
		api.GET("/tags", handlers.GetTags)
		api.GET("/backup", handlers.CreateBackup)
		api.POST("/restore", handlers.RestoreBackup)
	}

	// Serve Embedded Frontend
	fsys, err := fs.Sub(frontendEmbed, "frontend")
	if err != nil {
		log.Fatal("Failed to load frontend:", err)
	}

	r.StaticFileFS("/style.css", "style.css", http.FS(fsys))
	r.StaticFileFS("/app.js", "app.js", http.FS(fsys))

	// Catch-all for index.html (SPA support if needed, or just root)
	r.GET("/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		content, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to read index: "+err.Error())
			return
		}
		c.String(http.StatusOK, string(content))
	})

	log.Println("Server starting on :8081")
	if err := r.Run(":8081"); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
