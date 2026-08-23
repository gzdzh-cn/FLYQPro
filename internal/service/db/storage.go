package db

import (
	"context"
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
	defaultAppDataDir = "LANChat"
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
	for _, migration := range []struct{ table, column, definition string }{
		{"profiles", "avatar_hash", "TEXT NOT NULL DEFAULT ''"},
		{"profiles", "avatar_version", "INTEGER NOT NULL DEFAULT 0"},
		{"peers", "avatar_hash", "TEXT NOT NULL DEFAULT ''"},
		{"peers", "avatar_version", "INTEGER NOT NULL DEFAULT 0"},
	} {
		var columns []struct {
			Name string `orm:"name"`
		}
		rows, queryErr := database.GetAll(ctx, "PRAGMA table_info("+migration.table+")")
		if queryErr != nil || rows.Structs(&columns) != nil {
			database = nil
			return gerror.NewCode(gcode.CodeInternalError, "检查 SQLite 字段失败")
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
				database = nil
				return gerror.WrapCode(gcode.CodeInternalError, err, "迁移 SQLite 字段失败")
			}
		}
	}
	if _, err := database.Exec(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(1, datetime('now')) ON CONFLICT(version) DO NOTHING`); err != nil {
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
	if !productionBuild {
		return filepath.Abs(filepath.Join(developmentResourceDir(), defaultSQLiteName))
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, defaultAppDataDir, defaultSQLiteName), nil
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
