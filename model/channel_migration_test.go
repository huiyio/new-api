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
// A numeric source column keeps its verbatim ::bigint cast.
func TestChannelBigintMigrationSQL_BuildsCanonicalConversion(t *testing.T) {
	col := channelBigintColumn{Name: "status", Default: "1"}
	assert.Equal(t, []string{
		`ALTER TABLE channels ALTER COLUMN "status" DROP DEFAULT`,
		`ALTER TABLE channels ALTER COLUMN "status" TYPE bigint USING "status"::bigint`,
		`ALTER TABLE channels ALTER COLUMN "status" SET DEFAULT 1`,
	}, channelBigintMigrationSQL(col, "integer", true))
}

// TestChannelStatusBigintUsingExpr_MapsLegacyTokens locks the legacy-string-status mapping that
// fixes SQLSTATE 22P02 (status stored as text like 'active'). It asserts the documented token ->
// New API status mapping, case/whitespace insensitivity, the numeric passthrough, and the
// unknown(0) fallback, evaluating the generated CASE expression on a real SQLite engine so the SQL
// is verified rather than string-matched.
func TestChannelStatusBigintUsingExpr_MapsLegacyTokens(t *testing.T) {
	newChannelKeyMigrationDB(t) // in-memory SQLite

	// SQLite understands lower() but not Postgres btrim()/::text/~ regex; rewrite only those
	// dialect tokens so the CASE/WHEN logic and literal mapping under test stay intact.
	expr := channelStatusBigintUsingExpr()
	expr = strings.ReplaceAll(expr, `"status"::text`, `"status"`)
	expr = strings.ReplaceAll(expr, `::bigint`, "")
	expr = strings.ReplaceAll(expr, ` ~ '^[0-9]+$'`, ` GLOB '[0-9]*'`)
	expr = strings.ReplaceAll(expr, `btrim(`, `trim(`)

	require.NoError(t, DB.Exec(`CREATE TABLE channels (id integer PRIMARY KEY, status text)`).Error)

	cases := []struct {
		in   string
		want int64
	}{
		{"active", int64(common.ChannelStatusEnabled)},
		{"ENABLED", int64(common.ChannelStatusEnabled)},
		{" enable ", int64(common.ChannelStatusEnabled)},
		{"true", int64(common.ChannelStatusEnabled)},
		{"1", int64(common.ChannelStatusEnabled)},
		{"disabled", int64(common.ChannelStatusManuallyDisabled)},
		{"inactive", int64(common.ChannelStatusManuallyDisabled)},
		{"  Disable", int64(common.ChannelStatusManuallyDisabled)},
		{"false", int64(common.ChannelStatusManuallyDisabled)},
		{"0", int64(common.ChannelStatusManuallyDisabled)},
		{"2", int64(common.ChannelStatusManuallyDisabled)},
		{"manual_disabled", int64(common.ChannelStatusManuallyDisabled)},
		{"manually_disabled", int64(common.ChannelStatusManuallyDisabled)},
		{"manual-disabled", int64(common.ChannelStatusManuallyDisabled)},
		{"auto_disabled", int64(common.ChannelStatusAutoDisabled)},
		{"AUTO-DISABLED", int64(common.ChannelStatusAutoDisabled)},
		{"auto", int64(common.ChannelStatusAutoDisabled)},
		{"3", int64(common.ChannelStatusAutoDisabled)},
		{"42", int64(42)}, // genuine numeric string survives untouched
		{"unknown", int64(common.ChannelStatusUnknown)},
		{"", int64(common.ChannelStatusUnknown)},
		{"   ", int64(common.ChannelStatusUnknown)},
		{"garbage", int64(common.ChannelStatusUnknown)}, // unrecognized text -> unknown, never aborts
	}
	for i, tc := range cases {
		id := i + 1
		require.NoError(t, DB.Exec(`INSERT INTO channels (id, status) VALUES (?, ?)`, id, tc.in).Error)
		var got int64
		require.NoError(t, DB.Raw(`SELECT (`+expr+`) FROM channels WHERE id = ?`, id).Row().Scan(&got))
		assert.Equalf(t, tc.want, got, "status %q must map to %d", tc.in, tc.want)
	}
}

// TestChannelBigintMigrationSQL_StringStatusUsesMapping proves a string-typed status column no
// longer emits the bare "status"::bigint cast (which is exactly the SQLSTATE 22P02 trigger) and
// instead routes through the legacy-token CASE expression, while DROP/SET DEFAULT are unchanged.
func TestChannelBigintMigrationSQL_StringStatusUsesMapping(t *testing.T) {
	col := channelBigintColumn{Name: "status", Default: "1"}
	for _, dataType := range []string{"character varying", "text", "character", "varchar"} {
		stmts := channelBigintMigrationSQL(col, dataType, true)
		require.Lenf(t, stmts, 3, "data type %s", dataType)
		assert.Equal(t, `ALTER TABLE channels ALTER COLUMN "status" DROP DEFAULT`, stmts[0])
		assert.NotContains(t, stmts[1], `"status"::bigint`, "string status must not use a bare ::bigint cast (SQLSTATE 22P02)")
		assert.Contains(t, stmts[1], channelStatusBigintUsingExpr(), "string status must route through the legacy-token mapping")
		assert.Equal(t, `ALTER TABLE channels ALTER COLUMN "status" SET DEFAULT 1`, stmts[2])
	}
}

// TestChannelBigintMigrationSQL_StringNumericColumnsGuardBlanks confirms the non-status numeric
// columns, when found as a string type, trim and cast their value and fall back to the column
// default for blanks instead of emitting a bare ::bigint cast on raw text.
func TestChannelBigintMigrationSQL_StringNumericColumnsGuardBlanks(t *testing.T) {
	for _, column := range channelBigintColumns() {
		if column.Name == "status" {
			continue
		}
		stmts := channelBigintMigrationSQL(column, "character varying", true)
		require.Lenf(t, stmts, 3, "column %s", column.Name)
		quoted := `"` + column.Name + `"`
		assert.NotContains(t, stmts[1], quoted+`::bigint`, "string %s must not use a bare ::bigint cast", column.Name)
		assert.Contains(t, stmts[1], "THEN "+column.Default, "blank %s must fall back to its default", column.Name)
	}
}

// TestChannelBigintMigrationSQL_PreservesValuesForEveryColumn checks every managed column
// produces a value-preserving conversion (a USING expression, never an UPDATE/DELETE) and a
// restored default matching the struct tag, across both legacy source kinds: a numeric source
// (integer, the SQLSTATE 42804 case) keeps the verbatim ::bigint cast, while a string source
// (character varying, the SQLSTATE 22P02 case) routes through a normalizing USING expression.
func TestChannelBigintMigrationSQL_PreservesValuesForEveryColumn(t *testing.T) {
	for _, column := range channelBigintColumns() {
		quoted := `"` + column.Name + `"`

		intStmts := channelBigintMigrationSQL(column, "integer", true)
		require.Len(t, intStmts, 3, "column %s (integer)", column.Name)
		assert.Equal(t, `ALTER TABLE channels ALTER COLUMN `+quoted+` DROP DEFAULT`, intStmts[0])
		assert.Equal(t, `ALTER TABLE channels ALTER COLUMN `+quoted+` TYPE bigint USING `+quoted+`::bigint`, intStmts[1])
		assert.Equal(t, `ALTER TABLE channels ALTER COLUMN `+quoted+` SET DEFAULT `+column.Default, intStmts[2])

		strStmts := channelBigintMigrationSQL(column, "character varying", true)
		require.Len(t, strStmts, 3, "column %s (character varying)", column.Name)
		assert.Equal(t, `ALTER TABLE channels ALTER COLUMN `+quoted+` DROP DEFAULT`, strStmts[0])
		assert.Contains(t, strStmts[1], `ALTER TABLE channels ALTER COLUMN `+quoted+` TYPE bigint USING `)
		assert.Equal(t, `ALTER TABLE channels ALTER COLUMN `+quoted+` SET DEFAULT `+column.Default, strStmts[2])

		for _, stmts := range [][]string{intStmts, strStmts} {
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
