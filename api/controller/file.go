package controller

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func UploadFile(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only image files allowed"})
		return
	}

	ct := c.ContentType()
	if !strings.HasPrefix(ct, "multipart/form-data") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid content type"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to open file"})
		return
	}
	defer f.Close()

	header := make([]byte, 12)
	n, _ := io.ReadFull(f, header)
	if n < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too short"})
		return
	}
	if !isAllowedImage(header, ext) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image content"})
		return
	}

	filename := fmt.Sprintf("%d_%s%s", time.Now().Unix(), uuid.New().String(), ext)
	savePath := filepath.Join("uploads", filename)

	_ = os.MkdirAll("uploads", 0755)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	url := "/uploads/" + filename
	c.JSON(http.StatusOK, gin.H{
		"url": url,
	})
}

func isAllowedImage(header []byte, ext string) bool {
	if strings.HasSuffix(ext, "png") {
		sig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		return len(header) >= 8 && equalPrefix(header, sig)
	}
	if ext == ".jpg" || ext == ".jpeg" {
		return len(header) >= 2 && header[0] == 0xFF && header[1] == 0xD8
	}
	if ext == ".gif" {
		return len(header) >= 6 && (string(header[:6]) == "GIF87a" || string(header[:6]) == "GIF89a")
	}
	if ext == ".webp" {
		return len(header) >= 12 && string(header[:4]) == "RIFF" && string(header[8:12]) == "WEBP"
	}
	return false
}

func equalPrefix(a, b []byte) bool {
	if len(a) < len(b) {
		return false
	}
	for i := range b {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
