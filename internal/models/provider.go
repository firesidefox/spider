package models

import "time"

type Provider struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	EncryptedAPIKey string    `json:"-"`
	BaseURL         string    `json:"base_url"`
	SelectedModel   string    `json:"selected_model"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ProviderModel struct {
	ID          int       `json:"id"`
	ProviderID  string    `json:"provider_id"`
	ModelID     string    `json:"model_id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}
