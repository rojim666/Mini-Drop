package apiserver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"mini-drop/internal/minidrop"
)

const (
	aiSettingEnabled    = "ai.enabled"
	aiSettingBaseURL    = "ai.base_url"
	aiSettingAPIKey     = "ai.api_key"
	aiSettingModel      = "ai.model"
	aiSettingTimeoutSec = "ai.timeout_sec"
	aiSettingMaxTokens  = "ai.max_tokens"
)

type aiRuntimeState struct {
	Enabled      bool
	BaseURL      string
	APIKey       string
	Model        string
	TimeoutSec   int
	MaxTokens    int
	UpdatedAt    *time.Time
	StoredFields map[string]bool
}

func defaultAIState(cfg Config) aiRuntimeState {
	cfg = cfg.withDefaults()
	return aiRuntimeState{
		Enabled:      cfg.AIEnabled,
		BaseURL:      strings.TrimSpace(cfg.AIBaseURL),
		APIKey:       strings.TrimSpace(cfg.AIAPIKey),
		Model:        strings.TrimSpace(cfg.AIModel),
		TimeoutSec:   int(cfg.AITimeout.Seconds()),
		MaxTokens:    cfg.AIMaxTokens,
		StoredFields: map[string]bool{},
	}
}

func loadAIState(db *gorm.DB, fallback aiRuntimeState) (aiRuntimeState, error) {
	state := fallback
	state.StoredFields = map[string]bool{}

	var settings []minidrop.AppSetting
	if err := db.Where("key IN ?", []string{
		aiSettingEnabled,
		aiSettingBaseURL,
		aiSettingAPIKey,
		aiSettingModel,
		aiSettingTimeoutSec,
		aiSettingMaxTokens,
	}).Find(&settings).Error; err != nil {
		return aiRuntimeState{}, err
	}

	for _, setting := range settings {
		state.StoredFields[setting.Key] = true
		if state.UpdatedAt == nil || setting.UpdatedAt.After(*state.UpdatedAt) {
			updatedAt := setting.UpdatedAt
			state.UpdatedAt = &updatedAt
		}
		switch setting.Key {
		case aiSettingEnabled:
			state.Enabled = boolSettingValue(setting.Value, state.Enabled)
		case aiSettingBaseURL:
			if value := strings.TrimSpace(setting.Value); value != "" {
				state.BaseURL = value
			}
		case aiSettingAPIKey:
			state.APIKey = strings.TrimSpace(setting.Value)
		case aiSettingModel:
			if value := strings.TrimSpace(setting.Value); value != "" {
				state.Model = value
			}
		case aiSettingTimeoutSec:
			state.TimeoutSec = intSettingValue(setting.Value, state.TimeoutSec)
		case aiSettingMaxTokens:
			state.MaxTokens = intSettingValue(setting.Value, state.MaxTokens)
		}
	}

	return normalizeAIState(state), nil
}

func saveAIState(db *gorm.DB, state aiRuntimeState, now time.Time) (aiRuntimeState, error) {
	state = normalizeAIState(state)
	settings := []minidrop.AppSetting{
		{Key: aiSettingEnabled, Value: boolSettingString(state.Enabled), Secret: false},
		{Key: aiSettingBaseURL, Value: state.BaseURL, Secret: false},
		{Key: aiSettingAPIKey, Value: state.APIKey, Secret: true},
		{Key: aiSettingModel, Value: state.Model, Secret: false},
		{Key: aiSettingTimeoutSec, Value: fmt.Sprintf("%d", state.TimeoutSec), Secret: false},
		{Key: aiSettingMaxTokens, Value: fmt.Sprintf("%d", state.MaxTokens), Secret: false},
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		for _, setting := range settings {
			var existing minidrop.AppSetting
			err := tx.First(&existing, "key = ?", setting.Key).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				setting.CreatedAt = now
				setting.UpdatedAt = now
				if err := tx.Create(&setting).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			existing.Value = setting.Value
			existing.Secret = setting.Secret
			existing.UpdatedAt = now
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return aiRuntimeState{}, err
	}

	state.UpdatedAt = &now
	state.StoredFields = map[string]bool{
		aiSettingEnabled:    true,
		aiSettingBaseURL:    true,
		aiSettingAPIKey:     true,
		aiSettingModel:      true,
		aiSettingTimeoutSec: true,
		aiSettingMaxTokens:  true,
	}
	return state, nil
}

func normalizeAIState(state aiRuntimeState) aiRuntimeState {
	state.BaseURL = strings.TrimSpace(state.BaseURL)
	if state.BaseURL == "" {
		state.BaseURL = "https://api.openai.com/v1"
	}
	state.APIKey = strings.TrimSpace(state.APIKey)
	state.Model = strings.TrimSpace(state.Model)
	if state.Model == "" {
		state.Model = "gpt-4o-mini"
	}
	if state.TimeoutSec <= 0 {
		state.TimeoutSec = 20
	}
	if state.MaxTokens <= 0 {
		state.MaxTokens = 800
	}
	if state.StoredFields == nil {
		state.StoredFields = map[string]bool{}
	}
	return state
}

func validateAIState(state aiRuntimeState) error {
	state = normalizeAIState(state)
	if state.TimeoutSec < 3 || state.TimeoutSec > 120 {
		return errors.New("timeout_sec must be between 3 and 120")
	}
	if state.MaxTokens < 128 || state.MaxTokens > 4096 {
		return errors.New("max_tokens must be between 128 and 4096")
	}
	if _, err := chatCompletionsURL(state.BaseURL); err != nil {
		return err
	}
	if state.Enabled && strings.TrimSpace(state.APIKey) == "" {
		return errors.New("api_key is required when AI attribution is enabled")
	}
	return nil
}

func aiConfigFromState(state aiRuntimeState) Config {
	state = normalizeAIState(state)
	return Config{
		AIEnabled:   state.Enabled,
		AIBaseURL:   state.BaseURL,
		AIAPIKey:    state.APIKey,
		AIModel:     state.Model,
		AITimeout:   time.Duration(state.TimeoutSec) * time.Second,
		AIMaxTokens: state.MaxTokens,
	}
}

func boolSettingValue(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func boolSettingString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func intSettingValue(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func maskedSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 8 {
		return "****"
	}
	return string(runes[:4]) + "..." + string(runes[len(runes)-4:])
}
