package model

import (
	"database/sql"
	"strings"
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

// migratedPasswordRow captures a users row with a NULL-aware password so a test can assert the
// exact post-migration state, including whether a backfilled value is empty or a preserved hash.
type migratedPasswordRow struct {
	Id       int
	Password sql.NullString
}

func loadUserPasswordRows(t *testing.T) []migratedPasswordRow {
	t.Helper()
	var rows []migratedPasswordRow
	require.NoError(t, DB.Raw("SELECT id, password FROM users ORDER BY id ASC").Scan(&rows).Error)
	return rows
}

func TestEnsureUserPasswordColumn_NoTableIsNoop(t *testing.T) {
	newUserMigrationDB(t)

	require.NoError(t, ensureUserPasswordColumn())
	// Function must not create the table; AutoMigrate is responsible for that.
	assert.False(t, DB.Migrator().HasTable("users"))
}

func TestEnsureUserPasswordColumn_AddsMissingColumnAndBackfills(t *testing.T) {
	newUserMigrationDB(t)

	// Legacy table without a password column at all.
	require.NoError(t, DB.Exec("CREATE TABLE users (id integer PRIMARY KEY, username text)").Error)
	require.NoError(t, DB.Exec("INSERT INTO users (id, username) VALUES (1, 'root'), (2, 'alice')").Error)

	require.NoError(t, ensureUserPasswordColumn())

	require.True(t, DB.Migrator().HasColumn(&User{}, "password"))
	rows := loadUserPasswordRows(t)
	require.Len(t, rows, 2)
	for _, r := range rows {
		assert.Equal(t, sql.NullString{String: "", Valid: true}, r.Password, "new column is backfilled with empty strings, never NULL")
	}
}

func TestEnsureUserPasswordColumn_BackfillsNullsAndKeepsHashes(t *testing.T) {
	newUserMigrationDB(t)

	// Column exists but some rows hold NULL; existing hashes must survive untouched.
	require.NoError(t, DB.Exec("CREATE TABLE users (id integer PRIMARY KEY, username text, password text)").Error)
	require.NoError(t, DB.Exec(
		"INSERT INTO users (id, username, password) VALUES (1, 'root', '$2a$hashedroot'), (2, 'alice', NULL)").Error)

	require.NoError(t, ensureUserPasswordColumn())

	rows := loadUserPasswordRows(t)
	require.Len(t, rows, 2)
	byID := make(map[int]sql.NullString, len(rows))
	for _, r := range rows {
		byID[r.Id] = r.Password
	}
	assert.Equal(t, sql.NullString{String: "$2a$hashedroot", Valid: true}, byID[1], "existing password hash is preserved")
	assert.Equal(t, sql.NullString{String: "", Valid: true}, byID[2], "NULL password is backfilled with empty string")
}

func TestEnsureUserPasswordColumn_RepeatableRun(t *testing.T) {
	newUserMigrationDB(t)

	require.NoError(t, DB.Exec("CREATE TABLE users (id integer PRIMARY KEY, username text)").Error)
	require.NoError(t, DB.Exec("INSERT INTO users (id, username) VALUES (1, 'root')").Error)
	require.NoError(t, DB.Exec("UPDATE users SET id = id").Error) // touch to ensure table is writable

	// First run adds the column; a second run must be a stable no-op without errors.
	require.NoError(t, ensureUserPasswordColumn())
	require.NoError(t, DB.Exec("UPDATE users SET password = '$2a$realhash' WHERE id = 1").Error)
	require.NoError(t, ensureUserPasswordColumn())

	rows := loadUserPasswordRows(t)
	require.Len(t, rows, 1)
	assert.Equal(t, sql.NullString{String: "$2a$realhash", Valid: true}, rows[0].Password, "repeated run does not clobber a real hash")
}

// TestUserBigintColumns_CoversDefaultedIntColumns locks the set of users columns the bigint
// pre-migration touches. These must be exactly the User int-family fields that carry a GORM default
// and map to bigint; InviterId (no default) and the int64 timestamp columns are intentionally absent.
func TestUserBigintColumns_CoversDefaultedIntColumns(t *testing.T) {
	got := make(map[string]string)
	for _, c := range userBigintColumns() {
		got[c.Name] = c.Default
	}
	assert.Equal(t, map[string]string{
		"role":          "1",
		"status":        "1",
		"quota":         "0",
		"used_quota":    "0",
		"request_count": "0",
		"aff_count":     "0",
		"aff_quota":     "0",
		"aff_history":   "0",
	}, got)
}

// TestUserBigintMigrationSQL_SkipsWhenNoMigrationNeeded proves the conversion is idempotent and only
// runs when required: a missing column is deferred to AutoMigrate, and a column that is already
// bigint must not be rewritten again on every startup.
func TestUserBigintMigrationSQL_SkipsWhenNoMigrationNeeded(t *testing.T) {
	col := userBigintColumn{Name: "role", Default: "1"}
	assert.Nil(t, userBigintMigrationSQL(col, "", false), "missing column is left to AutoMigrate")
	assert.Nil(t, userBigintMigrationSQL(col, "bigint", true), "already-bigint column must not be rewritten")
}

// TestUserBigintMigrationSQL_BuildsCanonicalConversion verifies the exact statements emitted for the
// column from the production log (role, currently integer), including the ordering that unblocks
// SQLSTATE 42804: DROP DEFAULT must precede the type change, then the default is restored. A numeric
// source column keeps its verbatim ::bigint cast.
func TestUserBigintMigrationSQL_BuildsCanonicalConversion(t *testing.T) {
	col := userBigintColumn{Name: "role", Default: "1"}
	assert.Equal(t, []string{
		`ALTER TABLE users ALTER COLUMN "role" DROP DEFAULT`,
		`ALTER TABLE users ALTER COLUMN "role" TYPE bigint USING "role"::bigint`,
		`ALTER TABLE users ALTER COLUMN "role" SET DEFAULT 1`,
	}, userBigintMigrationSQL(col, "integer", true))
}

// sqliteRewriteUsingExpr rewrites a PostgreSQL USING expression so its CASE/WHEN logic and literal
// mapping can be evaluated on SQLite, mirroring the channel-status test: only the Postgres-only
// dialect tokens are swapped, the mapping under test stays intact.
func sqliteRewriteUsingExpr(expr, column string) string {
	expr = strings.ReplaceAll(expr, `"`+column+`"::text`, `"`+column+`"`)
	expr = strings.ReplaceAll(expr, `::bigint`, "")
	expr = strings.ReplaceAll(expr, ` ~ '^[0-9]+$'`, ` GLOB '[0-9]*'`)
	expr = strings.ReplaceAll(expr, `btrim(`, `trim(`)
	return expr
}

// TestUserRoleBigintUsingExpr_MapsLegacyTokens locks the legacy-string-role mapping that fixes
// SQLSTATE 22P02 (role stored as text like 'admin'). It asserts the documented token -> New API role
// mapping, case/whitespace insensitivity, the numeric passthrough, and the least-privilege
// RoleGuestUser fallback, evaluating the generated CASE expression on a real SQLite engine.
func TestUserRoleBigintUsingExpr_MapsLegacyTokens(t *testing.T) {
	newUserMigrationDB(t) // in-memory SQLite

	expr := sqliteRewriteUsingExpr(userRoleBigintUsingExpr(), "role")
	require.NoError(t, DB.Exec(`CREATE TABLE users (id integer PRIMARY KEY, role text)`).Error)

	cases := []struct {
		in   string
		want int64
	}{
		{"root", int64(common.RoleRootUser)},
		{"ROOT", int64(common.RoleRootUser)},
		{" super_admin ", int64(common.RoleRootUser)},
		{"admin", int64(common.RoleAdminUser)},
		{"administrator", int64(common.RoleAdminUser)},
		{"common", int64(common.RoleCommonUser)},
		{"user", int64(common.RoleCommonUser)},
		{"member", int64(common.RoleCommonUser)},
		{"guest", int64(common.RoleGuestUser)},
		{"anonymous", int64(common.RoleGuestUser)},
		{"100", int64(common.RoleRootUser)},      // genuine numeric role survives untouched
		{"10", int64(common.RoleAdminUser)},      // ditto
		{"1", int64(common.RoleCommonUser)},      // ditto
		{"0", int64(common.RoleGuestUser)},       // ditto
		{"42", int64(42)},                        // arbitrary numeric string preserved
		{"", int64(common.RoleGuestUser)},        // empty -> least privilege, never aborts
		{"   ", int64(common.RoleGuestUser)},     // blank -> least privilege
		{"garbage", int64(common.RoleGuestUser)}, // unrecognized text -> least privilege
	}
	for i, tc := range cases {
		id := i + 1
		require.NoError(t, DB.Exec(`INSERT INTO users (id, role) VALUES (?, ?)`, id, tc.in).Error)
		var got int64
		require.NoError(t, DB.Raw(`SELECT (`+expr+`) FROM users WHERE id = ?`, id).Row().Scan(&got))
		assert.Equalf(t, tc.want, got, "role %q must map to %d", tc.in, tc.want)
	}
}

// TestUserStatusBigintUsingExpr_MapsLegacyTokens locks the legacy-string-status mapping. Textual
// tokens map to enabled/disabled, pure-integer strings pass through verbatim (matching the integer
// column path, so 0/1/2 are preserved), and unrecognized text collapses to the fail-closed
// UserStatusDisabled. The CASE expression is evaluated on a real SQLite engine.
func TestUserStatusBigintUsingExpr_MapsLegacyTokens(t *testing.T) {
	newUserMigrationDB(t) // in-memory SQLite

	expr := sqliteRewriteUsingExpr(userStatusBigintUsingExpr(), "status")
	require.NoError(t, DB.Exec(`CREATE TABLE users (id integer PRIMARY KEY, status text)`).Error)

	cases := []struct {
		in   string
		want int64
	}{
		{"enabled", int64(common.UserStatusEnabled)},
		{"ENABLED", int64(common.UserStatusEnabled)},
		{" active ", int64(common.UserStatusEnabled)},
		{"on", int64(common.UserStatusEnabled)},
		{"true", int64(common.UserStatusEnabled)},
		{"yes", int64(common.UserStatusEnabled)},
		{"disabled", int64(common.UserStatusDisabled)},
		{"inactive", int64(common.UserStatusDisabled)},
		{"  Disable", int64(common.UserStatusDisabled)},
		{"false", int64(common.UserStatusDisabled)},
		{"banned", int64(common.UserStatusDisabled)},
		{"blocked", int64(common.UserStatusDisabled)},
		{"1", int64(common.UserStatusEnabled)},  // genuine numeric status preserved
		{"2", int64(common.UserStatusDisabled)}, // ditto
		{"0", int64(0)},                         // sentinel preserved, matching the integer path
		{"42", int64(42)},                       // arbitrary numeric string preserved
		{"", int64(common.UserStatusDisabled)},  // empty -> fail-closed, never aborts
		{"   ", int64(common.UserStatusDisabled)},
		{"garbage", int64(common.UserStatusDisabled)}, // unrecognized text -> fail-closed
	}
	for i, tc := range cases {
		id := i + 1
		require.NoError(t, DB.Exec(`INSERT INTO users (id, status) VALUES (?, ?)`, id, tc.in).Error)
		var got int64
		require.NoError(t, DB.Raw(`SELECT (`+expr+`) FROM users WHERE id = ?`, id).Row().Scan(&got))
		assert.Equalf(t, tc.want, got, "status %q must map to %d", tc.in, tc.want)
	}
}

// TestUserBigintMigrationSQL_StringRoleAndStatusUseMapping proves a string-typed role or status
// column no longer emits the bare ::bigint cast (the SQLSTATE 22P02 trigger) and instead routes
// through its legacy-token CASE expression, while DROP/SET DEFAULT are unchanged.
func TestUserBigintMigrationSQL_StringRoleAndStatusUseMapping(t *testing.T) {
	cases := []struct {
		column userBigintColumn
		expr   string
	}{
		{userBigintColumn{Name: "role", Default: "1"}, userRoleBigintUsingExpr()},
		{userBigintColumn{Name: "status", Default: "1"}, userStatusBigintUsingExpr()},
	}
	for _, c := range cases {
		quoted := `"` + c.column.Name + `"`
		for _, dataType := range []string{"character varying", "text", "character", "varchar"} {
			stmts := userBigintMigrationSQL(c.column, dataType, true)
			require.Lenf(t, stmts, 3, "%s/%s", c.column.Name, dataType)
			assert.Equal(t, `ALTER TABLE users ALTER COLUMN `+quoted+` DROP DEFAULT`, stmts[0])
			assert.NotContainsf(t, stmts[1], quoted+`::bigint`, "string %s must not use a bare ::bigint cast (SQLSTATE 22P02)", c.column.Name)
			assert.Containsf(t, stmts[1], c.expr, "string %s must route through the legacy-token mapping", c.column.Name)
			assert.Equal(t, `ALTER TABLE users ALTER COLUMN `+quoted+` SET DEFAULT `+c.column.Default, stmts[2])
		}
	}
}

// TestUserBigintMigrationSQL_StringNumericColumnsGuardBlanks confirms the quota-family columns, when
// found as a string type, trim and cast their value and fall back to the column default for blanks
// instead of emitting a bare ::bigint cast on raw text.
func TestUserBigintMigrationSQL_StringNumericColumnsGuardBlanks(t *testing.T) {
	for _, column := range userBigintColumns() {
		if column.Name == "role" || column.Name == "status" {
			continue
		}
		stmts := userBigintMigrationSQL(column, "character varying", true)
		require.Lenf(t, stmts, 3, "column %s", column.Name)
		quoted := `"` + column.Name + `"`
		assert.NotContainsf(t, stmts[1], quoted+`::bigint`, "string %s must not use a bare ::bigint cast", column.Name)
		assert.Containsf(t, stmts[1], "THEN "+column.Default, "blank %s must fall back to its default", column.Name)
	}
}

// TestUserBigintMigrationSQL_PreservesValuesForEveryColumn checks every managed column produces a
// value-preserving conversion (a USING expression, never an UPDATE/DELETE) and a restored default
// matching the struct tag, across both legacy source kinds: a numeric source (integer, the SQLSTATE
// 42804 case) keeps the verbatim ::bigint cast, while a string source (character varying, the
// SQLSTATE 22P02 case) routes through a normalizing USING expression.
func TestUserBigintMigrationSQL_PreservesValuesForEveryColumn(t *testing.T) {
	for _, column := range userBigintColumns() {
		quoted := `"` + column.Name + `"`

		intStmts := userBigintMigrationSQL(column, "integer", true)
		require.Len(t, intStmts, 3, "column %s (integer)", column.Name)
		assert.Equal(t, `ALTER TABLE users ALTER COLUMN `+quoted+` DROP DEFAULT`, intStmts[0])
		assert.Equal(t, `ALTER TABLE users ALTER COLUMN `+quoted+` TYPE bigint USING `+quoted+`::bigint`, intStmts[1])
		assert.Equal(t, `ALTER TABLE users ALTER COLUMN `+quoted+` SET DEFAULT `+column.Default, intStmts[2])

		strStmts := userBigintMigrationSQL(column, "character varying", true)
		require.Len(t, strStmts, 3, "column %s (character varying)", column.Name)
		assert.Equal(t, `ALTER TABLE users ALTER COLUMN `+quoted+` DROP DEFAULT`, strStmts[0])
		assert.Contains(t, strStmts[1], `ALTER TABLE users ALTER COLUMN `+quoted+` TYPE bigint USING `)
		assert.Equal(t, `ALTER TABLE users ALTER COLUMN `+quoted+` SET DEFAULT `+column.Default, strStmts[2])

		for _, stmts := range [][]string{intStmts, strStmts} {
			for _, stmt := range stmts {
				upper := strings.ToUpper(stmt)
				assert.NotContains(t, upper, "UPDATE ", "must not rewrite row values")
				assert.NotContains(t, upper, "DELETE ", "must not delete rows")
			}
		}
	}
}

// TestEnsureUserBigintColumns_NonPostgresIsNoop confirms the pre-migration leaves SQLite (and by
// extension MySQL, which widens defaults implicitly) completely untouched, including existing data.
func TestEnsureUserBigintColumns_NonPostgresIsNoop(t *testing.T) {
	newUserMigrationDB(t) // configures SQLite

	require.NoError(t, DB.Exec("CREATE TABLE users (id integer PRIMARY KEY, role integer DEFAULT 1, status integer DEFAULT 1)").Error)
	require.NoError(t, DB.Exec("INSERT INTO users (id, role, status) VALUES (1, 10, 2)").Error)

	require.NoError(t, ensureUserBigintColumns(), "must be a no-op on non-PostgreSQL")

	var role, status int64
	require.NoError(t, DB.Raw("SELECT role, status FROM users WHERE id = ?", 1).Row().Scan(&role, &status))
	assert.Equal(t, int64(10), role, "existing role must be preserved")
	assert.Equal(t, int64(2), status, "existing status must be preserved")
}

// TestEnsureUserBigintColumns_NoTableIsNoop ensures a fresh install (no users table yet) is left
// entirely to AutoMigrate, even when PostgreSQL is the active database.
func TestEnsureUserBigintColumns_NoTableIsNoop(t *testing.T) {
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

	// users table is absent, so the function must return before issuing any PostgreSQL DDL.
	require.NoError(t, ensureUserBigintColumns())
	assert.False(t, DB.Migrator().HasTable("users"))
}

// TestUserTimestampColumns_CoversInt64TimeFields locks the set and per-column default behavior of the
// timestamp pre-migration: created_at (autoCreateTime, so no SQL default to restore) and
// last_login_at (gorm:"default:0", so DEFAULT 0 is restored). These are exactly the int64 Unix-second
// User fields a legacy PostgreSQL table may store as a timestamp/date, and they must never overlap
// with userBigintColumns.
func TestUserTimestampColumns_CoversInt64TimeFields(t *testing.T) {
	got := make(map[string]string)
	for _, c := range userTimestampColumns() {
		got[c.Name] = c.RestoreDefault
	}
	assert.Equal(t, map[string]string{
		"created_at":    "",
		"last_login_at": "0",
	}, got)

	// The two guards must touch disjoint columns: the verbatim ::bigint cast in userBigintColumns
	// cannot convert a timestamp, and EXTRACT(EPOCH ...) must not run on a plain integer column.
	bigint := make(map[string]struct{})
	for _, c := range userBigintColumns() {
		bigint[c.Name] = struct{}{}
	}
	for _, c := range userTimestampColumns() {
		_, overlap := bigint[c.Name]
		assert.Falsef(t, overlap, "%s must not be handled by both bigint and timestamp guards", c.Name)
	}
}

// TestUserTimestampMigrationSQL_SkipsWhenNoMigrationNeeded proves the conversion only runs for an
// actual timestamp/date column: a missing column defers to AutoMigrate, an already-bigint column is
// never rewritten, and an integer column is left to AutoMigrate's own ::bigint cast (which already
// promotes integer->bigint without the SQLSTATE 42846 failure).
func TestUserTimestampMigrationSQL_SkipsWhenNoMigrationNeeded(t *testing.T) {
	for _, column := range userTimestampColumns() {
		assert.Nilf(t, userTimestampMigrationSQL(column, "", false), "%s: missing column is left to AutoMigrate", column.Name)
		assert.Nilf(t, userTimestampMigrationSQL(column, "bigint", true), "%s: already-bigint column must not be rewritten", column.Name)
		assert.Nilf(t, userTimestampMigrationSQL(column, "integer", true), "%s: integer column is left to AutoMigrate's ::bigint cast", column.Name)
	}
}

// TestUserTimestampMigrationSQL_ConvertsTimeTypesToEpoch verifies the exact statements emitted for
// every legacy time type (timestamptz, timestamp, date). The conversion drops any legacy default
// first (e.g. now()) so the type change does not re-coerce it (SQLSTATE 42846), extracts Unix seconds
// with a NULL->0 guard, and restores a default only for last_login_at (0, matching gorm:"default:0");
// created_at restores none, matching its autoCreateTime (no SQL default).
func TestUserTimestampMigrationSQL_ConvertsTimeTypesToEpoch(t *testing.T) {
	for _, dataType := range []string{"timestamp with time zone", "timestamp without time zone", "date"} {
		created := userTimestampMigrationSQL(userTimestampColumn{Name: "created_at", RestoreDefault: ""}, dataType, true)
		assert.Equalf(t, []string{
			`ALTER TABLE users ALTER COLUMN "created_at" DROP DEFAULT`,
			`ALTER TABLE users ALTER COLUMN "created_at" TYPE bigint USING COALESCE(EXTRACT(EPOCH FROM "created_at")::bigint, 0)`,
		}, created, "created_at/%s drops default, extracts epoch, and never restores a default", dataType)

		login := userTimestampMigrationSQL(userTimestampColumn{Name: "last_login_at", RestoreDefault: "0"}, dataType, true)
		assert.Equalf(t, []string{
			`ALTER TABLE users ALTER COLUMN "last_login_at" DROP DEFAULT`,
			`ALTER TABLE users ALTER COLUMN "last_login_at" TYPE bigint USING COALESCE(EXTRACT(EPOCH FROM "last_login_at")::bigint, 0)`,
			`ALTER TABLE users ALTER COLUMN "last_login_at" SET DEFAULT 0`,
		}, login, "last_login_at/%s drops the legacy default, extracts epoch, and restores default 0", dataType)
	}
}

// TestUserTimestampMigrationSQL_NeverRewritesData ensures the conversion preserves values via a USING
// expression and never issues UPDATE/DELETE, so it stays safe and idempotent on large tables, and
// that every emitted type change carries the NULL->0 epoch guard.
func TestUserTimestampMigrationSQL_NeverRewritesData(t *testing.T) {
	for _, column := range userTimestampColumns() {
		for _, dataType := range []string{"timestamp with time zone", "timestamp without time zone", "date"} {
			stmts := userTimestampMigrationSQL(column, dataType, true)
			require.NotEmptyf(t, stmts, "%s/%s must emit a conversion", column.Name, dataType)
			for _, stmt := range stmts {
				upper := strings.ToUpper(stmt)
				assert.NotContains(t, upper, "UPDATE ", "must not rewrite row values")
				assert.NotContains(t, upper, "DELETE ", "must not delete rows")
			}
			assert.Contains(t, stmts[1], `COALESCE(EXTRACT(EPOCH FROM "`+column.Name+`")::bigint, 0)`,
				"the type change extracts Unix seconds with a NULL->0 guard")
		}
	}
}

// TestEnsureUserTimestampColumns_NonPostgresIsNoop confirms the timestamp pre-migration leaves SQLite
// (and by extension MySQL) completely untouched, including existing data.
func TestEnsureUserTimestampColumns_NonPostgresIsNoop(t *testing.T) {
	newUserMigrationDB(t) // configures SQLite

	require.NoError(t, DB.Exec("CREATE TABLE users (id integer PRIMARY KEY, created_at bigint, last_login_at bigint DEFAULT 0)").Error)
	require.NoError(t, DB.Exec("INSERT INTO users (id, created_at, last_login_at) VALUES (1, 1700000000, 1700000001)").Error)

	require.NoError(t, ensureUserTimestampColumns(), "must be a no-op on non-PostgreSQL")

	var createdAt, lastLoginAt int64
	require.NoError(t, DB.Raw("SELECT created_at, last_login_at FROM users WHERE id = ?", 1).Row().Scan(&createdAt, &lastLoginAt))
	assert.Equal(t, int64(1700000000), createdAt, "existing created_at must be preserved")
	assert.Equal(t, int64(1700000001), lastLoginAt, "existing last_login_at must be preserved")
}

// TestEnsureUserTimestampColumns_NoTableIsNoop ensures a fresh install (no users table yet) is left
// entirely to AutoMigrate, even when PostgreSQL is the active database.
func TestEnsureUserTimestampColumns_NoTableIsNoop(t *testing.T) {
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

	// users table is absent, so the function must return before issuing any PostgreSQL DDL.
	require.NoError(t, ensureUserTimestampColumns())
	assert.False(t, DB.Migrator().HasTable("users"))
}
