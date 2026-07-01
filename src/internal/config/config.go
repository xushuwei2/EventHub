package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var envFileCandidates = []string{
	"config/.env",
	filepath.Join("..", ".run", "config", ".env"),
}

type Config struct {
	DBHost             string
	DBPort             int
	DBUser             string
	DBPassword         string
	DBName             string
	DBTLS              string
	HTTPAddr           string
	AdminUsername      string
	AdminPasswordHash  string
	AdminSessionSecret string
	MaxBodyBytes       int64
	MaxEventsPerBatch  int
	MaxMessageLen      int
	MaxStackLen        int
}

func Load() (*Config, error) {
	loadEnvFiles(envFileCandidates...)

	port, err := strconv.Atoi(getEnv("DB_PORT", "3306"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_PORT: %w", err)
	}

	maxBody, err := strconv.ParseInt(getEnv("MAX_BODY_BYTES", "65536"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_BODY_BYTES: %w", err)
	}

	maxEvents, err := strconv.Atoi(getEnv("MAX_EVENTS_PER_BATCH", "20"))
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_EVENTS_PER_BATCH: %w", err)
	}

	cfg := &Config{
		DBHost:             getEnv("DB_HOST", "127.0.0.1"),
		DBPort:             port,
		DBUser:             getEnv("DB_USER", "root"),
		DBPassword:         getEnv("DB_PASSWORD", ""),
		DBName:             getEnv("DB_NAME", "eventhub"),
		DBTLS:              getEnv("DB_TLS", ""),
		HTTPAddr:           getEnv("HTTP_ADDR", ":8080"),
		AdminUsername:      getEnv("ADMIN_USERNAME", "admin"),
		AdminPasswordHash:  getEnv("ADMIN_PASSWORD_HASH", ""),
		AdminSessionSecret: getEnv("ADMIN_SESSION_SECRET", ""),
		MaxBodyBytes:       maxBody,
		MaxEventsPerBatch:  maxEvents,
		MaxMessageLen:      512,
		MaxStackLen:        8192,
	}

	if cfg.AdminPasswordHash == "" {
		return nil, fmt.Errorf("ADMIN_PASSWORD_HASH is required")
	}
	if cfg.AdminSessionSecret == "" {
		return nil, fmt.Errorf("ADMIN_SESSION_SECRET is required")
	}

	return cfg, nil
}

func (c *Config) DSN() string {
	query := "parseTime=true&charset=utf8mb4&loc=UTC"
	if strings.EqualFold(c.DBTLS, "false") {
		query += "&tls=false"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, query)
}

func EnvFilePath() string {
	for _, p := range envFileCandidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func UpdateEnvFileKey(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	found := false
	prefix := key + "="
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, _, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(k) == key {
			lines[i] = prefix + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, prefix+value)
	}

	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func loadEnvFiles(paths ...string) {
	for _, p := range paths {
		if err := loadEnvFile(p); err == nil {
			return
		}
	}
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
	return scanner.Err()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
