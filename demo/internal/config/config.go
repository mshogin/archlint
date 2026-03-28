// Package config provides application configuration.
package config

// AppConfig holds all application settings.
type AppConfig struct {
	Host     string
	Port     int
	DSN      string
	LogLevel string
	Timeout  int
	MaxConn  int
	CacheURL string
	TLSCert  string
}

// Configurator is a fat interface that violates Interface Segregation Principle.
// VIOLATION: ISP - 8+ methods; clients are forced to depend on methods they do not use.
// Should be split into: DatabaseConfig, CacheConfig, ServerConfig, TLSConfig, etc.
type Configurator interface {
	// --- Server settings ---
	GetHost() string
	GetPort() int
	GetTimeout() int

	// --- Database settings ---
	GetDSN() string
	GetMaxConnections() int

	// --- Cache settings ---
	GetCacheURL() string

	// --- Logging ---
	GetLogLevel() string

	// --- TLS ---
	GetTLSCert() string
}

// DefaultConfig implements the fat Configurator interface.
type DefaultConfig struct {
	cfg AppConfig
}

// NewDefaultConfig creates a new DefaultConfig with default values.
func NewDefaultConfig() *DefaultConfig {
	return &DefaultConfig{cfg: AppConfig{
		Host:     "0.0.0.0",
		Port:     8080,
		DSN:      "postgres://localhost/demo",
		LogLevel: "info",
		Timeout:  30,
		MaxConn:  10,
		CacheURL: "redis://localhost:6379",
		TLSCert:  "",
	}}
}

func (c *DefaultConfig) GetHost() string         { return c.cfg.Host }
func (c *DefaultConfig) GetPort() int             { return c.cfg.Port }
func (c *DefaultConfig) GetTimeout() int          { return c.cfg.Timeout }
func (c *DefaultConfig) GetDSN() string           { return c.cfg.DSN }
func (c *DefaultConfig) GetMaxConnections() int   { return c.cfg.MaxConn }
func (c *DefaultConfig) GetCacheURL() string      { return c.cfg.CacheURL }
func (c *DefaultConfig) GetLogLevel() string      { return c.cfg.LogLevel }
func (c *DefaultConfig) GetTLSCert() string       { return c.cfg.TLSCert }
