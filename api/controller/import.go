package controller

import (
	"encoding/csv"
	"fmt"
	"io"
	"ledger/api/db"
	"ledger/api/model"
	"log"
	"mime/multipart"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// ImportWechat processes the uploaded WeChat bill CSV (although it says xlsx in prompt, user provided example is text/csv like structure, but user said xlsx file. We should support csv for now as the structure looks like csv, but if it is real xlsx we need excelize. Let's assume CSV first based on structure, or check file extension.
// Update: User said "微信支付账单信息: 为一个 xlsx 文件". So we must use excelize.
func ImportWechat(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "文件上传失败"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(500, gin.H{"error": "无法打开文件"})
		return
	}
	defer src.Close()

	// Parse CSV or Excel
	// WeChat exports CSV mostly, but user said xlsx. Let's try to detect or just support CSV as the example is text based.
	// Wait, WeChat official export IS csv but with .csv extension.
	// However, if user insists on xlsx, we use excelize.
	// Let's support both if possible or just assuming CSV as the text example suggests csv structure (comma/tab separated).
	// Actually WeChat export is CSV. The user might be mistaken or converted it.
	// But let's look at the structure: "微信支付账单明细...". This is typical WeChat CSV header.
	// If it is indeed XLSX, we need to read it differently.
	// Let's check file extension.

	filename := strings.ToLower(file.Filename)
	var records [][]string

	if strings.HasSuffix(filename, ".xlsx") {
		// Use excelize to read stream? excelize needs a file path or reader.
		// file.Open() returns multipart.File which is a ReaderAt.
		f, err := excelize.OpenReader(src)
		if err != nil {
			c.JSON(400, gin.H{"error": "无法解析Excel文件: " + err.Error()})
			return
		}
		defer f.Close()

		// Get first sheet
		rows, err := f.GetRows(f.GetSheetName(0))
		if err != nil {
			c.JSON(400, gin.H{"error": "无法读取Excel行: " + err.Error()})
			return
		}
		records = rows
	} else {
		// Assume CSV
		records, err = parseWechatCSV(src)
		if err != nil {
			c.JSON(400, gin.H{"error": "CSV解析失败: " + err.Error()})
			return
		}
	}

	// Filter and Process Records
	// We need to find the header row first.
	// Header: "交易时间", "交易类型", ...

	var validRecords []model.TransactionDraft
	var headerFound bool

	for _, row := range records {
		if !headerFound {
			// Check if this row is header
			// WeChat header usually starts with "交易时间"
			if len(row) > 0 && strings.TrimSpace(row[0]) == "交易时间" {
				headerFound = true
			}
			continue
		}

		// Process data rows
		if len(row) < 8 { // Minimal columns
			continue
		}

		// 交易时间 should contain ":" (time part) or "-" (date part)
		if !strings.Contains(row[0], "-") && !strings.Contains(row[0], ":") {
			continue
		}

		userID, _ := c.Get("userID")
		draft := processWechatRecord(userID.(uint), row)

		// Deduplication check
		var count int64
		db.DB.Model(&model.Transaction{}).Where("external_id = ?", draft.ExternalID).Count(&count)
		if count > 0 {
			draft.Status = "error"
			draft.ErrorMsg = "重复记录 (已存在)"
		}

		validRecords = append(validRecords, draft)
	}

	if !headerFound {
		c.JSON(400, gin.H{"error": "无效的微信账单格式 (未找到表头)"})
		return
	}

	// Save valid records
	var savedDrafts []model.TransactionDraft
	var importedCount int

	for _, draft := range validRecords {
		if err := db.DB.Create(&draft).Error; err == nil {
			savedDrafts = append(savedDrafts, draft)
			importedCount++
		}
	}

	// Trigger AI
	go analyzeDrafts(savedDrafts)

	c.JSON(200, gin.H{
		"message": fmt.Sprintf("成功导入 %d 条记录", importedCount),
		"count":   importedCount,
	})
}

func parseWechatCSV(file multipart.File) ([][]string, error) {
	// WeChat CSV is UTF-8 with BOM usually, or just UTF-8.
	// But sometimes it can be plain text.
	// Standard csv reader.
	csvReader := csv.NewReader(file)
	csvReader.LazyQuotes = true
	csvReader.FieldsPerRecord = -1

	return csvReader.ReadAll()
}

func processWechatRecord(userID uint, row []string) model.TransactionDraft {
	// WeChat Columns (based on provided example):
	// 0: 交易时间 (2025-11-13 11:29:18)
	// 1: 交易类型 (商户消费)
	// 2: 交易对方 (上海市第六人民医院)
	// 3: 商品 (戴赛-医疗费)
	// 4: 收/支 (支出)
	// 5: 金额(元) (¥5.84)
	// 6: 支付方式 (招商银行信用卡(1686))
	// 7: 当前状态 (支付成功)
	// 8: 交易单号 (4200002924202511133855988976)
	// 9: 商户单号 (C25111398013296372)
	// 10: 备注 (/)

	draft := model.TransactionDraft{
		UserID: userID,
		Status: "pending",
	}

	// 1. Time
	// Try parsing with location first
	loc, _ := time.LoadLocation("Local")
	tTime, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(row[0]), loc)
	if err != nil {
		// Try without seconds?
		tTime = time.Now() // Fallback
	}
	draft.TransactionTime = tTime

	// 2. Merchant & Remark
	merchant := strings.TrimSpace(row[2]) // 交易对方
	product := strings.TrimSpace(row[3])  // 商品
	draft.Merchant = merchant
	if merchant == "/" || merchant == "" {
		draft.Merchant = product
	}

	remark := strings.TrimSpace(row[10])
	if remark == "/" {
		remark = ""
	}
	draft.Remark = fmt.Sprintf("%s %s", product, remark)

	// 3. Amount & Type
	amountStr := strings.TrimSpace(row[5])
	amountStr = strings.ReplaceAll(amountStr, "¥", "")
	amountStr = strings.ReplaceAll(amountStr, ",", "") // Remove thousands separator
	var amount float64
	fmt.Sscanf(amountStr, "%f", &amount)
	draft.Amount = amount

	typeStr := strings.TrimSpace(row[4])
	statusStr := strings.TrimSpace(row[7]) // 当前状态

	if strings.Contains(statusStr, "退款") {
		draft.Status = "error"
		draft.ErrorMsg = "退款记录 (已跳过)"
	} else if typeStr == "支出" {
		// Specific handling for Pinduoduo and JD (lack of product info)
		if strings.Contains(merchant, "拼多多") || strings.Contains(merchant, "京东") {
			draft.Status = "error"
			draft.ErrorMsg = "缺少商品信息 (拼多多/京东), 请手动补充后分类"
		}
		// OK
	} else if typeStr == "收入" {
		draft.Status = "error"
		draft.ErrorMsg = "收入记录 (已跳过)"
	} else {
		draft.Status = "error"
		draft.ErrorMsg = "非支出记录: " + typeStr
	}

	// 交易类型过滤亲属卡
	trxType := strings.TrimSpace(row[1])
	if strings.Contains(trxType, "亲属卡") {
		draft.Status = "error"
		draft.ErrorMsg = "亲情卡 (已跳过)"
	}

	// 4. Source
	draft.Source = "微信"

	// 5. External ID
	draft.ExternalID = strings.TrimSpace(row[8]) // 交易单号

	if strings.TrimSpace(os.Getenv("STORE_IMPORT_RAW")) == "1" {
		raw := strings.Join(row, ",")
		if len(raw) > 500 {
			raw = raw[:500]
		}
		draft.RawData = raw
	}

	return draft
}

// ImportAlipay processes the uploaded Alipay bill CSV
func ImportAlipay(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "文件上传失败"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(500, gin.H{"error": "无法打开文件"})
		return
	}
	defer src.Close()

	// Parse CSV
	records, err := parseAlipayCSV(src)
	if err != nil {
		c.JSON(400, gin.H{"error": "CSV解析失败: " + err.Error()})
		return
	}

	userID, _ := c.Get("userID")
	uid := userID.(uint)

	var drafts []model.TransactionDraft
	var importedCount int

	for _, record := range records {
		draft := processAlipayRecord(uid, record)

		// Check deduplication in Transaction table
		var count int64
		db.DB.Model(&model.Transaction{}).Where("external_id = ?", draft.ExternalID).Count(&count)
		if count > 0 {
			draft.Status = "error"
			draft.ErrorMsg = "重复记录 (已存在)"
		}

		// Save draft
		if err := db.DB.Create(&draft).Error; err == nil {
			drafts = append(drafts, draft)
			importedCount++
		}
	}

	// Trigger AI analysis asynchronously
	go analyzeDrafts(drafts)

	c.JSON(200, gin.H{
		"message": fmt.Sprintf("成功导入 %d 条记录", importedCount),
		"count":   importedCount,
	})
}

func parseAlipayCSV(file multipart.File) ([][]string, error) {
	// Alipay CSV is usually GBK encoded
	reader := transform.NewReader(file, simplifiedchinese.GBK.NewDecoder())
	csvReader := csv.NewReader(reader)
	csvReader.LazyQuotes = true    // Allow relaxed parsing
	csvReader.FieldsPerRecord = -1 // Variable number of fields

	var records [][]string
	var headerFound bool

	for {
		line, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip malformed lines if possible, or return error
			continue
		}

		// Find header line starting with "交易时间"
		if !headerFound {
			if len(line) > 0 && strings.TrimSpace(line[0]) == "交易时间" {
				headerFound = true
				// Verify required columns exist here if needed
			}
			continue
		}

		// Process data lines
		// Alipay CSV often ends with statistics lines, check if line is a valid record
		// Valid record usually has a date in first column
		if len(line) < 10 {
			continue
		}

		// Basic check if first column looks like a date/time
		if !strings.Contains(line[0], ":") {
			continue
		}

		records = append(records, line)
	}

	if !headerFound {
		return nil, fmt.Errorf("无效的支付宝账单格式 (未找到表头)")
	}

	return records, nil
}

func processAlipayRecord(userID uint, row []string) model.TransactionDraft {
	// Column mapping based on standard Alipay export:
	// 0: 交易时间
	// 1: 交易分类
	// 2: 交易对方
	// 3: 对方账号
	// 4: 商品说明
	// 5: 收/支
	// 6: 金额
	// 7: 收/付款方式
	// 8: 交易状态
	// 9: 交易订单号
	// 10: 商家订单号
	// 11: 备注

	draft := model.TransactionDraft{
		UserID: userID,
		Status: "pending",
	}

	// 1. Transaction Time
	tTimeStr := strings.TrimSpace(row[0])
	// Try parsing with location first
	loc, _ := time.LoadLocation("Local")
	tTime, err := time.ParseInLocation("2006/01/02 15:04", tTimeStr, loc) // Common Alipay format
	if err != nil {
		log.Printf("Failed to parse time [%s] with location: %v", tTimeStr, err)
		tTime, err = time.ParseInLocation("2006/1/2 15:04", tTimeStr, loc)
		if err != nil {
			log.Printf("Failed to parse time [%s]: %v", tTimeStr, err)
			// Try another format just in case
			tTime, err = time.ParseInLocation("2006-01-02 15:04:05", tTimeStr, loc)
			if err != nil {
				log.Printf("Failed to parse time [%s] with format 2006-01-02 15:04:05: %v", tTimeStr, err)
			}
		}
	}
	draft.TransactionTime = tTime

	// 过滤亲情付
	alipayClassify := strings.TrimSpace(row[1])
	if strings.Contains(alipayClassify, "亲友代付") {
		draft.Status = "error"
		draft.ErrorMsg = "亲情卡消费 (已跳过)"
	}

	// 2. Merchant & Remark
	merchant := strings.TrimSpace(row[2])
	desc := strings.TrimSpace(row[4])
	draft.Merchant = merchant
	if merchant == "" || merchant == "/" {
		draft.Merchant = desc // Fallback to description
	}
	draft.Remark = fmt.Sprintf("%s - %s", desc, strings.TrimSpace(row[11]))

	// 3. Amount & Type
	amountStr := strings.TrimSpace(row[6])
	var amount float64
	fmt.Sscanf(amountStr, "%f", &amount)
	draft.Amount = amount

	typeStr := strings.TrimSpace(row[5]) // 收/支

	// Only record expenses
	if typeStr == "支出" {
		// Valid expense
	} else if typeStr == "收入" {
		draft.Status = "error"
		draft.ErrorMsg = "收入记录 (已跳过)"
	} else if typeStr == "不计收支" {
		draft.Status = "error"
		draft.ErrorMsg = "不计收支记录 (已跳过)"
	} else {
		draft.Status = "error"
		draft.ErrorMsg = "未知类型: " + typeStr
	}

	trxStatus := strings.TrimSpace(row[8]) // 交易状态
	if strings.Contains(trxStatus, "退款") {
		draft.Status = "error"
		draft.ErrorMsg = "退款记录 (已跳过)"
	} else if strings.Contains(trxStatus, "关闭") {
		draft.Status = "error"
		draft.ErrorMsg = "交易关闭 (已跳过)"
	}

	// 4. Source & Deduplication
	draft.Source = "支付宝"
	draft.ExternalID = strings.TrimSpace(row[9]) // Transaction No.

	if strings.TrimSpace(os.Getenv("STORE_IMPORT_RAW")) == "1" {
		raw := strings.Join(row, ",")
		if len(raw) > 500 {
			raw = raw[:500]
		}
		draft.RawData = raw
	}

	return draft
}

func analyzeDrafts(drafts []model.TransactionDraft) {
	// Simple worker pool or sequential processing
	for _, d := range drafts {
		if d.Status != "pending" {
			continue
		}

		// Update status to analyzing
		db.DB.Model(&d).Update("status", "analyzing")

		// Call AI
		l1, l2, err := AnalyzeWithAI(d.Merchant, d.Amount, d.TransactionTime.Format("2006-01-02 15:04:05"), d.Remark)

		updates := map[string]interface{}{}
		if err != nil {
			updates["status"] = "error"
			updates["error_msg"] = "AI 分析失败: " + err.Error()
		} else {
			updates["status"] = "success" // Ready for review
			updates["category_l1_name"] = l1
			updates["category_l2_name"] = l2
		}

		db.DB.Model(&d).Updates(updates)

		// Basic Rate Limiting: Sleep a bit to avoid hitting rate limits
		time.Sleep(500 * time.Millisecond)
	}
}
