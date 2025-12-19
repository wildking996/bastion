package models

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Bastion model
type Bastion struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	Name           string `gorm:"uniqueIndex;not null" json:"name"`
	Host           string `gorm:"not null" json:"host"`
	Port           int    `gorm:"default:22" json:"port"`
	Username       string `gorm:"not null" json:"username"`
	Password       string `json:"password,omitempty"`
	PkeyPath       string `json:"pkey_path,omitempty"`
	PkeyPassphrase string `json:"pkey_passphrase,omitempty"`
}

// BastionCreate request payload for creating a bastion host
type BastionCreate struct {
	Name           string `json:"name"`
	Host           string `json:"host" binding:"required"`
	Port           int    `json:"port"`
	Username       string `json:"username" binding:"required"`
	Password       string `json:"password"`
	PkeyPath       string `json:"pkey_path"`
	PkeyPassphrase string `json:"pkey_passphrase"`
}

// Normalize trims whitespace from input fields
func (b *BastionCreate) Normalize() {
	b.Name = strings.TrimSpace(b.Name)
	b.Host = strings.TrimSpace(b.Host)
	b.Username = strings.TrimSpace(b.Username)
	b.Password = strings.TrimSpace(b.Password)
	b.PkeyPath = strings.TrimSpace(b.PkeyPath)
	b.PkeyPassphrase = strings.TrimSpace(b.PkeyPassphrase)
}

// Mapping port mapping model
type Mapping struct {
	ID         string `gorm:"primaryKey" json:"id"`
	LocalHost  string `gorm:"default:'127.0.0.1'" json:"local_host"`
	LocalPort  int    `gorm:"not null" json:"local_port"`
	RemoteHost string `json:"remote_host"`
	RemotePort int    `json:"remote_port"`
	ChainJSON  string `gorm:"column:chain_json;default:'[]'" json:"-"`
	AllowJSON  string `gorm:"column:allow_cidrs_json;default:'[]'" json:"-"`
	DenyJSON   string `gorm:"column:deny_cidrs_json;default:'[]'" json:"-"`
	Type       string `gorm:"default:'tcp'" json:"type"`
	AutoStart  bool   `gorm:"default:false" json:"auto_start"`
}

// GetChain returns the chain as a slice
func (m *Mapping) GetChain() []string {
	var chain []string
	if m.ChainJSON != "" {
		_ = json.Unmarshal([]byte(m.ChainJSON), &chain)
	}
	return chain
}

// SetChain stores the chain slice as JSON
func (m *Mapping) SetChain(chain []string) {
	data, _ := json.Marshal(chain)
	m.ChainJSON = string(data)
}

func (m *Mapping) GetAllowCIDRs() []string {
	var cidrs []string
	if m.AllowJSON != "" {
		_ = json.Unmarshal([]byte(m.AllowJSON), &cidrs)
	}
	return cidrs
}

func (m *Mapping) SetAllowCIDRs(cidrs []string) {
	data, _ := json.Marshal(cidrs)
	m.AllowJSON = string(data)
}

func (m *Mapping) GetDenyCIDRs() []string {
	var cidrs []string
	if m.DenyJSON != "" {
		_ = json.Unmarshal([]byte(m.DenyJSON), &cidrs)
	}
	return cidrs
}

func (m *Mapping) SetDenyCIDRs(cidrs []string) {
	data, _ := json.Marshal(cidrs)
	m.DenyJSON = string(data)
}

// MappingCreate request payload for creating a mapping
type MappingCreate struct {
	ID         string   `json:"id"`
	LocalHost  string   `json:"local_host"`
	LocalPort  int      `json:"local_port" binding:"required"`
	RemoteHost string   `json:"remote_host"`
	RemotePort int      `json:"remote_port"`
	Chain      []string `json:"chain"`
	AllowCIDRs []string `json:"allow_cidrs"`
	DenyCIDRs  []string `json:"deny_cidrs"`
	Type       string   `json:"type"`
	AutoStart  bool     `json:"auto_start"`
}

// Normalize trims whitespace from input fields
func (m *MappingCreate) Normalize() {
	m.ID = strings.TrimSpace(m.ID)
	m.LocalHost = strings.TrimSpace(m.LocalHost)
	m.RemoteHost = strings.TrimSpace(m.RemoteHost)
	m.Type = strings.TrimSpace(m.Type)

	for i, name := range m.Chain {
		m.Chain[i] = strings.TrimSpace(name)
	}

	normalizeCIDRs := func(in []string) []string {
		out := make([]string, 0, len(in))
		for _, v := range in {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			out = append(out, v)
		}
		return out
	}
	m.AllowCIDRs = normalizeCIDRs(m.AllowCIDRs)
	m.DenyCIDRs = normalizeCIDRs(m.DenyCIDRs)
}

// MappingRead response model for reading mappings
type MappingRead struct {
	ID         string   `json:"id"`
	LocalHost  string   `json:"local_host"`
	LocalPort  int      `json:"local_port"`
	RemoteHost string   `json:"remote_host"`
	RemotePort int      `json:"remote_port"`
	Chain      []string `json:"chain"`
	AllowCIDRs []string `json:"allow_cidrs"`
	DenyCIDRs  []string `json:"deny_cidrs"`
	Type       string   `json:"type"`
	AutoStart  bool     `json:"auto_start"`
	Running    bool     `json:"running"`
}

// BeforeCreate GORM hook - auto-generate name when missing
func (b *Bastion) BeforeCreate(tx *gorm.DB) error {
	if strings.TrimSpace(b.Name) == "" {
		b.Name = fmt.Sprintf("%s:%d", strings.TrimSpace(b.Host), b.Port)
	}
	return nil
}
