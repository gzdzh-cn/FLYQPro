package db

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	_ "github.com/gogf/gf/contrib/drivers/sqlite/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

const (
	defaultAppDataDir = "FlyQPro"
	legacyProductDir  = "POPChat"
	legacyAppDataDir  = "LANChat"
	defaultSQLiteName = "app.db"
)

var (
	storageMu sync.RWMutex
	database  gdb.DB
	dbPath    string
)

// Open configures GoFrame's default database group and initializes SQLite.
func Open(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	storageMu.Lock()
	defer storageMu.Unlock()
	if database != nil {
		return nil
	}

	path, err := resolveSQLitePath()
	if err != nil {
		return gerror.WrapCode(gcode.CodeInternalError, err, "解析 SQLite 路径失败")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return gerror.WrapCode(gcode.CodeInternalError, err, "创建 SQLite 目录失败")
	}
	if err := gdb.SetDefaultConfigGroup(gdb.ConfigGroup{{
		Type:             "sqlite",
		Name:             path,
		Extra:            "busy_timeout=5000",
		MaxOpenConnCount: 1,
		MaxIdleConnCount: 1,
		Debug:            !productionBuild,
	}}); err != nil {
		return gerror.WrapCode(gcode.CodeInternalError, err, "配置 GoFrame SQLite 连接失败")
	}

	database = g.DB()
	dbPath = path
	for _, statement := range schemaStatements {
		if _, err := database.Exec(ctx, statement); err != nil {
			database = nil
			return gerror.WrapCode(gcode.CodeInternalError, err, "初始化 SQLite 表结构失败")
		}
	}
	if err := ensureSchemaColumns(ctx, database); err != nil {
		database = nil
		return gerror.WrapCode(gcode.CodeInternalError, err, "迁移 SQLite 字段失败")
	}
	if _, err := database.Exec(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(1, datetime('now')) ON CONFLICT(version) DO NOTHING`); err != nil {
		database = nil
		return gerror.WrapCode(gcode.CodeInternalError, err, "记录 SQLite 迁移版本失败")
	}
	if _, err := database.Exec(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(2, datetime('now')) ON CONFLICT(version) DO NOTHING`); err != nil {
		database = nil
		return gerror.WrapCode(gcode.CodeInternalError, err, "记录 SQLite 迁移版本失败")
	}
	if _, err := database.Exec(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(3, datetime('now')) ON CONFLICT(version) DO NOTHING`); err != nil {
		database = nil
		return gerror.WrapCode(gcode.CodeInternalError, err, "记录 SQLite 迁移版本失败")
	}
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := database.Exec(ctx, pragma); err != nil {
			database = nil
			return gerror.WrapCode(gcode.CodeInternalError, err, "设置 SQLite 参数失败")
		}
	}
	return nil
}

// ensureSchemaColumns upgrades databases created by older builds. CREATE TABLE
// IF NOT EXISTS does not add columns to an existing SQLite table, so every
// field used by the current storage layer is checked independently and added
// with a backward-compatible default.
func ensureSchemaColumns(ctx context.Context, database gdb.DB) error {
	migrations := []struct{ table, column, definition string }{
		{"profiles", "avatar_path", "TEXT NOT NULL DEFAULT ''"},
		{"profiles", "avatar_hash", "TEXT NOT NULL DEFAULT ''"},
		{"profiles", "avatar_version", "INTEGER NOT NULL DEFAULT 0"},
		{"profiles", "discoverable", "INTEGER NOT NULL DEFAULT 0"},
		{"profiles", "auto_save", "INTEGER NOT NULL DEFAULT 0"},
		{"profiles", "file_save_path", "TEXT NOT NULL DEFAULT ''"},
		{"profiles", "theme", "TEXT NOT NULL DEFAULT 'system'"},
		{"profiles", "launch_at_startup", "INTEGER NOT NULL DEFAULT 0"},
		{"profiles", "created_at", "TEXT NOT NULL DEFAULT ''"},
		{"profiles", "updated_at", "TEXT NOT NULL DEFAULT ''"},
		{"device_identity", "public_key_pem", "TEXT NOT NULL DEFAULT ''"},
		{"device_identity", "private_key_pem", "TEXT NOT NULL DEFAULT ''"},
		{"device_identity", "certificate_pem", "TEXT NOT NULL DEFAULT ''"},
		{"device_identity", "certificate_fingerprint", "TEXT NOT NULL DEFAULT ''"},
		{"device_identity", "created_at", "TEXT NOT NULL DEFAULT ''"},
		{"device_identity", "updated_at", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "avatar_path", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "avatar_hash", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "avatar_version", "INTEGER NOT NULL DEFAULT 0"},
		{"peers", "platform", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "os_version", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "ip", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "port", "INTEGER NOT NULL DEFAULT 0"},
		{"peers", "public_key_pem", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "certificate_fingerprint", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "relation", "TEXT NOT NULL DEFAULT 'discovered'"},
		{"peers", "remark", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "protocol_name", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "protocol_major", "INTEGER NOT NULL DEFAULT 0"},
		{"peers", "discovery_magic", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "capabilities", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "discovery_visible", "INTEGER NOT NULL DEFAULT 0"},
		{"peers", "last_seen", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "created_at", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "updated_at", "TEXT NOT NULL DEFAULT ''"},
		{"friend_requests", "nickname", "TEXT NOT NULL DEFAULT ''"},
		{"friend_requests", "message", "TEXT NOT NULL DEFAULT ''"},
		{"friend_requests", "status", "TEXT NOT NULL DEFAULT 'pending'"},
		{"friend_requests", "direction", "TEXT NOT NULL DEFAULT ''"},
		{"friend_requests", "created_at", "TEXT NOT NULL DEFAULT ''"},
		{"friend_requests", "accepted_at", "TEXT NOT NULL DEFAULT ''"},
		{"friend_requests", "updated_at", "TEXT NOT NULL DEFAULT ''"},
		{"conversations", "last_message", "TEXT NOT NULL DEFAULT ''"},
		{"conversations", "last_message_at", "TEXT NOT NULL DEFAULT ''"},
		{"conversations", "unread_count", "INTEGER NOT NULL DEFAULT 0"},
		{"conversations", "pinned", "INTEGER NOT NULL DEFAULT 0"},
		{"conversations", "created_at", "TEXT NOT NULL DEFAULT ''"},
		{"conversations", "updated_at", "TEXT NOT NULL DEFAULT ''"},
		{"messages", "kind", "TEXT NOT NULL DEFAULT 'text'"},
		{"messages", "content", "TEXT NOT NULL DEFAULT ''"},
		{"messages", "status", "TEXT NOT NULL DEFAULT 'sent'"},
		{"messages", "created_at", "TEXT NOT NULL DEFAULT ''"},
		{"attachments", "mime_type", "TEXT NOT NULL DEFAULT 'application/octet-stream'"},
		{"attachments", "file_size", "INTEGER NOT NULL DEFAULT 0"},
		{"attachments", "sha256", "TEXT NOT NULL DEFAULT ''"},
		{"attachments", "thumbnail_data", "TEXT NOT NULL DEFAULT ''"},
		{"attachments", "thumbnail_mime", "TEXT NOT NULL DEFAULT ''"},
		{"attachments", "local_path", "TEXT NOT NULL DEFAULT ''"},
		{"attachments", "status", "TEXT NOT NULL DEFAULT 'pending'"},
		{"attachments", "created_at", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, migration := range migrations {
		var columns []struct {
			Name string `orm:"name"`
		}
		rows, err := database.GetAll(ctx, "PRAGMA table_info("+migration.table+")")
		if err != nil || rows.Structs(&columns) != nil {
			return gerror.New("检查 SQLite 字段失败")
		}
		found := false
		for _, column := range columns {
			if column.Name == migration.column {
				found = true
				break
			}
		}
		if !found {
			if _, err := database.Exec(ctx, "ALTER TABLE "+migration.table+" ADD COLUMN "+migration.column+" "+migration.definition); err != nil {
				return err
			}
		}
	}
	// Older builds stored the approval moment in updated_at only. Preserve
	// that historical timestamp when introducing the dedicated field.
	if _, err := database.Exec(ctx, `UPDATE friend_requests SET accepted_at=updated_at WHERE status='accepted' AND accepted_at=''`); err != nil {
		return err
	}
	return nil
}

func Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	storageMu.Lock()
	defer storageMu.Unlock()
	if database == nil {
		return nil
	}
	err := database.Close(ctx)
	database = nil
	return err
}

func DB() gdb.DB {
	storageMu.RLock()
	defer storageMu.RUnlock()
	return database
}

func Path() string {
	storageMu.RLock()
	defer storageMu.RUnlock()
	return dbPath
}

func ready() error {
	if DB() == nil {
		return gerror.NewCode(gcode.CodeInternalError, "SQLite 尚未初始化")
	}
	return nil
}

func resolveSQLitePath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("GOFLY_DB_PATH")); override != "" {
		return filepath.Abs(override)
	}
	if override := strings.TrimSpace(os.Getenv("FLYQPRO_DATA_DIR")); override != "" {
		return filepath.Abs(filepath.Join(override, defaultSQLiteName))
	}
	// Keep the former environment variables readable for one-way migration
	// and scripted deployments. New documentation uses FLYQPRO_DATA_DIR.
	if override := strings.TrimSpace(os.Getenv("POPCHAT_DATA_DIR")); override != "" {
		return filepath.Abs(filepath.Join(override, defaultSQLiteName))
	}
	if override := strings.TrimSpace(os.Getenv("LANCHAT_DATA_DIR")); override != "" {
		return filepath.Abs(filepath.Join(override, defaultSQLiteName))
	}
	if !productionBuild {
		return filepath.Abs(filepath.Join(developmentResourceDir(), defaultSQLiteName))
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	preferred := filepath.Join(configDir, defaultAppDataDir, defaultSQLiteName)
	legacyProduct := filepath.Join(configDir, legacyProductDir, defaultSQLiteName)
	legacy := filepath.Join(configDir, legacyAppDataDir, defaultSQLiteName)
	if _, err := os.Stat(preferred); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if _, err := os.Stat(preferred); os.IsNotExist(err) {
		if _, legacyErr := os.Stat(legacyProduct); legacyErr == nil {
			if err := migrateLegacyDirectory(filepath.Dir(legacyProduct), filepath.Dir(preferred)); err != nil {
				return "", err
			}
			return preferred, nil
		}
		if _, legacyErr := os.Stat(legacy); legacyErr == nil {
			if err := migrateLegacyDirectory(filepath.Dir(legacy), filepath.Dir(preferred)); err != nil {
				return "", err
			}
			return preferred, nil
		}
	}
	return preferred, nil
}

func migrateLegacyDirectory(source, target string) error {
	if source == target {
		return nil
	}
	return filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if info.IsDir() {
			return os.MkdirAll(destination, info.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode())
		if os.IsExist(err) {
			_ = input.Close()
			return nil
		}
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputErr := input.Close()
		outputErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputErr != nil {
			return inputErr
		}
		return outputErr
	})
}

func developmentResourceDir() string {
	if cwd, err := os.Getwd(); err == nil {
		for current := cwd; ; current = filepath.Dir(current) {
			if info, resourceErr := os.Stat(filepath.Join(current, "resource")); resourceErr == nil && info.IsDir() {
				return filepath.Join(current, "resource")
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
		}
	}
	if executable, err := os.Executable(); err == nil {
		if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
			executable = resolved
		}
		executableDir := filepath.Dir(executable)
		for _, candidate := range []string{
			filepath.Join(executableDir, "..", "resource"),
			filepath.Join(executableDir, "..", "..", "..", "..", "resource"),
		} {
			if info, resourceErr := os.Stat(candidate); resourceErr == nil && info.IsDir() {
				return candidate
			}
		}
	}
	_, sourceFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(sourceFile), "..", "..", "resource")
}

func invalidError(message string) error {
	return gerror.NewCode(gcode.CodeInvalidParameter, message)
}

func notFoundError(message string) error {
	return gerror.NewCode(gcode.CodeNotFound, message)
}
