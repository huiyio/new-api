package model

import (
	"database/sql"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newUserMigrationDB spins up an isolated in-memory SQLite database and points the package-global
// DB at it for the duration of the test, mirroring newChannelKeyMigrationDB so the users.username
// de-duplication can be exercised against a legacy table without disturbing the shared database.
func newUserMigrationDB(t *testing.T) {
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

// migratedUserRow captures a users row (including soft-deleted ones) with a NULL-aware username so
// a test can assert the exact post-migration state.
type migratedUserRow struct {
	Id       int
	Username sql.NullString
}

func loadUserRows(t *testing.T) []migratedUserRow {
	t.Helper()
	var rows []migratedUserRow
	require.NoError(t, DB.Raw("SELECT id, username FROM users ORDER BY id ASC").Scan(&rows).Error)
	return rows
}

func TestEnsureUserUsernameUnique_NoTableIsNoop(t *testing.T) {
	newUserMigrationDB(t)

	require.NoError(t, ensureUserUsernameUnique())
	// Function must not create the table; AutoMigrate is responsible for that.
	assert.False(t, DB.Migrator().HasTable("users"))
}

func TestEnsureUserUsernameUnique_NoDuplicatesUnchanged(t *testing.T) {
	newUserMigrationDB(t)

	require.NoError(t, DB.Exec("CREATE TABLE users (id integer PRIMARY KEY, username text)").Error)
	require.NoError(t, DB.Exec("INSERT INTO users (id, username) VALUES (1, 'root'), (2, 'alice'), (3, 'bob')").Error)

	require.NoError(t, ensureUserUsernameUnique())

	rows := loadUserRows(t)
	require.Len(t, rows, 3)
	assert.Equal(t, "root", rows[0].Username.String)
	assert.Equal(t, "alice", rows[1].Username.String)
	assert.Equal(t, "bob", rows[2].Username.String)
	// The whole point of the guard: a unique index now builds without SQLSTATE 23505.
	require.NoError(t, DB.Exec("CREATE UNIQUE INDEX idx_users_username ON users(username)").Error)
}

func TestEnsureUserUsernameUnique_RenamesLaterDuplicates(t *testing.T) {
	newUserMigrationDB(t)

	require.NoError(t, DB.Exec("CREATE TABLE users (id integer PRIMARY KEY, username text, deleted_at datetime)").Error)
	// id=1 is the earliest and must keep "alice"; id=2 (active) and id=3 (soft-deleted) are later
	// duplicates and must be renamed. The soft-deleted row matters because the unique index spans
	// it, so a GORM model query that hid it would leave the collision in place.
	require.NoError(t, DB.Exec(
		"INSERT INTO users (id, username, deleted_at) VALUES (1, 'alice', NULL), (2, 'alice', NULL), (3, 'alice', '2024-01-01 00:00:00')").Error)

	require.NoError(t, ensureUserUsernameUnique())

	rows := loadUserRows(t)
	require.Len(t, rows, 3)
	assert.Equal(t, "alice", rows[0].Username.String, "earliest id keeps the original username")
	assert.Equal(t, "alice_2", rows[1].Username.String, "later active duplicate is renamed deterministically")
	assert.Equal(t, "alice_3", rows[2].Username.String, "soft-deleted duplicate is renamed too")
	require.NoError(t, DB.Exec("CREATE UNIQUE INDEX idx_users_username ON users(username)").Error)
}

func TestEnsureUserUsernameUnique_CandidateCollisionStaysUnique(t *testing.T) {
	newUserMigrationDB(t)

	require.NoError(t, DB.Exec("CREATE TABLE users (id integer PRIMARY KEY, username text)").Error)
	// The natural rename for the id=6 duplicate is "alice_6", but that name already belongs to
	// id=1, so the guard must fall through to a still-unique value instead of recreating a clash.
	require.NoError(t, DB.Exec("INSERT INTO users (id, username) VALUES (1, 'alice_6'), (5, 'alice'), (6, 'alice')").Error)

	require.NoError(t, ensureUserUsernameUnique())

	rows := loadUserRows(t)
	require.Len(t, rows, 3)
	byID := make(map[int]string, len(rows))
	for _, r := range rows {
		byID[r.Id] = r.Username.String
	}
	assert.Equal(t, "alice_6", byID[1], "pre-existing account keeps its name")
	assert.Equal(t, "alice", byID[5], "earliest duplicate keeps the original")
	assert.Equal(t, "alice_6_1", byID[6], "collision with existing alice_6 falls through to a unique suffix")
	require.NoError(t, DB.Exec("CREATE UNIQUE INDEX idx_users_username ON users(username)").Error)
}

func TestEnsureUserUsernameUnique_EmptyAndNullHandling(t *testing.T) {
	newUserMigrationDB(t)

	require.NoError(t, DB.Exec("CREATE TABLE users (id integer PRIMARY KEY, username text)").Error)
	// Two empty-string usernames collide and must be de-duplicated; two NULLs are distinct in a
	// unique index and must be left exactly as-is.
	require.NoError(t, DB.Exec("INSERT INTO users (id, username) VALUES (1, ''), (2, ''), (3, NULL), (4, NULL)").Error)

	require.NoError(t, ensureUserUsernameUnique())

	rows := loadUserRows(t)
	require.Len(t, rows, 4)
	byID := make(map[int]sql.NullString, len(rows))
	for _, r := range rows {
		byID[r.Id] = r.Username
	}
	assert.Equal(t, sql.NullString{String: "", Valid: true}, byID[1], "earliest empty username is preserved")
	assert.Equal(t, sql.NullString{String: "migration_user_2", Valid: true}, byID[2], "later empty username uses the migration_user_<id> form")
	assert.False(t, byID[3].Valid, "NULL usernames are left untouched")
	assert.False(t, byID[4].Valid, "multiple NULLs never block the unique index")
	require.NoError(t, DB.Exec("CREATE UNIQUE INDEX idx_users_username ON users(username)").Error)
}
