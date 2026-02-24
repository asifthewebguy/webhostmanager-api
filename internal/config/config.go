package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	App      AppConfig
}

type ServerConfig struct {
	Port string
	Host string
	Env  string
}

type DatabaseConfig struct {
	DSN             string
	MigrationsPath  string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int
}

type AuthConfig struct {
	JWTSecret     string
	JWTExpiryHrs  int
	EncryptionKey string
}

type AppConfig struct {
	AllowedOrigins []string
}

func Load() (*Config, error) {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Defaults
	viper.SetDefault("SERVER_PORT", "8080")
	viper.SetDefault("SERVER_HOST", "0.0.0.0")
	viper.SetDefault("SERVER_ENV", "development")
	viper.SetDefault("DB_MAX_OPEN_CONNS", 25)
	viper.SetDefault("DB_MAX_IDLE_CONNS", 5)
	viper.SetDefault("DB_CONN_MAX_LIFETIME", 300)
	viper.SetDefault("MIGRATIONS_PATH", "file://migrations")
	viper.SetDefault("ALLOWED_ORIGINS", "http://localhost:3000")
	viper.SetDefault("JWT_EXPIRY_HRS", 24)
	viper.SetDefault("JWT_SECRET", "change-me-in-production")
	viper.SetDefault("ENCRYPTION_KEY", "change-me-in-production-32chars!")

	_ = viper.ReadInConfig()

	cfg := &Config{
		Server: ServerConfig{
			Port: viper.GetString("SERVER_PORT"),
			Host: viper.GetString("SERVER_HOST"),
			Env:  viper.GetString("SERVER_ENV"),
		},
		Database: DatabaseConfig{
			DSN:             viper.GetString("DATABASE_URL"),
			MigrationsPath:  viper.GetString("MIGRATIONS_PATH"),
			MaxOpenConns:    viper.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:    viper.GetInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetime: viper.GetInt("DB_CONN_MAX_LIFETIME"),
		},
		Auth: AuthConfig{
			JWTSecret:     viper.GetString("JWT_SECRET"),
			JWTExpiryHrs:  viper.GetInt("JWT_EXPIRY_HRS"),
			EncryptionKey: viper.GetString("ENCRYPTION_KEY"),
		},
		App: AppConfig{
			AllowedOrigins: strings.Split(viper.GetString("ALLOWED_ORIGINS"), ","),
		},
	}

	return cfg, nil
}
