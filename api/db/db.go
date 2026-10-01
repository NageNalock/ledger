package db

import (
	"ledger/api/model"
	"log"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	var err error
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "ledger.db"
	}

	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto Migrate
	err = autoMigratePreservingSequences(DB, &model.User{}, &model.SystemSetting{}, &model.CategoryL1{}, &model.CategoryL2{}, &model.Transaction{}, &model.Project{}, &model.TransactionDraft{}, &model.AnalysisRun{}, &model.Budget{}, &model.BudgetLine{})
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Seed initial categories if empty
	seedCategories()

	// Seed admin user
	seedAdmin()
}

func seedAdmin() {
	if strings.TrimSpace(os.Getenv("SEED_ADMIN")) != "1" {
		return
	}

	var count int64
	DB.Model(&model.User{}).Where("role = ?", "admin").Count(&count)
	if count > 0 {
		return
	}

	username := strings.TrimSpace(os.Getenv("ADMIN_USERNAME"))
	password := os.Getenv("ADMIN_PASSWORD")
	if username == "" {
		username = "admin"
	}
	if strings.TrimSpace(password) == "" {
		log.Println("SEED_ADMIN=1 but ADMIN_PASSWORD is empty; skip seeding admin")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash admin password: %v", err)
		return
	}
	admin := model.User{Username: username, PasswordHash: string(hashedPassword), Role: "admin"}

	if err := DB.Create(&admin).Error; err != nil {
		log.Printf("Failed to seed admin user: %v", err)
	} else {
		log.Printf("Admin user seeded successfully (username: %s)", username)
	}
}

func seedCategories() {
	var count int64
	DB.Model(&model.CategoryL1{}).Count(&count)
	if count > 0 {
		return
	}

	// Predefined Categories
	categories := []struct {
		Name string
		Subs []string
	}{
		{"餐饮美食", []string{"早餐", "午餐", "晚餐", "外卖", "零食", "饮料", "买菜", "水果"}},
		{"交通出行", []string{"公交地铁", "打车费用", "加油费", "停车费", "火车票", "飞机票", "保养维修", "过路费"}},
		{"购物消费", []string{"服饰", "日用品", "电子产品", "化妆护肤", "家居家纺", "母婴亲子", "宠物用品"}},
		{"娱乐休闲", []string{"电影", "游戏", "旅游", "运动健身", "洗浴足疗", "KTV", "演出展览"}},
		{"生活缴费", []string{"水费", "电费", "燃气费", "话费宽带", "物业费", "房租", "保险"}},
		{"医疗保健", []string{"药品", "挂号", "体检", "治疗费"}},
		{"教育培训", []string{"书籍", "课程", "学费", "考试费"}},
		{"人情往来", []string{"红包", "礼物", "请客"}},
		{"金融理财", []string{"投资亏损", "手续费", "利息支出"}},
		{"其他支出", []string{"其他", "意外丢失", "未分类"}},
	}

	for _, c := range categories {
		l1 := model.CategoryL1{Name: c.Name}
		DB.Create(&l1)
		for _, sub := range c.Subs {
			DB.Create(&model.CategoryL2{ParentID: l1.ID, Name: sub})
		}
	}
}

// SQLite table rebuilds may reset AUTOINCREMENT to MAX(id), forgetting IDs
// belonging to deleted rows. Preserve the high-water marks across migrations.
func autoMigratePreservingSequences(database *gorm.DB, models ...any) error {
	type sequence struct {
		Name string
		Seq  int64
	}
	var sequences []sequence
	if database.Migrator().HasTable("sqlite_sequence") {
		if err := database.Table("sqlite_sequence").Find(&sequences).Error; err != nil {
			return err
		}
	}
	if err := database.AutoMigrate(models...); err != nil {
		return err
	}
	for _, s := range sequences {
		if err := database.Exec("UPDATE sqlite_sequence SET seq = ? WHERE name = ? AND seq < ?", s.Seq, s.Name, s.Seq).Error; err != nil {
			return err
		}
	}
	return nil
}
