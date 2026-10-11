// Package config 加载服务端 YAML 配置。
// Package config loads the server YAML configuration.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	MySQL   MySQLConfig   `yaml:"mysql"`
	Auth    AuthConfig    `yaml:"auth"`
	Storage StorageConfig `yaml:"storage"`
}

type ServerConfig struct {
	// 监听地址，如 ":8080"
	// Listen address, e.g. ":8080".
	Addr string `yaml:"addr"`
	// 可选：前端构建产物目录（如 "./web/dist"）；为空时只提供 API，前端由 nginx 等独立托管
	// Optional frontend build output directory (e.g. "./web/dist"); empty means API only,
	// with the frontend hosted separately (e.g. by nginx).
	WebDir string `yaml:"web_dir"`
	// 部署在 nginx 等反向代理之后时置 true，按 X-Real-IP 识别客户端（用于登录限流）
	// Set to true behind a reverse proxy such as nginx so clients are identified by X-Real-IP (for login rate limiting).
	TrustProxy bool `yaml:"trust_proxy"`
}

type MySQLConfig struct {
	DSN          string `yaml:"dsn"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

type AuthConfig struct {
	// access token 有效期（Go duration 格式，如 "24h"、"30m"），同时作为 cookie 过期时间
	// Access token lifetime (Go duration such as "24h", "30m"); also used as the cookie expiry.
	TokenTTL Duration `yaml:"token_ttl"`
	// 存放 access token 的 cookie 名
	// Name of the cookie that carries the access token.
	CookieName string `yaml:"cookie_name"`
	// HTTPS 部署时置 true，cookie 只通过 HTTPS 发送
	// Set to true behind HTTPS so the cookie is only sent over TLS.
	CookieSecure bool `yaml:"cookie_secure"`
	// 是否开放自助注册
	// Whether self-service registration is open.
	AllowRegister bool `yaml:"allow_register"`
	// 用户表为空时自动创建的初始账号
	// Initial account created when the user table is empty.
	InitAdmin InitAdmin `yaml:"init_admin"`
}

type InitAdmin struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type StorageConfig struct {
	// 图片等大文件的本地根目录，MySQL 只保存相对此目录的路径
	// Local root for large files such as images; MySQL only stores paths relative to it.
	UploadDir string `yaml:"upload_dir"`
	// 单张图片上传上限（MB）
	// Per-image upload limit in MB.
	MaxImageMB int64 `yaml:"max_image_mb"`
	// 导入数据包上传上限（MB）
	// Upload limit for import packages in MB.
	MaxImportMB int64 `yaml:"max_import_mb"`
}

// Duration 让 YAML 可以直接写 "24h" 这类字符串。
// Duration lets YAML use strings such as "24h".
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var raw string
	if err := node.Decode(&raw); err != nil {
		return err
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", raw, err)
	}
	d.Duration = v
	return nil
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{Addr: ":8080"},
		MySQL:  MySQLConfig{MaxOpenConns: 20, MaxIdleConns: 5},
		Auth: AuthConfig{
			TokenTTL:   Duration{24 * time.Hour},
			CookieName: "cw_access_token",
		},
		Storage: StorageConfig{UploadDir: "./data/uploads", MaxImageMB: 10, MaxImportMB: 512},
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.MySQL.DSN == "" {
		return nil, fmt.Errorf("mysql.dsn is required")
	}
	if cfg.Auth.TokenTTL.Duration <= 0 {
		return nil, fmt.Errorf("auth.token_ttl must be positive")
	}
	return cfg, nil
}
