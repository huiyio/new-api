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

// newChannelKeyMigrationDB spins up an isolated in-memory SQLite database and points the
// package-global DB at it for the duration of the test, so each scenario can model a legacy
// channels table without disturbing the shared test database.
func newChannelKeyMigrationDB(t *testing.T) {
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

func TestEnsureChannelKeyColumn_NoTableIsNoop(t *testing.T) {
	newChannelKeyMigrationDB(t)

	require.NoError(t, ensureChannelKeyColumn())
	// Function must not create the table; AutoMigrate is responsible for that.
	assert.False(t, DB.Migrator().HasTable("channels"))
}

func TestEnsureChannelKeyColumn_AddsMissingColumn(t *testing.T) {
	newChannelKeyMigrationDB(t)

	require.NoError(t, DB.Exec("CREATE TABLE channels (id integer PRIMARY KEY, name text)").Error)
	require.NoError(t, DB.Exec("INSERT INTO channels (id, name) VALUES (1, 'c1')").Error)

	require.NoError(t, ensureChannelKeyColumn())

	assert.True(t, DB.Migrator().HasColumn(&Channel{}, "key"))

	var name, key string
	require.NoError(t, DB.Raw("SELECT name, `key` FROM channels WHERE id = ?", 1).Row().Scan(&name, &key))
	assert.Equal(t, "c1", name, "existing data must be preserved")
	assert.Equal(t, "", key, "new key column must be backfilled with empty string")
}

func TestEnsureChannelKeyColumn_BackfillsNullKeys(t *testing.T) {
	newChannelKeyMigrationDB(t)

	require.NoError(t, DB.Exec("CREATE TABLE channels (id integer PRIMARY KEY, `key` text, name text)").Error)
	require.NoError(t, DB.Exec("INSERT INTO channels (id, `key`, name) VALUES (1, NULL, 'c1')").Error)
	require.NoError(t, DB.Exec("INSERT INTO channels (id, `key`, name) VALUES (2, 'real-key', 'c2')").Error)

	require.NoError(t, ensureChannelKeyColumn())

	var nullKey, realKey string
	require.NoError(t, DB.Raw("SELECT `key` FROM channels WHERE id = ?", 1).Row().Scan(&nullKey))
	require.NoError(t, DB.Raw("SELECT `key` FROM channels WHERE id = ?", 2).Row().Scan(&realKey))
	assert.Equal(t, "", nullKey, "NULL key must become empty string")
	assert.Equal(t, "real-key", realKey, "existing non-NULL key must be untouched")

	var nullCount int64
	require.NoError(t, DB.Raw("SELECT COUNT(*) FROM channels WHERE `key` IS NULL").Row().Scan(&nullCount))
	assert.Equal(t, int64(0), nullCount)
}

func TestEnsureChannelKeyColumn_RepeatablePreservesKeys(t *testing.T) {
	newChannelKeyMigrationDB(t)

	require.NoError(t, DB.Exec("CREATE TABLE channels (id integer PRIMARY KEY, name text)").Error)
	require.NoError(t, DB.Exec("INSERT INTO channels (id, name) VALUES (1, 'c1')").Error)

	// First run adds the column and backfills.
	require.NoError(t, ensureChannelKeyColumn())
	require.NoError(t, DB.Exec("UPDATE channels SET `key` = 'sk-existing' WHERE id = ?", 1).Error)

	// Subsequent runs must be idempotent and must not clobber a real key.
	require.NoError(t, ensureChannelKeyColumn())
	require.NoError(t, ensureChannelKeyColumn())

	var key string
	require.NoError(t, DB.Raw("SELECT `key` FROM channels WHERE id = ?", 1).Row().Scan(&key))
	assert.Equal(t, "sk-existing", key)
}

// TestChannelBigintColumns_CoversDefaultedIntColumns locks the set of channels columns the
// bigint pre-migration touches. These must be exactly the Channel int-family fields that carry
// a GORM default (Type/Status/Weight/AutoBan); the others are already bigint or have no default.
func TestChannelBigintColumns_CoversDefaultedIntColumns(t *testing.T) {
	got := make(map[string]string)
	for _, c := range channelBigintColumns() {
		got[c.Name] = c.Default
	}
	assert.Equal(t, map[string]string{
		"status":   "1",
		"type":     "0",
		"weight":   "0",
		"auto_ban": "1",
	}, got)
}

// TestChannelBigintMigrationSQL_SkipsWhenNoMigrationNeeded proves the conversion is idempotent
// and only runs when required: a missing column is deferred to AutoMigrate, and a column that
// is already bigint must not be rewritten again on every startup.
func TestChannelBigintMigrationSQL_SkipsWhenNoMigrationNeeded(t *testing.T) {
	col := channelBigintColumn{Name: "status", Default: "1"}
	assert.Nil(t, channelBigintMigrationSQL(col, "", false), "missing column is left to AutoMigrate")
	assert.Nil(t, channelBigintMigrationSQL(col, "bigint", true), "already-bigint column must not be rewritten")
}

// TestChannelBigintMigrationSQL_BuildsCanonicalConversion verifies the exact statements emitted
// for the column from the production log (status, currently integer), including the ordering that
// unblocks SQLSTATE 42804: DROP DEFAULT must precede the type change, then the default is restored.
func TestChannelBigintMigrationSQL_BuildsCanonicalConversion(t *testing.T) {
	col := channelBigintColumn{Name: "status", Default: "1"}
	assert.Equal(t, []string{
		`ALTER TABLE channels ALTER COLUMN "status" DROP DEFAULT`,
		`ALTER TABLE channels ALTER COLUMN "status" TYPE bigint USING "status"::bigint`,
		`ALTER TABLE channels ALTER COLUMN "status" SET DEFAULT 1`,
	}, channelBigintMigrationSQL(col, "integer", true))
}

// TestChannelBigintMigrationSQL_PreservesValuesForEveryColumn checks every managed column,
// across the legacy types that trigger the bug (integer and varchar), produces a value-preserving
// conversion: a USING cast (never an UPDATE/DELETE) and a restored default matching the struct tag.
func TestChannelBigintMigrationSQL_PreservesValuesForEveryColumn(t *testing.T) {
	for _, column := range channelBigintColumns() {
		for _, dataType := range []string{"integer", "character varying"} {
			stmts := channelBigintMigrationSQL(column, dataType, true)
			require.Len(t, stmts, 3, "column %s (%s)", column.Name, dataType)

			quoted := `"` + column.Name + `"`
			assert.Equal(t, `ALTER TABLE channels ALTER COLUMN `+quoted+` DROP DEFAULT`, stmts[0])
			assert.Equal(t, `ALTER TABLE channels ALTER COLUMN `+quoted+` TYPE bigint USING `+quoted+`::bigint`, stmts[1])
			assert.Equal(t, `ALTER TABLE channels ALTER COLUMN `+quoted+` SET DEFAULT `+column.Default, stmts[2])

			for _, stmt := range stmts {
				upper := strings.ToUpper(stmt)
				assert.NotContains(t, upper, "UPDATE ", "must not rewrite row values")
				assert.NotContains(t, upper, "DELETE ", "must not delete rows")
			}
		}
	}
}

// TestEnsureChannelBigintColumns_NonPostgresIsNoop confirms the pre-migration leaves SQLite
// (and by extension MySQL, which widens defaults implicitly) completely untouched, including
// existing column data.
func TestEnsureChannelBigintColumns_NonPostgresIsNoop(t *testing.T) {
	newChannelKeyMigrationDB(t) // configures SQLite

	require.NoError(t, DB.Exec("CREATE TABLE channels (id integer PRIMARY KEY, status integer DEFAULT 1)").Error)
	require.NoError(t, DB.Exec("INSERT INTO channels (id, status) VALUES (1, 2)").Error)

	require.NoError(t, ensureChannelBigintColumns(), "must be a no-op on non-PostgreSQL")

	var status int64
	require.NoError(t, DB.Raw("SELECT status FROM channels WHERE id = ?", 1).Row().Scan(&status))
	assert.Equal(t, int64(2), status, "existing values must be preserved")
}

// TestEnsureChannelBigintColumns_NoTableIsNoop ensures a fresh install (no channels table yet)
// is left entirely to AutoMigrate, even when PostgreSQL is the active database.
func TestEnsureChannelBigintColumns_NoTableIsNoop(t *testing.T) {
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

	// channels table is absent, so the function must return before issuing any PostgreSQL DDL.
	require.NoError(t, ensureChannelBigintColumns())
	assert.False(t, DB.Migrator().HasTable("channels"))
}
