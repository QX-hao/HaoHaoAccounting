package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Printf("skip .env: %v", err)
	}

	driver := normalizeDriver(fallbackEnv("DB_DRIVER", "postgres"))
	dsn := strings.TrimSpace(os.Getenv("DB_DSN"))
	if dsn == "" {
		log.Fatal("DB_DSN is required for dbmigrate")
	}

	db, err := openDB(driver, dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	if err := runMigrations(db, driver); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
}

func openDB(driver, dsn string) (*gorm.DB, error) {
	switch normalizeDriver(driver) {
	case "postgres":
		return gorm.Open(postgres.Open(dsn), &gorm.Config{})
	case "mysql":
		return gorm.Open(mysql.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER: %s", driver)
	}
}

func runMigrations(db *gorm.DB, driver string) error {
	dir, err := migrationsDir(driver)
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("no migration files found in %s", dir)
	}

	if err := ensureSchemaMigrations(db, driver); err != nil {
		return err
	}
	for _, file := range files {
		version := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		applied, err := migrationApplied(db, version)
		if err != nil {
			return err
		}
		if applied {
			log.Printf("skip migration %s", version)
			continue
		}

		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		log.Printf("apply migration %s", version)
		if err := db.Transaction(func(tx *gorm.DB) error {
			for _, statement := range splitSQLStatements(string(sqlBytes)) {
				if err := tx.Exec(statement).Error; err != nil {
					if isIgnorableMigrationError(err) {
						log.Printf("skip existing database object in %s: %v", version, err)
						continue
					}
					return fmt.Errorf("%s: %w", version, err)
				}
			}
			return tx.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", version, time.Now()).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

func migrationsDir(driver string) (string, error) {
	driver = normalizeDriver(driver)
	candidates := []string{
		filepath.Join("migrations", driver),
		filepath.Join("backend", "migrations", driver),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("migration directory not found for driver %s", driver)
}

func ensureSchemaMigrations(db *gorm.DB, driver string) error {
	switch normalizeDriver(driver) {
	case "postgres":
		return db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL
		)`).Error
	case "mysql":
		return db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at DATETIME(3) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`).Error
	default:
		return fmt.Errorf("unsupported DB_DRIVER: %s", driver)
	}
}

func migrationApplied(db *gorm.DB, version string) (bool, error) {
	var count int64
	if err := db.Table("schema_migrations").Where("version = ?", version).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func splitSQLStatements(sqlText string) []string {
	lines := strings.Split(sqlText, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		cleaned = append(cleaned, line)
	}

	parts := strings.Split(strings.Join(cleaned, "\n"), ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		statement := strings.TrimSpace(part)
		if statement != "" {
			statements = append(statements, statement)
		}
	}
	return statements
}

func isIgnorableMigrationError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key name") ||
		strings.Contains(message, "already exists")
}

func normalizeDriver(driver string) string {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "pgsql" {
		return "postgres"
	}
	return driver
}

func fallbackEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func loadDotEnv(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		clean := strings.TrimSpace(line)
		if clean == "" || strings.HasPrefix(clean, "#") {
			continue
		}
		key, value, ok := strings.Cut(clean, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return nil
}
