package model

import (
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
