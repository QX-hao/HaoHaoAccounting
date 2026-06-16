package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/config"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("skip .env: %v", err)
	}

	cfg, err := config.LoadStrict()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	dbCfg, err := migrationDatabaseConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}

	db, err := openDB(dbCfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() {
		if err := closeDB(db); err != nil {
			log.Printf("close database: %v", err)
		}
	}()
	if err := runMigrations(db, dbCfg.Driver); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
}

func migrationDatabaseConfig(cfg config.Config) (config.DatabaseConfig, error) {
	dsn := config.ExplicitDatabaseDSN()
	if dsn == "" {
		return config.DatabaseConfig{}, fmt.Errorf("DB_DSN is required for dbmigrate")
	}
	dbCfg := cfg.Database
	dbCfg.Driver = normalizeDriver(dbCfg.Driver)
	dbCfg.DSN = dsn
	return dbCfg, nil
}

func openDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	var (
		db  *gorm.DB
		err error
	)
	switch normalizeDriver(cfg.Driver) {
	case "postgres":
		db, err = gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	case "mysql":
		db, err = gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER: %s", cfg.Driver)
	}
	if err != nil {
		return nil, err
	}
	if err := store.ApplyPoolConfig(db, store.PoolConfig{
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
	}); err != nil {
		return nil, err
	}
	return db, nil
}

func closeDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
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
	statements := make([]string, 0, 8)
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	dollarQuoteTag := ""

	for i := 0; i < len(sqlText); i++ {
		if !inSingleQuote && !inDoubleQuote && dollarQuoteTag == "" && i+1 < len(sqlText) && sqlText[i] == '-' && sqlText[i+1] == '-' {
			current.WriteByte(' ')
			i = skipLineComment(sqlText, i)
			continue
		}
		if !inSingleQuote && !inDoubleQuote && dollarQuoteTag == "" && i+1 < len(sqlText) && sqlText[i] == '/' && sqlText[i+1] == '*' {
			current.WriteByte(' ')
			i = skipBlockComment(sqlText, i)
			continue
		}

		if !inSingleQuote && !inDoubleQuote && dollarQuoteTag == "" && sqlText[i] == '$' {
			if tag, ok := dollarQuote(sqlText[i:]); ok {
				dollarQuoteTag = tag
				current.WriteString(tag)
				i += len(tag) - 1
				continue
			}
		}

		if dollarQuoteTag != "" {
			if strings.HasPrefix(sqlText[i:], dollarQuoteTag) {
				current.WriteString(dollarQuoteTag)
				i += len(dollarQuoteTag) - 1
				dollarQuoteTag = ""
				continue
			}
			current.WriteByte(sqlText[i])
			continue
		}

		switch sqlText[i] {
		case '\'':
			current.WriteByte(sqlText[i])
			if inSingleQuote && i+1 < len(sqlText) && sqlText[i+1] == '\'' {
				i++
				current.WriteByte(sqlText[i])
				continue
			}
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			}
		case '"':
			current.WriteByte(sqlText[i])
			if inDoubleQuote && i+1 < len(sqlText) && sqlText[i+1] == '"' {
				i++
				current.WriteByte(sqlText[i])
				continue
			}
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		case ';':
			if inSingleQuote || inDoubleQuote {
				current.WriteByte(sqlText[i])
				continue
			}
			if statement := strings.TrimSpace(current.String()); statement != "" {
				statements = append(statements, statement)
			}
			current.Reset()
		default:
			current.WriteByte(sqlText[i])
		}
	}

	if statement := strings.TrimSpace(current.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}

func skipLineComment(sqlText string, start int) int {
	for start < len(sqlText) && sqlText[start] != '\n' {
		start++
	}
	return start
}

func skipBlockComment(sqlText string, start int) int {
	end := strings.Index(sqlText[start+2:], "*/")
	if end < 0 {
		return len(sqlText)
	}
	return start + end + 3
}

func dollarQuote(sqlText string) (string, bool) {
	end := strings.IndexByte(sqlText[1:], '$')
	if end < 0 {
		return "", false
	}
	end++
	for _, r := range sqlText[1:end] {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return "", false
		}
	}
	return sqlText[:end+1], true
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
