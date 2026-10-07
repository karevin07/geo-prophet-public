package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Address        string
	DatabaseURL    string
	AllowedOrigins []string
	// AdminLogins are the Tailscale logins allowed into the admin console (ADMIN_LOGINS,
	// comma-separated). Empty admits nobody.
	AdminLogins []string
	// BackupDir is where the backup service's dumps are mounted (BACKUP_DIR), or empty.
	BackupDir           string
	BackupKeep          int
	BackupIntervalHours int
}

func Load() Config {
	address := os.Getenv("API_ADDR")
	if address == "" {
		address = ":8080"
	}
	origins := splitNonEmpty(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000"}
	}
	return Config{
		Address: address, DatabaseURL: os.Getenv("DATABASE_URL"), AllowedOrigins: origins,
		AdminLogins: splitNonEmpty(os.Getenv("ADMIN_LOGINS")), BackupDir: os.Getenv("BACKUP_DIR"),
		BackupKeep:          intOr(os.Getenv("BACKUP_KEEP"), 7),
		BackupIntervalHours: intOr(os.Getenv("BACKUP_INTERVAL"), 86400) / 3600,
	}
}

func intOr(value string, fallback int) int {
	if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
		return parsed
	}
	return fallback
}

func splitNonEmpty(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
