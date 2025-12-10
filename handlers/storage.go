package handlers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"standalone_storage/db"
	"standalone_storage/models"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var StorageDir = "data/uploads"

func InitStorage() {
	if err := os.MkdirAll(StorageDir, 0755); err != nil {
		log.Fatal("Failed to create storage directory:", err)
	}
}

func UploadFile(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}

	// Generate ID and Paths
	id := uuid.New().String()
	ext := filepath.Ext(fileHeader.Filename)
	safeFilename := fmt.Sprintf("%s%s", id, ext)
	dstPath := filepath.Join(StorageDir, safeFilename)

	// Save File
	if err := c.SaveUploadedFile(fileHeader, dstPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Process Tags
	tagsStr := c.PostForm("tags")
	// Normalize tags to comma separated string (frontend sends comma separated)
	tagsList := strings.Split(tagsStr, ",")
	var cleanTags []string
	for _, t := range tagsList {
		t = strings.TrimSpace(t)
		if t != "" {
			cleanTags = append(cleanTags, t)
		}
	}

	// DB Record
	fileRecord := models.File{
		ID:          id,
		Filename:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get("Content-Type"),
		Size:        fileHeader.Size,
		Tags:        cleanTags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if result := db.DB.Create(&fileRecord); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save metadata"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File uploaded successfully", "file": fileRecord})
}

func SearchFiles(c *gin.Context) {
	query := c.Query("q")
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")

	page := 1
	limit := 50
	fmt.Sscanf(pageStr, "%d", &page)
	fmt.Sscanf(limitStr, "%d", &limit)
	offset := (page - 1) * limit

	var files []models.File
	tx := db.DB.Model(&models.File{})

	if query != "" {
		// SQLite JSON search: LIKE '%"tag"%'
		// This is a simple approximation for exact match in JSON array ["a","b"]
		tx = tx.Where("tags LIKE ?", "%\""+query+"\"%")
	}

	tx.Order("created_at desc").Limit(limit).Offset(offset).Find(&files)

	// Transform for frontend (add URL)
	var response []gin.H
	for _, f := range files {
		url := fmt.Sprintf("/api/files/%s/download?preview=true", f.ID)
		response = append(response, gin.H{
			"id":           f.ID,
			"filename":     f.Filename,
			"content_type": f.ContentType,
			"size":         f.Size,
			"tags":         f.Tags,
			"created_at":   f.CreatedAt,
			"url":          url,
		})
	}

	if response == nil {
		response = []gin.H{}
	}
	c.JSON(http.StatusOK, response)
}

func DownloadFile(c *gin.Context) {
	id := c.Param("id")
	var fileRecord models.File
	if result := db.DB.First(&fileRecord, "id = ?", id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Reconstruct filename on disk (ID + original extension)
	// Actually we lost the extension in the DB save above? No, we saved filename.
	// But we verified we saved on disk as ID + Ext.
	// Wait, we failed to save the extension separately or the stored filename on disk in DB.
	// We need to match what we saved in UploadFile.
	// In UploadFile: safeFilename := fmt.Sprintf("%s%s", id, ext)
	ext := filepath.Ext(fileRecord.Filename)
	filePath := filepath.Join(StorageDir, id+ext)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File content not found"})
		return
	}

	c.File(filePath)
}

func DeleteFile(c *gin.Context) {
	id := c.Param("id")
	var fileRecord models.File
	if result := db.DB.First(&fileRecord, "id = ?", id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Delete from disk
	ext := filepath.Ext(fileRecord.Filename)
	filePath := filepath.Join(StorageDir, id+ext)
	os.Remove(filePath)

	// Delete from DB
	db.DB.Delete(&fileRecord)

	c.JSON(http.StatusOK, gin.H{"message": "File deleted"})
}

func UpdateFile(c *gin.Context) {
	id := c.Param("id")
	var fileRecord models.File
	if result := db.DB.First(&fileRecord, "id = ?", id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	var input struct {
		Tags []string `json:"tags"`
	}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	fileRecord.Tags = input.Tags
	db.DB.Save(&fileRecord)

	c.JSON(http.StatusOK, fileRecord)
}

func GetTags(c *gin.Context) {
	var files []models.File
	db.DB.Select("tags").Find(&files)

	tagMap := make(map[string]bool)
	for _, f := range files {
		for _, t := range f.Tags {
			t = strings.TrimSpace(t)
			if t != "" {
				tagMap[t] = true
			}
		}
	}

	var tags []string
	for t := range tagMap {
		tags = append(tags, t)
	}

	c.JSON(http.StatusOK, tags)
}

// CreateBackup creates a ZIP of data/uploads + metadata.json
func CreateBackup(c *gin.Context) {
	// Fetch all files
	var files []models.File
	if err := db.DB.Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch metadata"})
		return
	}

	filename := fmt.Sprintf("backup-%s.zip", time.Now().Format("20060102-150405"))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()

	// Metadata
	metaFile, err := zipWriter.Create("metadata.json")
	if err != nil {
		log.Printf("Failed to create metadata entry: %v", err)
		return
	}
	if err := json.NewEncoder(metaFile).Encode(files); err != nil {
		log.Printf("Failed to encode metadata: %v", err)
		return
	}

	// Files
	for _, f := range files {
		ext := filepath.Ext(f.Filename)
		srcPath := filepath.Join(StorageDir, f.ID+ext)

		// Entry name in ZIP: files/<original_name> or files/<id>_<name>?
		// Old app used files/<filename>. Let's stick to that for compatibility
		// BUT wait, old app used ObjectName which was time-filename.
		// Detailed check: Old app RestoreBackup used `files/` prefix and matched by Filename from zip to metadata.
		// If we create backup, we should probably follow a standard.
		// Let's use `files/<id><ext>` for our backups to avoid collision.

		zipEntryName := fmt.Sprintf("files/%s%s", f.ID, ext)
		fWriter, err := zipWriter.Create(zipEntryName)
		if err != nil {
			log.Printf("Failed to create zip entry %s: %v", f.ID, err)
			continue
		}

		srcFile, err := os.Open(srcPath)
		if err != nil {
			log.Printf("Failed to open file %s: %v", srcPath, err)
			continue
		}

		if _, err := io.Copy(fWriter, srcFile); err != nil {
			log.Printf("Failed to copy content: %v", err)
		}
		srcFile.Close()
	}
}

// RestoreBackup handles uploading a ZIP file
func RestoreBackup(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file"})
		return
	}
	defer f.Close()

	zipReader, err := zip.NewReader(f, fileHeader.Size)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid zip file"})
		return
	}

	// 1. Decode Metadata
	// We need a temporary struct to handle old format (Tags []string) AND new format (Tags []string)
	// Actually both go []string. GORM handles serialization.
	// But the JSON in the file has "tags": ["a", "b"].
	// Our model `Tags []string ` with `json:"tags"` will decode that correctly!
	// So we can use models.File directly for decoding json.
	// Wait, does GORM serializer affect simple `json.Unmarshal`?
	// `json:"tags"` tag is key. If the field is `[]string`, standard json decoder works fine with `["a","b"]`.
	// So we should be good.

	var metadata []models.File
	for _, zf := range zipReader.File {
		if zf.Name == "metadata.json" {
			rc, err := zf.Open()
			if err != nil {
				continue
			}
			// Use a temporary struct if needed, but []string should match
			if err := json.NewDecoder(rc).Decode(&metadata); err != nil {
				log.Printf("Failed to decode metadata: %v", err)
			}
			rc.Close()
			break
		}
	}

	if len(metadata) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metadata.json not found or empty"})
		return
	}

	// Create lookup map
	metadataMap := make(map[string]models.File)
	// Old app: ObjectName was unique key in MinIO. Filename was display name.
	// In backup zip: "files/<filename>".
	// There is ambiguity if multiple files have same filename in old app?
	// Old backup code: zipWriter.Create(fmt.Sprintf("files/%s", f.Filename)).
	// YES, if multiple files had same filename, zip entry conflicts! Old app bug.
	// Assuming distinct filenames for now or last one wins.

	// We map Filename -> Metadata for finding which file metadata belongs to the zip entry.
	for _, m := range metadata {
		metadataMap[m.Filename] = m
	}

	restoredCount := 0

	for _, zf := range zipReader.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		if !strings.HasPrefix(zf.Name, "files/") {
			continue
		}

		cleanName := strings.TrimPrefix(zf.Name, "files/")
		// cleanName is "filename.ext" from old backup.

		// Find metadata
		// Try exact match on Filename
		meta, ok := metadataMap[cleanName]
		if !ok {
			// Try matching ObjectName if available? Old backup didn't put ObjectName in zip path.
			// It put "files/" + f.Filename.
			continue
		}

		// Prepare destination
		// We want to store as ID.ext.
		// If meta.ID is uuid, use it. If not compatible or empty, make new.
		// Old app used UUID (Postgres).

		if meta.ID == "" {
			meta.ID = uuid.New().String()
		}

		ext := filepath.Ext(meta.Filename)
		dstFilename := meta.ID + ext
		dstPath := filepath.Join(StorageDir, dstFilename)

		// Extract file
		rc, err := zf.Open()
		if err != nil {
			log.Printf("Failed to open zip entry: %v", err)
			continue
		}

		dstFile, err := os.Create(dstPath)
		if err != nil {
			rc.Close()
			log.Printf("Failed to create dst file: %v", err)
			continue
		}

		if _, err := io.Copy(dstFile, rc); err != nil {
			log.Printf("Failed to write file: %v", err)
		}
		dstFile.Close()
		rc.Close()

		// Upsert Metadata
		// Check ID existence
		var existing models.File
		if err := db.DB.First(&existing, "id = ?", meta.ID).Error; err != nil {
			// Create
			// Reset times if needed, or keep original
			if err := db.DB.Create(&meta).Error; err != nil {
				log.Printf("Failed to insert metadata for %s: %v", meta.Filename, err)
			}
		} else {
			// Update
			existing.Filename = meta.Filename
			existing.Tags = meta.Tags
			existing.Size = meta.Size
			existing.ContentType = meta.ContentType
			existing.CreatedAt = meta.CreatedAt
			if err := db.DB.Save(&existing).Error; err != nil {
				log.Printf("Failed to update metadata for %s: %v", meta.Filename, err)
			}
		}
		restoredCount++
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Restored %d files", restoredCount)})
}
