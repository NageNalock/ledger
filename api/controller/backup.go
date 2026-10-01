package controller

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"io/fs"
	"ledger/api/db"
	"ledger/api/model"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var backupMu sync.Mutex

// ExportBackup snapshots SQLite through its engine (safe with WAL), not by
// copying a live database file. Secrets in the database are intentionally kept.
func ExportBackup(c *gin.Context) {
	var user model.User
	if err := db.DB.First(&user, c.GetUint("userID")).Error; err != nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	if user.Role != "admin" {
		c.JSON(403, gin.H{"error": "Admin only"})
		return
	}
	if !backupMu.TryLock() {
		c.JSON(409, gin.H{"error": "An export is already in progress"})
		return
	}
	defer backupMu.Unlock()
	dir, err := os.MkdirTemp("", "ledger-backup-")
	if err != nil {
		c.JSON(500, gin.H{"error": "Cannot create backup"})
		return
	}
	defer os.RemoveAll(dir)
	snapshot := filepath.Join(dir, "ledger.db")
	if err = db.DB.WithContext(c.Request.Context()).Exec("VACUUM INTO ?", snapshot).Error; err != nil {
		c.JSON(500, gin.H{"error": "Database snapshot failed"})
		return
	}
	archive := filepath.Join(dir, "backup.zip")
	if err = writeBackup(archive, snapshot, "uploads"); err != nil {
		c.JSON(500, gin.H{"error": "Backup packaging failed"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.FileAttachment(archive, "ledger-backup-"+time.Now().UTC().Format("20060102T150405Z")+".zip")
}

func writeBackup(dest, snapshot, uploads string) (err error) {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if e := f.Close(); err == nil {
			err = e
		}
	}()
	z := zip.NewWriter(f)
	defer func() {
		if e := z.Close(); err == nil {
			err = e
		}
	}()
	hashes := map[string]string{}
	add := func(path, name string) error {
		info, e := os.Lstat(path)
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular backup file")
		}
		src, e := os.Open(path)
		if e != nil {
			return e
		}
		defer src.Close()
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0600)
		dst, e := z.CreateHeader(h)
		if e != nil {
			return e
		}
		sum := sha256.New()
		if _, e = io.Copy(io.MultiWriter(dst, sum), src); e != nil {
			return e
		}
		hashes[name] = hex.EncodeToString(sum.Sum(nil))
		return nil
	}
	if err = add(snapshot, "ledger.db"); err != nil {
		return err
	}
	if info, e := os.Lstat(uploads); e == nil {
		if !info.IsDir() {
			return fmt.Errorf("uploads is not a directory")
		}
		err = filepath.WalkDir(uploads, func(path string, entry fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if entry.IsDir() {
				return nil
			}
			rel, e := filepath.Rel(uploads, path)
			if e != nil {
				return e
			}
			return add(path, "uploads/"+filepath.ToSlash(rel))
		})
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	manifest := map[string]any{"format": "ledger-backup", "version": 1, "created_at": time.Now().UTC().Format(time.RFC3339), "sha256": hashes, "notes": "Contains all users, password hashes and AI settings. Does not include .env; configure JWT_SECRET and DB_PATH on the new host. Stop writes before the final migration export."}
	w, e := z.Create("manifest.json")
	if e != nil {
		return e
	}
	return json.NewEncoder(w).Encode(manifest)
}
