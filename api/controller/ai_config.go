package controller

import (
	"fmt"
	"ledger/api/db"
	dbModel "ledger/api/model"
	"strings"
)

type aiConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

func getAIConfig() (aiConfig, error) {
	keys := []string{"openai_api_key", "openai_base_url", "openai_model", "ark_api_key", "ark_endpoint_id"}
	settingMap := map[string]string{}
	var settings []dbModel.SystemSetting
	db.DB.Where("setting_key IN ?", keys).Find(&settings)
	for _, s := range settings {
		settingMap[s.SettingKey] = s.Value
	}

	apiKey := strings.TrimSpace(settingMap["openai_api_key"])
	model := strings.TrimSpace(settingMap["openai_model"])
	baseURL := strings.TrimSpace(settingMap["openai_base_url"])

	if apiKey == "" {
		apiKey = strings.TrimSpace(settingMap["ark_api_key"])
	}
	if model == "" {
		model = strings.TrimSpace(settingMap["ark_endpoint_id"])
	}
	apiKey = firstAPIKey(apiKey)

	legacyArk := strings.TrimSpace(settingMap["ark_api_key"]) != ""
	if baseURL == "" {
		if legacyArk {
			baseURL = "https://ark.cn-beijing.volces.com/api/v3"
		} else {
			baseURL = "https://api.openai.com/v1"
		}
	}
	baseURL = strings.TrimRight(baseURL, "/")

	if apiKey == "" || model == "" {
		return aiConfig{}, fmt.Errorf("AI configuration missing (configure in Settings -> System Settings)")
	}
	return aiConfig{BaseURL: baseURL, APIKey: apiKey, Model: model}, nil
}

func firstAPIKey(raw string) string {
	// Allow multiple keys separated by comma/newline; pick the first non-empty.
	raw = strings.ReplaceAll(raw, "\n", ",")
	parts := strings.Split(raw, ",")
	for _, p := range parts {
		k := strings.TrimSpace(p)
		if k != "" {
			return k
		}
	}
	return ""
}
