package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newLegacyTimestampMigrationDB spins up an isolated in-memory SQLite database and points the
// package-global DB at it for the duration of the test, mirroring newUserMigrationDB so the unified
// timestamp pre-migration can be exercised without disturbing the shared database.
func newLegacyTimestampMigrationDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	initCol()

	orig := DB
	DB = db
	t.Cleanup(func() { DB = orig })
}

// TestLegacyTimestampBigintColumns_CoversAllInt64TimeColumns locks the exact set, order, and per-column
// default of the unified timestamp pre-migration: every int64 Unix-second created_at/updated_at/
// last_login_at column across the AutoMigrated models that a legacy or externally-created PostgreSQL
// schema may store as a timestamp/date, triggering SQLSTATE 42846. Only users.last_login_at restores a
// SQL default (0, matching gorm:"default:0"); the rest are written by Go (autoCreateTime or
// BeforeCreate/BeforeUpdate hooks) and must restore none.
func TestLegacyTimestampBigintColumns_CoversAllInt64TimeColumns(t *testing.T) {
	assert.Equal(t, []legacyTimestampBigintColumn{
		{Table: "users", Name: "created_at"},
		{Table: "users", Name: "last_login_at", RestoreDefault: "0"},
		{Table: "logs", Name: "created_at"},
		{Table: "quota_data", Name: "created_at"},
		{Table: "tasks", Name: "created_at"},
		{Table: "tasks", Name: "updated_at"},
		{Table: "checkins", Name: "created_at"},
		{Table: "user_subscriptions", Name: "created_at"},
		{Table: "user_subscriptions", Name: "updated_at"},
		{Table: "subscription_plans", Name: "created_at"},
		{Table: "subscription_plans", Name: "updated_at"},
		{Table: "subscription_pre_consume_records", Name: "created_at"},
		{Table: "subscription_pre_consume_records", Name: "updated_at"},
		{Table: "system_instances", Name: "created_at"},
		{Table: "system_instances", Name: "updated_at"},
		{Table: "system_tasks", Name: "created_at"},
		{Table: "system_tasks", Name: "updated_at"},
		{Table: "system_task_locks", Name: "updated_at"},
	}, legacyTimestampBigintColumns())

	// last_login_at is the only column with a SQL default; every other entry is hook/auto-written and
	// must restore none, so the converted schema matches the model and AutoMigrate does not re-ALTER.
	for _, c := range legacyTimestampBigintColumns() {
		if c.Table == "users" && c.Name == "last_login_at" {
			assert.Equal(t, "0", c.RestoreDefault, "users.last_login_at restores DEFAULT 0")
		} else {
			assert.Emptyf(t, c.RestoreDefault, "%s.%s is hook/auto-written and must restore no SQL default", c.Table, c.Name)
		}
	}

	// The timestamp guard and the defaulted-int bigint guard must touch disjoint users columns: the
	// verbatim ::bigint cast in userBigintColumns cannot convert a timestamp, and EXTRACT(EPOCH ...)
	// must not run on a plain integer column.
	bigint := make(map[string]struct{})
	for _, c := range userBigintColumns() {
		bigint[c.Name] = struct{}{}
	}
	for _, c := range legacyTimestampBigintColumns() {
		if c.Table != "users" {
			continue
		}
		_, overlap := bigint[c.Name]
		assert.Falsef(t, overlap, "users.%s must not be handled by both the bigint and timestamp guards", c.Name)
	}
}

// TestLegacyTimestampBigintMigrationSQL_SkipsWhenNoMigrationNeeded proves the conversion only runs for
// an actual timestamp/date column: a missing column defers to AutoMigrate, an already-bigint column is
// never rewritten, and an integer column is left to AutoMigrate's own ::bigint cast (which already
// promotes integer->bigint without the SQLSTATE 42846 failure).
func TestLegacyTimestampBigintMigrationSQL_SkipsWhenNoMigrationNeeded(t *testing.T) {
	for _, column := range legacyTimestampBigintColumns() {
		assert.Nilf(t, legacyTimestampBigintMigrationSQL(column, "", false),
			"%s.%s: missing column is left to AutoMigrate", column.Table, column.Name)
		assert.Nilf(t, legacyTimestampBigintMigrationSQL(column, "bigint", true),
			"%s.%s: already-bigint column must not be rewritten", column.Table, column.Name)
		assert.Nilf(t, legacyTimestampBigintMigrationSQL(column, "integer", true),
			"%s.%s: integer column is left to AutoMigrate's ::bigint cast", column.Table, column.Name)
	}
}

// TestLegacyTimestampBigintMigrationSQL_ConvertsTimeTypesToEpoch verifies the exact statements emitted
// for every legacy time type (timestamptz, timestamp, date) on both default behaviors. A hook-written
// column (user_subscriptions.created_at, the production-log case) drops any legacy default, extracts
// Unix seconds with a NULL->0 guard, and restores no default. users.last_login_at is the only column
// that restores a default (0). Both quote their own table name.
func TestLegacyTimestampBigintMigrationSQL_ConvertsTimeTypesToEpoch(t *testing.T) {
	for _, dataType := range []string{"timestamp with time zone", "timestamp without time zone", "date"} {
		noDefault := legacyTimestampBigintMigrationSQL(
			legacyTimestampBigintColumn{Table: "user_subscriptions", Name: "created_at"}, dataType, true)
		assert.Equalf(t, []string{
			`ALTER TABLE "user_subscriptions" ALTER COLUMN "created_at" DROP DEFAULT`,
			`ALTER TABLE "user_subscriptions" ALTER COLUMN "created_at" TYPE bigint USING COALESCE(EXTRACT(EPOCH FROM "created_at")::bigint, 0)`,
		}, noDefault, "user_subscriptions.created_at/%s drops default, extracts epoch, and restores no default", dataType)

		login := legacyTimestampBigintMigrationSQL(
			legacyTimestampBigintColumn{Table: "users", Name: "last_login_at", RestoreDefault: "0"}, dataType, true)
		assert.Equalf(t, []string{
			`ALTER TABLE "users" ALTER COLUMN "last_login_at" DROP DEFAULT`,
			`ALTER TABLE "users" ALTER COLUMN "last_login_at" TYPE bigint USING COALESCE(EXTRACT(EPOCH FROM "last_login_at")::bigint, 0)`,
			`ALTER TABLE "users" ALTER COLUMN "last_login_at" SET DEFAULT 0`,
		}, login, "users.last_login_at/%s drops the legacy default, extracts epoch, and restores default 0", dataType)
	}
}

// TestLegacyTimestampBigintMigrationSQL_NeverRewritesData ensures every managed column converts via a
// value-preserving USING expression (never UPDATE/DELETE), drops its default first, quotes its own
// table and column, carries the NULL->0 epoch guard, and restores a SQL default only for
// users.last_login_at — across every legacy time type.
func TestLegacyTimestampBigintMigrationSQL_NeverRewritesData(t *testing.T) {
	for _, column := range legacyTimestampBigintColumns() {
		for _, dataType := range []string{"timestamp with time zone", "timestamp without time zone", "date"} {
			stmts := legacyTimestampBigintMigrationSQL(column, dataType, true)
			require.NotEmptyf(t, stmts, "%s.%s/%s must emit a conversion", column.Table, column.Name, dataType)

			tbl := `"` + column.Table + `"`
			col := `"` + column.Name + `"`
			assert.Equalf(t, `ALTER TABLE `+tbl+` ALTER COLUMN `+col+` DROP DEFAULT`, stmts[0],
				"%s.%s drops the default first", column.Table, column.Name)
			assert.Containsf(t, stmts[1], `ALTER TABLE `+tbl+` ALTER COLUMN `+col+` TYPE bigint USING `,
				"%s.%s changes type on its own quoted table/column", column.Table, column.Name)
			assert.Containsf(t, stmts[1], `COALESCE(EXTRACT(EPOCH FROM `+col+`)::bigint, 0)`,
				"%s.%s extracts Unix seconds with a NULL->0 guard", column.Table, column.Name)

			restoresDefault := false
			for _, stmt := range stmts {
				upper := strings.ToUpper(stmt)
				assert.NotContains(t, upper, "UPDATE ", "must not rewrite row values")
				assert.NotContains(t, upper, "DELETE ", "must not delete rows")
				if strings.Contains(upper, "SET DEFAULT") {
					restoresDefault = true
				}
			}

			if column.Table == "users" && column.Name == "last_login_at" {
				require.Lenf(t, stmts, 3, "users.last_login_at restores its default")
				assert.Equal(t, `ALTER TABLE "users" ALTER COLUMN "last_login_at" SET DEFAULT 0`, stmts[2])
				assert.True(t, restoresDefault, "users.last_login_at restores DEFAULT 0")
			} else {
				require.Lenf(t, stmts, 2, "%s.%s restores no default", column.Table, column.Name)
				assert.Falsef(t, restoresDefault, "%s.%s is hook/auto-written and must not SET DEFAULT", column.Table, column.Name)
			}
		}
	}
}

// TestEnsureLegacyTimestampBigintColumns_NonPostgresIsNoop confirms the pre-migration leaves SQLite
// (and by extension MySQL) completely untouched, including existing data, across multiple managed
// tables shaped the way a healthy install already stores them (bigint).
func TestEnsureLegacyTimestampBigintColumns_NonPostgresIsNoop(t *testing.T) {
	newLegacyTimestampMigrationDB(t) // configures SQLite

	require.NoError(t, DB.Exec("CREATE TABLE users (id integer PRIMARY KEY, created_at bigint, last_login_at bigint DEFAULT 0)").Error)
	require.NoError(t, DB.Exec("INSERT INTO users (id, created_at, last_login_at) VALUES (1, 1700000000, 1700000001)").Error)
	require.NoError(t, DB.Exec("CREATE TABLE user_subscriptions (id integer PRIMARY KEY, created_at bigint, updated_at bigint)").Error)
	require.NoError(t, DB.Exec("INSERT INTO user_subscriptions (id, created_at, updated_at) VALUES (1, 1700000002, 1700000003)").Error)

	require.NoError(t, ensureLegacyTimestampBigintColumns(), "must be a no-op on non-PostgreSQL")

	var createdAt, lastLoginAt int64
	require.NoError(t, DB.Raw("SELECT created_at, last_login_at FROM users WHERE id = ?", 1).Row().Scan(&createdAt, &lastLoginAt))
	assert.Equal(t, int64(1700000000), createdAt, "existing users.created_at must be preserved")
	assert.Equal(t, int64(1700000001), lastLoginAt, "existing users.last_login_at must be preserved")

	var subCreated, subUpdated int64
	require.NoError(t, DB.Raw("SELECT created_at, updated_at FROM user_subscriptions WHERE id = ?", 1).Row().Scan(&subCreated, &subUpdated))
	assert.Equal(t, int64(1700000002), subCreated, "existing user_subscriptions.created_at must be preserved")
	assert.Equal(t, int64(1700000003), subUpdated, "existing user_subscriptions.updated_at must be preserved")
}

// TestEnsureLegacyTimestampBigintColumns_NoTableIsNoop ensures a fresh install (none of the managed
// tables exist yet) is left entirely to AutoMigrate, even when PostgreSQL is the active database: the
// per-table HasTable check must skip every column before any PostgreSQL-only DDL runs against the
// SQLite test engine.
func TestEnsureLegacyTimestampBigintColumns_NoTableIsNoop(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypePostgreSQL)
	initCol()
	orig := DB
	DB = db
	t.Cleanup(func() {
		DB = orig
		common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
		initCol()
	})

	// No managed table exists, so the function must return before issuing any PostgreSQL DDL.
	require.NoError(t, ensureLegacyTimestampBigintColumns())
	for _, c := range legacyTimestampBigintColumns() {
		assert.Falsef(t, DB.Migrator().HasTable(c.Table), "%s must not be created by the guard", c.Table)
	}
}
