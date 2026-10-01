package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

type sequenceTestRow struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:100"`
}

func (sequenceTestRow) TableName() string { return "sequence_test_rows" }

func TestMigrationPreservesDeletedRowIDs(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB()
	defer sqlDB.Close()
	if err = database.Exec("CREATE TABLE sequence_test_rows (id integer PRIMARY KEY AUTOINCREMENT, name text NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	database.Exec("INSERT INTO sequence_test_rows (id,name) VALUES (1,'keep'), (100,'deleted')")
	database.Exec("DELETE FROM sequence_test_rows WHERE id = 100")
	for i := 0; i < 2; i++ {
		if err = autoMigratePreservingSequences(database, &sequenceTestRow{}); err != nil {
			t.Fatal(err)
		}
		var seq int64
		database.Raw("SELECT seq FROM sqlite_sequence WHERE name = 'sequence_test_rows'").Scan(&seq)
		if seq != 100 {
			t.Fatalf("lost sequence high-water mark: %d", seq)
		}
	}
	row := sequenceTestRow{Name: "new"}
	if err = database.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ID != 101 {
		t.Fatalf("reused deleted ID: %d", row.ID)
	}
}
