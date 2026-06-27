package model

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var commonGroupCol string
var commonKeyCol string
var commonTrueVal string
var commonFalseVal string

var logKeyCol string
var logGroupCol string

func initCol() {
	// init common column names
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		commonGroupCol = `"group"`
		commonKeyCol = `"key"`
		commonTrueVal = "true"
		commonFalseVal = "false"
	} else {
		commonGroupCol = "`group`"
		commonKeyCol = "`key`"
		commonTrueVal = "1"
		commonFalseVal = "0"
	}
	switch common.LogDatabaseType() {
	case common.DatabaseTypePostgreSQL:
		logGroupCol = `"group"`
		logKeyCol = `"key"`
	default:
		logGroupCol = "`group`"
		logKeyCol = "`key`"
	}
}

var DB *gorm.DB

var LOG_DB *gorm.DB

func createRootAccountIfNeed() error {
	var user User
	//if user.Status != common.UserStatusEnabled {
	if err := DB.First(&user).Error; err != nil {
		common.SysLog("no user exists, create a root user for you: username is root, password is 123456")
		hashedPassword, err := common.Password2Hash("123456")
		if err != nil {
			return err
		}
		rootUser := User{
			Username:    "root",
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: nil,
			Quota:       100000000,
		}
		DB.Create(&rootUser)
	}
	return nil
}

func CheckSetup() {
	setup := GetSetup()
	if setup == nil {
		// No setup record exists, check if we have a root user
		if RootUserExists() {
			common.SysLog("system is not initialized, but root user exists")
			// Create setup record
			newSetup := Setup{
				Version:       common.Version,
				InitializedAt: time.Now().Unix(),
			}
			err := DB.Create(&newSetup).Error
			if err != nil {
				common.SysLog("failed to create setup record: " + err.Error())
			}
			constant.Setup = true
		} else {
			common.SysLog("system is not initialized and no root user exists")
			constant.Setup = false
		}
	} else {
		// Setup record exists, system is initialized
		common.SysLog("system is already initialized at: " + time.Unix(setup.InitializedAt, 0).String())
		constant.Setup = true
	}
}

func isClickHouseDSN(dsn string) bool {
	return strings.HasPrefix(dsn, "clickhouse://") ||
		strings.HasPrefix(dsn, "tcp://") ||
		strings.HasPrefix(dsn, "http://") ||
		strings.HasPrefix(dsn, "https://")
}

func normalizeClickHouseDSN(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme != "https" {
		return dsn
	}
	query := parsed.Query()
	if _, ok := query["secure"]; !ok {
		query.Set("secure", "true")
		parsed.RawQuery = query.Encode()
	}
	return parsed.String()
}

func chooseDB(envName string, isLog bool) (*gorm.DB, common.DatabaseType, error) {
	dsn := os.Getenv(envName)
	if dsn != "" {
		if isClickHouseDSN(dsn) {
			if !isLog {
				return nil, "", fmt.Errorf("%s does not support ClickHouse; use SQLite, MySQL, or PostgreSQL for the primary database and LOG_SQL_DSN for ClickHouse logs", envName)
			}
			common.SysLog("using ClickHouse as log database")
			db, err := gorm.Open(clickhouse.Open(normalizeClickHouseDSN(dsn)), &gorm.Config{
				PrepareStmt: false,
			})
			return db, common.DatabaseTypeClickHouse, err
		}
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			// Use PostgreSQL
			common.SysLog("using PostgreSQL as database")
			db, err := gorm.Open(postgres.New(postgres.Config{
				DSN:                  dsn,
				PreferSimpleProtocol: true, // disables implicit prepared statement usage
			}), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
			return db, common.DatabaseTypePostgreSQL, err
		}
		if strings.HasPrefix(dsn, "local") {
			common.SysLog("SQL_DSN not set, using SQLite as database")
			db, err := gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
			return db, common.DatabaseTypeSQLite, err
		}
		// Use MySQL
		common.SysLog("using MySQL as database")
		// check parseTime
		if !strings.Contains(dsn, "parseTime") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
			PrepareStmt: true, // precompile SQL
		})
		return db, common.DatabaseTypeMySQL, err
	}
	// Use SQLite
	common.SysLog("SQL_DSN not set, using SQLite as database")
	db, err := gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
	return db, common.DatabaseTypeSQLite, err
}

func InitDB() (err error) {
	db, dbType, err := chooseDB("SQL_DSN", false)
	if err == nil {
		common.SetMainDatabaseType(dbType)
		if os.Getenv("LOG_SQL_DSN") == "" {
			common.SetLogDatabaseType(dbType)
		}
		initCol()
		if common.DebugEnabled {
			db = db.Debug()
		}
		DB = db
		// MySQL charset/collation startup check: ensure Chinese-capable charset
		if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
			if err := checkMySQLChineseSupport(DB); err != nil {
				panic(err)
			}
		}
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
			//_, _ = sqlDB.Exec("ALTER TABLE channels MODIFY model_mapping TEXT;") // TODO: delete this line when most users have upgraded
		}
		common.SysLog("database migration started")
		err = migrateDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func InitLogDB() (err error) {
	if os.Getenv("LOG_SQL_DSN") == "" {
		LOG_DB = DB
		common.SetLogDatabaseType(common.MainDatabaseType())
		initCol()
		return
	}
	db, dbType, err := chooseDB("LOG_SQL_DSN", true)
	if err == nil {
		common.SetLogDatabaseType(dbType)
		initCol()
		if common.DebugEnabled {
			db = db.Debug()
		}
		LOG_DB = db
		// If log DB is MySQL, also ensure Chinese-capable charset
		if common.UsingLogDatabase(common.DatabaseTypeMySQL) {
			if err := checkMySQLChineseSupport(LOG_DB); err != nil {
				panic(err)
			}
		}
		sqlDB, err := LOG_DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		common.SysLog("database migration started")
		err = migrateLOGDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func migrateDB() error {
	// Migrate price_amount column from float/double to decimal for existing tables
	migrateSubscriptionPlanPriceAmount()
	// Migrate model_limits column from varchar to text for existing tables
	if err := migrateTokenModelLimitsToText(); err != nil {
		return err
	}
	// Ensure channels.key exists and holds no NULLs before AutoMigrate enforces NOT NULL.
	// PostgreSQL otherwise aborts with SQLSTATE 23502 on existing tables.
	if err := ensureChannelKeyColumn(); err != nil {
		return err
	}
	// Convert legacy channels integer columns to bigint before AutoMigrate, so PostgreSQL
	// does not abort with SQLSTATE 42804 when it re-coerces their DEFAULT during the type change.
	if err := ensureChannelBigintColumns(); err != nil {
		return err
	}
	// Ensure users.username holds no duplicate values before AutoMigrate creates its unique index.
	// PostgreSQL otherwise aborts with SQLSTATE 23505 on existing tables.
	if err := ensureUserUsernameUnique(); err != nil {
		return err
	}
	// Ensure users.password exists and holds no NULLs before AutoMigrate enforces NOT NULL.
	// PostgreSQL otherwise aborts with SQLSTATE 23502 on existing tables.
	if err := ensureUserPasswordColumn(); err != nil {
		return err
	}
	// Convert legacy users integer columns to bigint before AutoMigrate, so PostgreSQL does not
	// abort with SQLSTATE 42804 when it re-coerces their DEFAULT during the type change.
	if err := ensureUserBigintColumns(); err != nil {
		return err
	}
	// Convert legacy users timestamp/date created_at/last_login_at columns to bigint Unix seconds
	// before AutoMigrate, so PostgreSQL does not abort with SQLSTATE 42846 ("cannot cast type
	// timestamp with time zone to bigint") on GORM's plain ::bigint cast.
	if err := ensureUserTimestampColumns(); err != nil {
		return err
	}

	err := DB.AutoMigrate(
		&Channel{},
		&Token{},
		&User{},
		&PasskeyCredential{},
		&Option{},
		&Redemption{},
		&Ability{},
		&Log{},
		&Midjourney{},
		&TopUp{},
		&QuotaData{},
		&Task{},
		&Model{},
		&Vendor{},
		&PrefillGroup{},
		&Setup{},
		&TwoFA{},
		&TwoFABackupCode{},
		&Checkin{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&SubscriptionPreConsumeRecord{},
		&CustomOAuthProvider{},
		&UserOAuthBinding{},
		&PerfMetric{},
		&SystemInstance{},
		&SystemTask{},
		&SystemTaskLock{},
	)
	if err != nil {
		return err
	}
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		if err := ensureSubscriptionPlanTableSQLite(); err != nil {
			return err
		}
	} else {
		if err := DB.AutoMigrate(&SubscriptionPlan{}); err != nil {
			return err
		}
	}
	return nil
}

func migrateDBFast() error {
	// Same NOT NULL guard as migrateDB: ensure channels.key before concurrent AutoMigrate.
	if err := ensureChannelKeyColumn(); err != nil {
		return err
	}
	// Same bigint guard as migrateDB: promote legacy channels integer columns before AutoMigrate.
	if err := ensureChannelBigintColumns(); err != nil {
		return err
	}
	// Same unique guard as migrateDB: de-duplicate users.username before concurrent AutoMigrate.
	if err := ensureUserUsernameUnique(); err != nil {
		return err
	}
	// Same NOT NULL guard as migrateDB: ensure users.password before concurrent AutoMigrate.
	if err := ensureUserPasswordColumn(); err != nil {
		return err
	}
	// Same bigint guard as migrateDB: promote legacy users integer columns before concurrent AutoMigrate.
	if err := ensureUserBigintColumns(); err != nil {
		return err
	}
	// Same timestamp guard as migrateDB: convert legacy users timestamp/date created_at/last_login_at
	// columns to bigint Unix seconds before concurrent AutoMigrate.
	if err := ensureUserTimestampColumns(); err != nil {
		return err
	}

	var wg sync.WaitGroup

	migrations := []struct {
		model interface{}
		name  string
	}{
		{&Channel{}, "Channel"},
		{&Token{}, "Token"},
		{&User{}, "User"},
		{&PasskeyCredential{}, "PasskeyCredential"},
		{&Option{}, "Option"},
		{&Redemption{}, "Redemption"},
		{&Ability{}, "Ability"},
		{&Log{}, "Log"},
		{&Midjourney{}, "Midjourney"},
		{&TopUp{}, "TopUp"},
		{&QuotaData{}, "QuotaData"},
		{&Task{}, "Task"},
		{&Model{}, "Model"},
		{&Vendor{}, "Vendor"},
		{&PrefillGroup{}, "PrefillGroup"},
		{&Setup{}, "Setup"},
		{&TwoFA{}, "TwoFA"},
		{&TwoFABackupCode{}, "TwoFABackupCode"},
		{&Checkin{}, "Checkin"},
		{&SubscriptionOrder{}, "SubscriptionOrder"},
		{&UserSubscription{}, "UserSubscription"},
		{&SubscriptionPreConsumeRecord{}, "SubscriptionPreConsumeRecord"},
		{&CustomOAuthProvider{}, "CustomOAuthProvider"},
		{&UserOAuthBinding{}, "UserOAuthBinding"},
		{&PerfMetric{}, "PerfMetric"},
		{&SystemInstance{}, "SystemInstance"},
		{&SystemTask{}, "SystemTask"},
		{&SystemTaskLock{}, "SystemTaskLock"},
	}
	// 动态计算migration数量，确保errChan缓冲区足够大
	errChan := make(chan error, len(migrations))

	for _, m := range migrations {
		wg.Add(1)
		go func(model interface{}, name string) {
			defer wg.Done()
			if err := DB.AutoMigrate(model); err != nil {
				errChan <- fmt.Errorf("failed to migrate %s: %v", name, err)
			}
		}(m.model, m.name)
	}

	// Wait for all migrations to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		if err := ensureSubscriptionPlanTableSQLite(); err != nil {
			return err
		}
	} else {
		if err := DB.AutoMigrate(&SubscriptionPlan{}); err != nil {
			return err
		}
	}
	common.SysLog("database migrated")
	return nil
}

func migrateLOGDB() error {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return migrateClickHouseLogDB()
	}
	return LOG_DB.AutoMigrate(&Log{})
}

func migrateClickHouseLogDB() error {
	ttlDays := clickHouseLogTTLDays()
	if err := LOG_DB.Exec(clickHouseLogCreateTableSQL(ttlDays)).Error; err != nil {
		return err
	}
	return syncClickHouseLogTTL(ttlDays)
}

func clickHouseLogTTLDays() int {
	ttlDays := common.GetEnvOrDefault("LOG_SQL_CLICKHOUSE_TTL_DAYS", 0)
	if ttlDays < 0 {
		return 0
	}
	return ttlDays
}

func clickHouseLogTTLExpression(ttlDays int) string {
	if ttlDays <= 0 {
		return ""
	}
	return fmt.Sprintf("toDateTime(created_at) + INTERVAL %d DAY DELETE", ttlDays)
}

func clickHouseLogTTLClause(ttlDays int) string {
	expression := clickHouseLogTTLExpression(ttlDays)
	if expression == "" {
		return ""
	}
	return "\nTTL " + expression
}

func clickHouseLogCreateTableSQL(ttlDays int) string {
	return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS logs (
	id Int64 DEFAULT 0,
	user_id Int32 DEFAULT 0,
	created_at Int64 DEFAULT 0,
	type Int32 DEFAULT 0,
	content String DEFAULT '',
	username String DEFAULT '',
	token_name String DEFAULT '',
	model_name String DEFAULT '',
	quota Int32 DEFAULT 0,
	prompt_tokens Int32 DEFAULT 0,
	completion_tokens Int32 DEFAULT 0,
	use_time Int32 DEFAULT 0,
	is_stream UInt8 DEFAULT 0,
	channel_id Int32 DEFAULT 0,
	token_id Int32 DEFAULT 0,
	`+"`group`"+` String DEFAULT '',
	ip String DEFAULT '',
	request_id String DEFAULT '',
	upstream_request_id String DEFAULT '',
	other String DEFAULT ''
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(toDateTime(created_at))
ORDER BY (created_at, request_id)%s`, clickHouseLogTTLClause(ttlDays))
}

func syncClickHouseLogTTL(ttlDays int) error {
	expression := clickHouseLogTTLExpression(ttlDays)
	if expression != "" {
		return LOG_DB.Exec("ALTER TABLE logs MODIFY TTL " + expression).Error
	}

	hasTTL, err := clickHouseLogTableHasTTL()
	if err != nil {
		return err
	}
	if !hasTTL {
		return nil
	}
	return LOG_DB.Exec("ALTER TABLE logs REMOVE TTL").Error
}

func clickHouseLogTableHasTTL() (bool, error) {
	var createTableSQL string
	if err := LOG_DB.Raw("SHOW CREATE TABLE logs").Scan(&createTableSQL).Error; err != nil {
		return false, err
	}
	return clickHouseCreateTableHasTTL(createTableSQL), nil
}

func clickHouseCreateTableHasTTL(createTableSQL string) bool {
	upperSQL := strings.ToUpper(createTableSQL)
	return strings.Contains(upperSQL, "\nTTL ") || strings.Contains(upperSQL, " TTL ")
}

type sqliteColumnDef struct {
	Name string
	DDL  string
}

func ensureSubscriptionPlanTableSQLite() error {
	if !common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return nil
	}
	tableName := "subscription_plans"
	if !DB.Migrator().HasTable(tableName) {
		createSQL := `CREATE TABLE ` + "`" + tableName + "`" + ` (
` + "`id`" + ` integer,
` + "`title`" + ` varchar(128) NOT NULL,
` + "`subtitle`" + ` varchar(255) DEFAULT '',
` + "`price_amount`" + ` decimal(10,6) NOT NULL,
` + "`currency`" + ` varchar(8) NOT NULL DEFAULT 'USD',
` + "`duration_unit`" + ` varchar(16) NOT NULL DEFAULT 'month',
` + "`duration_value`" + ` integer NOT NULL DEFAULT 1,
` + "`custom_seconds`" + ` bigint NOT NULL DEFAULT 0,
` + "`enabled`" + ` numeric DEFAULT 1,
` + "`sort_order`" + ` integer DEFAULT 0,
` + "`allow_balance_pay`" + ` numeric DEFAULT 1,
` + "`allow_wallet_overflow`" + ` numeric DEFAULT 1,
` + "`stripe_price_id`" + ` varchar(128) DEFAULT '',
` + "`creem_product_id`" + ` varchar(128) DEFAULT '',
` + "`waffo_pancake_product_id`" + ` varchar(128) DEFAULT '',
` + "`max_purchase_per_user`" + ` integer DEFAULT 0,
` + "`upgrade_group`" + ` varchar(64) DEFAULT '',
` + "`downgrade_group`" + ` varchar(64) DEFAULT '',
` + "`total_amount`" + ` bigint NOT NULL DEFAULT 0,
` + "`quota_reset_period`" + ` varchar(16) DEFAULT 'never',
` + "`quota_reset_custom_seconds`" + ` bigint DEFAULT 0,
` + "`created_at`" + ` bigint,
` + "`updated_at`" + ` bigint,
PRIMARY KEY (` + "`id`" + `)
)`
		return DB.Exec(createSQL).Error
	}
	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := DB.Raw("PRAGMA table_info(`" + tableName + "`)").Scan(&cols).Error; err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		existing[c.Name] = struct{}{}
	}
	required := []sqliteColumnDef{
		{Name: "title", DDL: "`title` varchar(128) NOT NULL"},
		{Name: "subtitle", DDL: "`subtitle` varchar(255) DEFAULT ''"},
		{Name: "price_amount", DDL: "`price_amount` decimal(10,6) NOT NULL"},
		{Name: "currency", DDL: "`currency` varchar(8) NOT NULL DEFAULT 'USD'"},
		{Name: "duration_unit", DDL: "`duration_unit` varchar(16) NOT NULL DEFAULT 'month'"},
		{Name: "duration_value", DDL: "`duration_value` integer NOT NULL DEFAULT 1"},
		{Name: "custom_seconds", DDL: "`custom_seconds` bigint NOT NULL DEFAULT 0"},
		{Name: "enabled", DDL: "`enabled` numeric DEFAULT 1"},
		{Name: "sort_order", DDL: "`sort_order` integer DEFAULT 0"},
		{Name: "allow_balance_pay", DDL: "`allow_balance_pay` numeric DEFAULT 1"},
		{Name: "allow_wallet_overflow", DDL: "`allow_wallet_overflow` numeric DEFAULT 1"},
		{Name: "stripe_price_id", DDL: "`stripe_price_id` varchar(128) DEFAULT ''"},
		{Name: "creem_product_id", DDL: "`creem_product_id` varchar(128) DEFAULT ''"},
		{Name: "waffo_pancake_product_id", DDL: "`waffo_pancake_product_id` varchar(128) DEFAULT ''"},
		{Name: "max_purchase_per_user", DDL: "`max_purchase_per_user` integer DEFAULT 0"},
		{Name: "upgrade_group", DDL: "`upgrade_group` varchar(64) DEFAULT ''"},
		{Name: "downgrade_group", DDL: "`downgrade_group` varchar(64) DEFAULT ''"},
		{Name: "total_amount", DDL: "`total_amount` bigint NOT NULL DEFAULT 0"},
		{Name: "quota_reset_period", DDL: "`quota_reset_period` varchar(16) DEFAULT 'never'"},
		{Name: "quota_reset_custom_seconds", DDL: "`quota_reset_custom_seconds` bigint DEFAULT 0"},
		{Name: "created_at", DDL: "`created_at` bigint"},
		{Name: "updated_at", DDL: "`updated_at` bigint"},
	}
	for _, col := range required {
		if _, ok := existing[col.Name]; ok {
			continue
		}
		if err := DB.Exec("ALTER TABLE `" + tableName + "` ADD COLUMN " + col.DDL).Error; err != nil {
			return err
		}
	}
	return nil
}

// ensureChannelKeyColumn makes sure the channels.key column exists and holds no NULL values
// before AutoMigrate enforces the NOT NULL constraint declared on Channel.Key. On existing
// tables PostgreSQL aborts startup migration with SQLSTATE 23502 ("column \"key\" of relation
// \"channels\" contains null values") when GORM tries to add or promote the column to NOT NULL
// while rows are NULL. This backfills empty strings and is safe to run repeatedly on SQLite,
// MySQL, and PostgreSQL without clobbering existing keys.
func ensureChannelKeyColumn() error {
	const tableName = "channels"

	// Fresh database: let AutoMigrate create the table with the correct schema.
	if !DB.Migrator().HasTable(tableName) {
		return nil
	}

	keyCol := commonKeyCol // dialect-quoted: "key" on PostgreSQL, `key` elsewhere
	hasColumn := DB.Migrator().HasColumn(&Channel{}, "key")

	switch {
	case common.UsingMainDatabase(common.DatabaseTypePostgreSQL):
		if !hasColumn {
			// Add as nullable first so existing rows do not violate NOT NULL.
			if err := DB.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s text`, tableName, keyCol)).Error; err != nil {
				return fmt.Errorf("ensure channels.key: add column: %w", err)
			}
		}
		if err := DB.Exec(fmt.Sprintf(`UPDATE %s SET %s = '' WHERE %s IS NULL`, tableName, keyCol, keyCol)).Error; err != nil {
			return fmt.Errorf("ensure channels.key: backfill nulls: %w", err)
		}
		// Promote to NOT NULL (idempotent) so AutoMigrate finds a matching schema.
		if err := DB.Exec(fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s SET NOT NULL`, tableName, keyCol)).Error; err != nil {
			return fmt.Errorf("ensure channels.key: set not null: %w", err)
		}
	case common.UsingMainDatabase(common.DatabaseTypeMySQL):
		if !hasColumn {
			// TEXT columns cannot carry a DEFAULT on older MySQL, so add nullable then backfill.
			if err := DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s text", tableName, keyCol)).Error; err != nil {
				return fmt.Errorf("ensure channels.key: add column: %w", err)
			}
		}
		if err := DB.Exec(fmt.Sprintf("UPDATE %s SET %s = '' WHERE %s IS NULL", tableName, keyCol, keyCol)).Error; err != nil {
			return fmt.Errorf("ensure channels.key: backfill nulls: %w", err)
		}
		if err := DB.Exec(fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s text NOT NULL", tableName, keyCol)).Error; err != nil {
			return fmt.Errorf("ensure channels.key: set not null: %w", err)
		}
	case common.UsingMainDatabase(common.DatabaseTypeSQLite):
		if !hasColumn {
			// SQLite can only add a NOT NULL column together with a default, and cannot
			// ALTER COLUMN afterwards, so add it complete in one statement.
			if err := DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s text NOT NULL DEFAULT ''", tableName, keyCol)).Error; err != nil {
				return fmt.Errorf("ensure channels.key: add column: %w", err)
			}
			return nil
		}
		// Column already exists; SQLite lacks ALTER COLUMN, so just clear NULLs to keep data consistent.
		if err := DB.Exec(fmt.Sprintf("UPDATE %s SET %s = '' WHERE %s IS NULL", tableName, keyCol, keyCol)).Error; err != nil {
			return fmt.Errorf("ensure channels.key: backfill nulls: %w", err)
		}
	}
	return nil
}

// channelBigintColumn is a Channel column declared as a Go int-family field with a GORM
// default, which GORM maps to PostgreSQL bigint. Default mirrors the struct tag so the
// column keeps its intended default after the type change.
type channelBigintColumn struct {
	Name    string
	Default string
}

// channelBigintColumns lists the Channel integer columns that carry a GORM default and are
// promoted to bigint. These are exactly the int-family fields with a `default:` tag
// (Type/Status/Weight/AutoBan); the other integer columns either already declare `bigint`
// (Priority, UsedQuota, *Time) or have no default (ResponseTime), so GORM converts them
// without re-coercing a DEFAULT and never hits SQLSTATE 42804.
func channelBigintColumns() []channelBigintColumn {
	return []channelBigintColumn{
		{Name: "status", Default: "1"},   // gorm:"default:1"
		{Name: "type", Default: "0"},     // gorm:"default:0"
		{Name: "weight", Default: "0"},   // *uint gorm:"default:0"
		{Name: "auto_ban", Default: "1"}, // *int gorm:"default:1"
	}
}

// isPostgresStringType reports whether a PostgreSQL information_schema.data_type names a
// character/text type. Legacy channels columns persisted as such may hold non-numeric values
// (e.g. status = 'active'), so a plain ::bigint cast aborts with SQLSTATE 22P02 and the column
// must instead be normalized with an explicit mapping before the type change.
func isPostgresStringType(dataType string) bool {
	switch dataType {
	case "text", "citext", "name":
		return true
	}
	// "character varying", "character", "varchar", "char", "bpchar".
	return strings.Contains(dataType, "char")
}

// channelStatusStringMapping maps a set of legacy free-form status tokens (already lowercased
// and whitespace-trimmed) to a New API numeric channel status.
type channelStatusStringMapping struct {
	Value  int
	Tokens []string
}

// channelStatusStringMappings is the single source of truth for normalizing legacy textual
// channel statuses into New API numeric statuses. It drives the PostgreSQL USING expression in
// channelStatusBigintUsingExpr and is asserted directly in tests, so the SQL and the documented
// mapping can never drift. '0' is intentionally treated as a legacy "off" token (-> manually
// disabled) rather than the New API unknown(0), matching how external systems store the field.
func channelStatusStringMappings() []channelStatusStringMapping {
	return []channelStatusStringMapping{
		{Value: common.ChannelStatusEnabled, Tokens: []string{"active", "enabled", "enable", "true", "1"}},
		{Value: common.ChannelStatusManuallyDisabled, Tokens: []string{"disabled", "disable", "inactive", "false", "0", "2", "manual_disabled", "manually_disabled", "manual-disabled"}},
		{Value: common.ChannelStatusAutoDisabled, Tokens: []string{"auto_disabled", "auto-disabled", "auto", "3"}},
		{Value: common.ChannelStatusUnknown, Tokens: []string{"unknown", ""}},
	}
}

// channelStatusBigintUsingExpr builds the PostgreSQL USING expression that converts a legacy
// string status column to bigint. Tokens are matched case- and whitespace-insensitively. Any
// other pure-integer string is cast verbatim so genuine numeric values survive untouched, and
// unrecognized free-form text collapses to unknown(0) instead of aborting startup.
func channelStatusBigintUsingExpr() string {
	norm := `lower(btrim("status"::text))`
	var b strings.Builder
	b.WriteString("CASE ")
	for _, m := range channelStatusStringMappings() {
		quoted := make([]string, len(m.Tokens))
		for i, tok := range m.Tokens {
			quoted[i] = "'" + tok + "'"
		}
		b.WriteString(fmt.Sprintf("WHEN %s IN (%s) THEN %d ", norm, strings.Join(quoted, ", "), m.Value))
	}
	b.WriteString(fmt.Sprintf("WHEN %s ~ '^[0-9]+$' THEN %s::bigint ", norm, norm))
	b.WriteString(fmt.Sprintf("ELSE %d END", common.ChannelStatusUnknown))
	return b.String()
}

// channelNumericBigintUsingExpr builds the USING expression for a string-typed numeric column
// (type/weight/auto_ban) that should still hold only numbers: blanks fall back to the column
// default, everything else is trimmed and cast. Genuine non-numeric junk fails the cast loudly
// (SQLSTATE 22P02), which the caller wraps with column context.
func channelNumericBigintUsingExpr(column channelBigintColumn) string {
	trimmed := fmt.Sprintf(`btrim("%s"::text)`, column.Name)
	return fmt.Sprintf(`CASE WHEN %s = '' THEN %s ELSE %s::bigint END`, trimmed, column.Default, trimmed)
}

// channelBigintUsingExpr picks the USING expression for the type change. A numeric source column
// is cast verbatim (preserving genuine values, including the original SQLSTATE 42804 path). A
// string source column is normalized: status through its legacy-token mapping, the other numeric
// columns through a trim-and-cast guard.
func channelBigintUsingExpr(column channelBigintColumn, isStringType bool) string {
	col := `"` + column.Name + `"`
	if !isStringType {
		return col + `::bigint`
	}
	if column.Name == "status" {
		return channelStatusBigintUsingExpr()
	}
	return channelNumericBigintUsingExpr(column)
}

// channelBigintMigrationSQL returns the PostgreSQL statements that convert one channels
// column to bigint, or nil when no work is needed. dataType is the column's current
// information_schema.data_type and exists reports whether the column is present.
//
// The conversion drops the DEFAULT first because PostgreSQL re-coerces a column's DEFAULT
// during ALTER COLUMN ... TYPE; an integer/varchar default cannot be cast automatically to
// bigint (SQLSTATE 42804), which is what aborts AutoMigrate. The USING expression then changes
// the type: a numeric source is cast verbatim, while a string source (which may hold legacy
// values like status = 'active', the SQLSTATE 22P02 case) is normalized first. After the type
// change the default is restored. It returns nil when the column is missing (AutoMigrate will
// add it correctly) or already bigint (so startup never rewrites the table twice).
func channelBigintMigrationSQL(column channelBigintColumn, dataType string, exists bool) []string {
	if !exists || dataType == "bigint" {
		return nil
	}
	col := `"` + column.Name + `"`
	using := channelBigintUsingExpr(column, isPostgresStringType(dataType))
	return []string{
		fmt.Sprintf(`ALTER TABLE channels ALTER COLUMN %s DROP DEFAULT`, col),
		fmt.Sprintf(`ALTER TABLE channels ALTER COLUMN %s TYPE bigint USING %s`, col, using),
		fmt.Sprintf(`ALTER TABLE channels ALTER COLUMN %s SET DEFAULT %s`, col, column.Default),
	}
}

// ensureChannelBigintColumns promotes legacy channels integer columns to bigint on
// PostgreSQL before AutoMigrate. GORM maps Channel's Type/Status/Weight/AutoBan to bigint;
// when an existing table stores them as a narrower type with a DEFAULT, GORM's
// "ALTER COLUMN ... TYPE bigint USING ...::bigint" makes PostgreSQL re-coerce the DEFAULT and
// abort with SQLSTATE 42804 ("default for column \"status\" cannot be cast automatically to
// type bigint"). A second class of legacy tables stores these columns as a string type holding
// non-numeric values (e.g. status = 'active'), where a plain ::bigint cast instead aborts with
// SQLSTATE 22P02 ("invalid input syntax for type bigint"). This pre-migration drops the default,
// changes the type with a column-appropriate USING expression (verbatim cast for numeric
// sources; a legacy-token mapping for a string status; a trim-and-cast guard for the other
// string numeric columns), then restores the default. It is a no-op on SQLite (type affinity)
// and MySQL (implicit default widening), skips a missing table/column, skips columns already
// bigint, preserves genuine values, and is safe to run repeatedly.
func ensureChannelBigintColumns() error {
	if !common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		return nil
	}
	const tableName = "channels"
	// Fresh database: AutoMigrate creates the table as bigint with the correct defaults.
	if !DB.Migrator().HasTable(tableName) {
		return nil
	}
	for _, column := range channelBigintColumns() {
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, column.Name).Scan(&dataType).Error; err != nil {
			return fmt.Errorf("ensure channels.%s: inspect type: %w", column.Name, err)
		}
		statements := channelBigintMigrationSQL(column, dataType, dataType != "")
		if len(statements) == 0 {
			continue
		}
		// Run the drop/alter/restore as one transaction so a failed conversion (e.g. dirty
		// data the USING cast cannot parse) never leaves the column without its default.
		if err := DB.Transaction(func(tx *gorm.DB) error {
			for _, stmt := range statements {
				if err := tx.Exec(stmt).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("ensure channels.%s: convert to bigint: %w", column.Name, err)
		}
		common.SysLog(fmt.Sprintf("migrated channels.%s from %s to bigint", column.Name, dataType))
	}
	return nil
}

// ensureUserUsernameUnique de-duplicates users.username before AutoMigrate enforces the unique
// index declared on User.Username. On existing tables PostgreSQL aborts startup migration with
// SQLSTATE 23505 ("could not create unique index \"idx_users_username\"") when two or more rows
// share a username. For each duplicated value it keeps the earliest row (lowest id) unchanged and
// renames the later rows to a traceable, collision-free username, so no user is deleted and the
// original owner of a name keeps it.
//
// NULL usernames are left untouched because every supported database treats NULLs as distinct in
// a unique constraint, so multiple NULLs never block the index; empty strings are de-duplicated
// because they compare equal and collide. The duplicate scan uses raw SQL rather than a GORM model query so
// it also covers soft-deleted rows, which still occupy the unique index. "username" is not a
// reserved word on any supported database, so it needs no dialect quoting. The function runs on
// SQLite, MySQL, and PostgreSQL alike and is safe to run repeatedly.
func ensureUserUsernameUnique() error {
	const tableName = "users"

	// Fresh database: let AutoMigrate create the table and its unique index.
	if !DB.Migrator().HasTable(tableName) {
		return nil
	}
	if !DB.Migrator().HasColumn(&User{}, "username") {
		return nil
	}

	// Usernames shared by more than one physical row. NULL is excluded (distinct in a unique
	// constraint); empty string is included because it collides.
	var duplicates []string
	if err := DB.Raw("SELECT username FROM " + tableName +
		" WHERE username IS NOT NULL GROUP BY username HAVING COUNT(*) > 1").Scan(&duplicates).Error; err != nil {
		return fmt.Errorf("ensure users.username unique: find duplicates: %w", err)
	}
	if len(duplicates) == 0 {
		return nil
	}

	// Every username currently stored, so a generated name never collides with a real account or
	// with another rename performed in this same run.
	var existing []string
	if err := DB.Raw("SELECT username FROM " + tableName + " WHERE username IS NOT NULL").Scan(&existing).Error; err != nil {
		return fmt.Errorf("ensure users.username unique: load existing usernames: %w", err)
	}
	taken := make(map[string]struct{}, len(existing))
	for _, name := range existing {
		taken[name] = struct{}{}
	}

	for _, username := range duplicates {
		var ids []int
		if err := DB.Raw("SELECT id FROM "+tableName+" WHERE username = ? ORDER BY id ASC", username).Scan(&ids).Error; err != nil {
			return fmt.Errorf("ensure users.username unique: list rows for username %q: %w", username, err)
		}
		if len(ids) < 2 {
			continue
		}
		// Keep the earliest row (ids[0]) untouched; rename every later duplicate.
		for _, id := range ids[1:] {
			base := username
			if base == "" {
				base = "migration_user"
			}
			// Candidate keeps the original name plus the unique row id; a numeric suffix is only
			// appended when that value is already taken by another account or an earlier rename.
			newName := fmt.Sprintf("%s_%d", base, id)
			for suffix := 1; ; suffix++ {
				if _, exists := taken[newName]; !exists {
					break
				}
				newName = fmt.Sprintf("%s_%d_%d", base, id, suffix)
			}
			if err := DB.Exec("UPDATE "+tableName+" SET username = ? WHERE id = ?", newName, id).Error; err != nil {
				return fmt.Errorf("ensure users.username unique: rename id %d: %w", id, err)
			}
			taken[newName] = struct{}{}
			common.SysLog(fmt.Sprintf("ensure users.username unique: renamed duplicate id %d to %q to allow unique index", id, newName))
		}
	}
	return nil
}

// ensureUserPasswordColumn makes sure the users.password column exists and holds no NULL values
// before AutoMigrate enforces the NOT NULL constraint declared on User.Password. On existing
// tables PostgreSQL aborts startup migration with SQLSTATE 23502 ("column \"password\" of relation
// \"users\" contains null values") when GORM tries to add or promote the column to NOT NULL while
// rows are NULL. This backfills empty strings for NULL passwords without clobbering existing
// non-NULL password hashes, and is safe to run repeatedly on SQLite, MySQL, and PostgreSQL.
//
// The empty-string backfill only satisfies the NOT NULL constraint; the login path never accepts
// an empty password, so a backfilled row cannot authenticate until its owner sets a real password.
func ensureUserPasswordColumn() error {
	const tableName = "users"

	// Fresh database: let AutoMigrate create the table with the correct schema.
	if !DB.Migrator().HasTable(tableName) {
		return nil
	}

	hasColumn := DB.Migrator().HasColumn(&User{}, "password")

	switch {
	case common.UsingMainDatabase(common.DatabaseTypePostgreSQL):
		if !hasColumn {
			// Add as nullable first so existing rows do not violate NOT NULL.
			if err := DB.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN password text`, tableName)).Error; err != nil {
				return fmt.Errorf("ensure users.password: add column: %w", err)
			}
		}
		if err := DB.Exec(fmt.Sprintf(`UPDATE %s SET password = '' WHERE password IS NULL`, tableName)).Error; err != nil {
			return fmt.Errorf("ensure users.password: backfill nulls: %w", err)
		}
		// Promote to NOT NULL (idempotent) so AutoMigrate finds a matching schema.
		if err := DB.Exec(fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN password SET NOT NULL`, tableName)).Error; err != nil {
			return fmt.Errorf("ensure users.password: set not null: %w", err)
		}
	case common.UsingMainDatabase(common.DatabaseTypeMySQL):
		if !hasColumn {
			// TEXT columns cannot carry a DEFAULT on older MySQL, so add nullable then backfill.
			if err := DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN password text", tableName)).Error; err != nil {
				return fmt.Errorf("ensure users.password: add column: %w", err)
			}
		}
		if err := DB.Exec(fmt.Sprintf("UPDATE %s SET password = '' WHERE password IS NULL", tableName)).Error; err != nil {
			return fmt.Errorf("ensure users.password: backfill nulls: %w", err)
		}
		if err := DB.Exec(fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN password text NOT NULL", tableName)).Error; err != nil {
			return fmt.Errorf("ensure users.password: set not null: %w", err)
		}
	case common.UsingMainDatabase(common.DatabaseTypeSQLite):
		if !hasColumn {
			// SQLite can only add a NOT NULL column together with a default, and cannot
			// ALTER COLUMN afterwards, so add it complete in one statement.
			if err := DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN password text NOT NULL DEFAULT ''", tableName)).Error; err != nil {
				return fmt.Errorf("ensure users.password: add column: %w", err)
			}
			return nil
		}
		// Column already exists; SQLite lacks ALTER COLUMN, so just clear NULLs to keep data consistent.
		if err := DB.Exec(fmt.Sprintf("UPDATE %s SET password = '' WHERE password IS NULL", tableName)).Error; err != nil {
			return fmt.Errorf("ensure users.password: backfill nulls: %w", err)
		}
	}
	return nil
}

// userBigintColumn is a User column declared as a Go int-family field with a GORM default, which
// GORM resolves to PostgreSQL bigint. Go int is 64-bit, so even the struct tag `type:int` resolves
// to GORM's abstract Int type and maps to bigint. Default mirrors the struct tag so the column keeps
// its intended default after the type change.
type userBigintColumn struct {
	Name    string
	Default string
}

// userBigintColumns lists the User integer columns that carry a GORM default and are promoted to
// bigint before AutoMigrate. Every entry declares a `default:` in its struct tag and maps to bigint
// (Go int is 64-bit), so on a legacy table that still stores them as a narrower integer type GORM's
// own "ALTER COLUMN ... TYPE bigint" re-coerces the column DEFAULT and aborts with SQLSTATE 42804
// ("default for column \"role\" cannot be cast automatically to type bigint"). InviterId is excluded
// because it has no default (GORM converts it without re-coercing one); CreatedAt/LastLoginAt are
// int64 and therefore already bigint. role/status additionally get legacy-string normalization,
// the quota-family columns only ever hold numbers.
func userBigintColumns() []userBigintColumn {
	return []userBigintColumn{
		{Name: "role", Default: "1"},          // gorm:"type:int;default:1"
		{Name: "status", Default: "1"},        // gorm:"type:int;default:1"
		{Name: "quota", Default: "0"},         // gorm:"type:int;default:0"
		{Name: "used_quota", Default: "0"},    // gorm:"type:int;default:0;column:used_quota"
		{Name: "request_count", Default: "0"}, // gorm:"type:int;default:0"
		{Name: "aff_count", Default: "0"},     // gorm:"type:int;default:0;column:aff_count"
		{Name: "aff_quota", Default: "0"},     // gorm:"type:int;default:0;column:aff_quota"
		{Name: "aff_history", Default: "0"},   // gorm:"type:int;default:0;column:aff_history"
	}
}

// userStringMapping maps a set of legacy free-form tokens (already lowercased and whitespace-trimmed)
// to a New API numeric value. It is shared by the role and status normalizers.
type userStringMapping struct {
	Value  int
	Tokens []string
}

// userRoleStringMappings is the single source of truth for normalizing legacy textual user roles
// into New API numeric roles. It drives userRoleBigintUsingExpr and is asserted directly in tests so
// the SQL and the documented mapping can never drift. Genuine numeric roles (0/1/10/100, or any
// custom value) are handled by the numeric passthrough in the USING expression, not here.
func userRoleStringMappings() []userStringMapping {
	return []userStringMapping{
		{Value: common.RoleRootUser, Tokens: []string{"root", "superadmin", "super_admin"}},
		{Value: common.RoleAdminUser, Tokens: []string{"admin", "administrator"}},
		{Value: common.RoleCommonUser, Tokens: []string{"common", "user", "normal", "member"}},
		{Value: common.RoleGuestUser, Tokens: []string{"guest", "anonymous"}},
	}
}

// userStatusStringMappings is the single source of truth for normalizing legacy textual user
// statuses into New API numeric statuses. It drives userStatusBigintUsingExpr and is asserted in
// tests. Pure-integer strings are intentionally left to the numeric passthrough so the string path
// preserves the same values the integer column would (e.g. 0/1/2); only purely textual tokens map
// to the canonical enabled/disabled values.
func userStatusStringMappings() []userStringMapping {
	return []userStringMapping{
		{Value: common.UserStatusEnabled, Tokens: []string{"enabled", "enable", "active", "on", "true", "yes"}},
		{Value: common.UserStatusDisabled, Tokens: []string{"disabled", "disable", "inactive", "off", "false", "no", "banned", "blocked"}},
	}
}

// userStringBigintUsingExpr builds a PostgreSQL USING expression that converts a legacy string
// column to bigint through a token mapping. Tokens are matched case- and whitespace-insensitively,
// any pure-integer string is cast verbatim so genuine numeric values survive untouched, and
// unrecognized free-form text collapses to fallback instead of aborting startup (SQLSTATE 22P02).
func userStringBigintUsingExpr(column string, mappings []userStringMapping, fallback int) string {
	norm := fmt.Sprintf(`lower(btrim("%s"::text))`, column)
	var b strings.Builder
	b.WriteString("CASE ")
	for _, m := range mappings {
		quoted := make([]string, len(m.Tokens))
		for i, tok := range m.Tokens {
			quoted[i] = "'" + tok + "'"
		}
		b.WriteString(fmt.Sprintf("WHEN %s IN (%s) THEN %d ", norm, strings.Join(quoted, ", "), m.Value))
	}
	b.WriteString(fmt.Sprintf("WHEN %s ~ '^[0-9]+$' THEN %s::bigint ", norm, norm))
	b.WriteString(fmt.Sprintf("ELSE %d END", fallback))
	return b.String()
}

// userRoleBigintUsingExpr normalizes a legacy string role column to bigint. Unrecognized text falls
// back to the least-privilege RoleGuestUser: it never silently escalates an unknown role to admin or
// root, and recognized privileged tokens (root/admin) are still mapped explicitly.
func userRoleBigintUsingExpr() string {
	return userStringBigintUsingExpr("role", userRoleStringMappings(), common.RoleGuestUser)
}

// userStatusBigintUsingExpr normalizes a legacy string status column to bigint. Unrecognized text
// falls back to the fail-closed UserStatusDisabled rather than enabling an account whose status
// could not be parsed.
func userStatusBigintUsingExpr() string {
	return userStringBigintUsingExpr("status", userStatusStringMappings(), common.UserStatusDisabled)
}

// userNumericBigintUsingExpr builds the USING expression for a string-typed numeric column
// (quota/used_quota/request_count/aff_count/aff_quota/aff_history) that should still hold only
// numbers: blanks fall back to the column default, everything else is trimmed and cast. Genuine
// non-numeric junk fails the cast loudly (SQLSTATE 22P02), which the caller wraps with column context.
func userNumericBigintUsingExpr(column userBigintColumn) string {
	trimmed := fmt.Sprintf(`btrim("%s"::text)`, column.Name)
	return fmt.Sprintf(`CASE WHEN %s = '' THEN %s ELSE %s::bigint END`, trimmed, column.Default, trimmed)
}

// userBigintUsingExpr picks the USING expression for the type change. A numeric source column is
// cast verbatim (preserving genuine values, including the original SQLSTATE 42804 path). A string
// source column is normalized: role and status through their legacy-token mappings, the quota-family
// numeric columns through a trim-and-cast guard.
func userBigintUsingExpr(column userBigintColumn, isStringType bool) string {
	col := `"` + column.Name + `"`
	if !isStringType {
		return col + `::bigint`
	}
	switch column.Name {
	case "role":
		return userRoleBigintUsingExpr()
	case "status":
		return userStatusBigintUsingExpr()
	default:
		return userNumericBigintUsingExpr(column)
	}
}

// userBigintMigrationSQL returns the PostgreSQL statements that convert one users column to bigint,
// or nil when no work is needed. dataType is the column's current information_schema.data_type and
// exists reports whether the column is present.
//
// The conversion drops the DEFAULT first because PostgreSQL re-coerces a column's DEFAULT during
// ALTER COLUMN ... TYPE; an integer/varchar default cannot be cast automatically to bigint
// (SQLSTATE 42804), which is what aborts AutoMigrate. The USING expression then changes the type: a
// numeric source is cast verbatim, while a string source (which may hold legacy values like role =
// 'admin' or status = 'enabled', the SQLSTATE 22P02 case) is normalized first. After the type change
// the default is restored. It returns nil when the column is missing (AutoMigrate will add it
// correctly) or already bigint (so startup never rewrites the table twice).
func userBigintMigrationSQL(column userBigintColumn, dataType string, exists bool) []string {
	if !exists || dataType == "bigint" {
		return nil
	}
	col := `"` + column.Name + `"`
	using := userBigintUsingExpr(column, isPostgresStringType(dataType))
	return []string{
		fmt.Sprintf(`ALTER TABLE users ALTER COLUMN %s DROP DEFAULT`, col),
		fmt.Sprintf(`ALTER TABLE users ALTER COLUMN %s TYPE bigint USING %s`, col, using),
		fmt.Sprintf(`ALTER TABLE users ALTER COLUMN %s SET DEFAULT %s`, col, column.Default),
	}
}

// ensureUserBigintColumns promotes legacy users integer columns to bigint on PostgreSQL before
// AutoMigrate. GORM maps User's defaulted int fields (Role/Status/Quota/...) to bigint; when an
// existing table stores them as a narrower type with a DEFAULT, GORM's
// "ALTER COLUMN ... TYPE bigint USING ...::bigint" makes PostgreSQL re-coerce the DEFAULT and abort
// with SQLSTATE 42804 ("default for column \"role\" cannot be cast automatically to type bigint").
// A second class of legacy tables stores these columns as a string type holding non-numeric values
// (e.g. role = 'admin', status = 'enabled'), where a plain ::bigint cast instead aborts with
// SQLSTATE 22P02 ("invalid input syntax for type bigint"). This pre-migration drops the default,
// changes the type with a column-appropriate USING expression (verbatim cast for numeric sources;
// legacy-token mappings for a string role/status; a trim-and-cast guard for the other string numeric
// columns), then restores the default. It is a no-op on SQLite (type affinity) and MySQL (implicit
// default widening), skips a missing table/column, skips columns already bigint, preserves genuine
// values, and is safe to run repeatedly.
func ensureUserBigintColumns() error {
	if !common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		return nil
	}
	const tableName = "users"
	// Fresh database: AutoMigrate creates the table as bigint with the correct defaults.
	if !DB.Migrator().HasTable(tableName) {
		return nil
	}
	for _, column := range userBigintColumns() {
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, column.Name).Scan(&dataType).Error; err != nil {
			return fmt.Errorf("ensure users.%s: inspect type: %w", column.Name, err)
		}
		statements := userBigintMigrationSQL(column, dataType, dataType != "")
		if len(statements) == 0 {
			continue
		}
		// Run the drop/alter/restore as one transaction so a failed conversion (e.g. dirty data the
		// USING cast cannot parse) never leaves the column without its default.
		if err := DB.Transaction(func(tx *gorm.DB) error {
			for _, stmt := range statements {
				if err := tx.Exec(stmt).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("ensure users.%s: convert to bigint: %w", column.Name, err)
		}
		common.SysLog(fmt.Sprintf("migrated users.%s from %s to bigint", column.Name, dataType))
	}
	return nil
}

// isPostgresTimestampType reports whether a PostgreSQL information_schema.data_type names a date/time
// type. Legacy users tables sometimes persist the int64 Unix-second columns (created_at,
// last_login_at) as a timestamp/date instead, so GORM's plain
// "ALTER COLUMN ... TYPE bigint USING ...::bigint" aborts with SQLSTATE 42846 ("cannot cast type
// timestamp with time zone to bigint"). Such columns must instead be converted with
// EXTRACT(EPOCH FROM ...) before the type change.
func isPostgresTimestampType(dataType string) bool {
	if dataType == "date" {
		return true
	}
	// "timestamp with time zone" and "timestamp without time zone".
	return strings.HasPrefix(dataType, "timestamp")
}

// userTimestampColumn is a User column declared as int64 Unix seconds (CreatedAt/LastLoginAt) that a
// legacy PostgreSQL table may instead store as a timestamp/date type. RestoreDefault is the bigint
// default to set after the type change, or "" to leave the column without a SQL default so the
// resulting schema matches the model and AutoMigrate does not re-issue ALTER on every restart.
type userTimestampColumn struct {
	Name           string
	RestoreDefault string
}

// userTimestampColumns lists the User int64 Unix-second columns that a legacy PostgreSQL table may
// store as a timestamp/date and that must be converted to bigint before AutoMigrate. created_at uses
// gorm:"autoCreateTime" (set in Go, no SQL default) so it restores no default; last_login_at uses
// gorm:"default:0" so it restores DEFAULT 0. They are intentionally absent from userBigintColumns,
// whose verbatim ::bigint cast cannot convert a timestamp value.
func userTimestampColumns() []userTimestampColumn {
	return []userTimestampColumn{
		{Name: "created_at", RestoreDefault: ""},     // gorm:"autoCreateTime" — no SQL default
		{Name: "last_login_at", RestoreDefault: "0"}, // gorm:"default:0"
	}
}

// userTimestampMigrationSQL returns the PostgreSQL statements that convert one legacy timestamp/date
// users column to a bigint Unix-second column, or nil when no work is needed. dataType is the
// column's current information_schema.data_type and exists reports whether the column is present.
//
// It returns nil when the column is missing (AutoMigrate adds it correctly), already bigint (so
// startup never rewrites the table twice), or any non-time type such as integer (AutoMigrate's own
// ::bigint cast already promotes integer->bigint without the 42846 failure). For an actual
// timestamp/date column it drops the default first — a legacy now()/CURRENT_TIMESTAMP default cannot
// be re-coerced to bigint during ALTER COLUMN ... TYPE — then changes the type with
// EXTRACT(EPOCH FROM ...) to derive Unix seconds, guarding NULL to 0 so the int64 field never reads a
// NULL. Only last_login_at restores a default (0); created_at deliberately keeps none. The whole
// thing is value-preserving (a USING expression, never UPDATE/DELETE) and idempotent.
func userTimestampMigrationSQL(column userTimestampColumn, dataType string, exists bool) []string {
	if !exists || dataType == "bigint" || !isPostgresTimestampType(dataType) {
		return nil
	}
	col := `"` + column.Name + `"`
	statements := []string{
		fmt.Sprintf(`ALTER TABLE users ALTER COLUMN %s DROP DEFAULT`, col),
		fmt.Sprintf(`ALTER TABLE users ALTER COLUMN %s TYPE bigint USING COALESCE(EXTRACT(EPOCH FROM %s)::bigint, 0)`, col, col),
	}
	if column.RestoreDefault != "" {
		statements = append(statements, fmt.Sprintf(`ALTER TABLE users ALTER COLUMN %s SET DEFAULT %s`, col, column.RestoreDefault))
	}
	return statements
}

// ensureUserTimestampColumns converts legacy users timestamp/date columns to bigint Unix seconds on
// PostgreSQL before AutoMigrate. User.CreatedAt and User.LastLoginAt are int64 and GORM maps them to
// bigint; when an older table stored them as a timestamp/date (e.g. created_at timestamptz DEFAULT
// now()), GORM's "ALTER COLUMN ... TYPE bigint USING ...::bigint" aborts startup with SQLSTATE 42846
// ("cannot cast type timestamp with time zone to bigint"). This pre-migration drops any legacy
// default, changes the type via EXTRACT(EPOCH FROM ...) with a NULL->0 guard, and restores DEFAULT 0
// only for last_login_at (created_at keeps none, matching autoCreateTime). It is a no-op on SQLite
// and MySQL, skips a missing table/column, skips columns already bigint or any non-time type,
// preserves values without UPDATE/DELETE, and is safe to run repeatedly. It runs alongside
// ensureUserBigintColumns, which handles the disjoint defaulted-int columns (role/status/quota/...).
func ensureUserTimestampColumns() error {
	if !common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		return nil
	}
	const tableName = "users"
	// Fresh database: AutoMigrate creates the table as bigint with the correct schema.
	if !DB.Migrator().HasTable(tableName) {
		return nil
	}
	for _, column := range userTimestampColumns() {
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, column.Name).Scan(&dataType).Error; err != nil {
			return fmt.Errorf("ensure users.%s: inspect type: %w", column.Name, err)
		}
		statements := userTimestampMigrationSQL(column, dataType, dataType != "")
		if len(statements) == 0 {
			continue
		}
		// Run the drop/alter/restore as one transaction so a failed conversion never leaves the
		// column without its default.
		if err := DB.Transaction(func(tx *gorm.DB) error {
			for _, stmt := range statements {
				if err := tx.Exec(stmt).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("ensure users.%s: convert to bigint: %w", column.Name, err)
		}
		common.SysLog(fmt.Sprintf("migrated users.%s from %s to bigint", column.Name, dataType))
	}
	return nil
}

// migrateTokenModelLimitsToText migrates model_limits column from varchar(1024) to text
// This is safe to run multiple times - it checks the column type first
func migrateTokenModelLimitsToText() error {
	// SQLite uses type affinity, so TEXT and VARCHAR are effectively the same — no migration needed
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return nil
	}

	tableName := "tokens"
	columnName := "model_limits"

	if !DB.Migrator().HasTable(tableName) {
		return nil
	}

	if !DB.Migrator().HasColumn(&Token{}, columnName) {
		return nil
	}

	var alterSQL string
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if dataType == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE text`, tableName, columnName)
	} else if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		var columnType string
		if err := DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if strings.ToLower(columnType) == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s text", tableName, columnName)
	} else {
		return nil
	}

	if alterSQL != "" {
		if err := DB.Exec(alterSQL).Error; err != nil {
			return fmt.Errorf("failed to migrate %s.%s to text: %w", tableName, columnName, err)
		}
		common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to text", tableName, columnName))
	}
	return nil
}

// migrateSubscriptionPlanPriceAmount migrates price_amount column from float/double to decimal(10,6)
// This is safe to run multiple times - it checks the column type first
func migrateSubscriptionPlanPriceAmount() {
	// SQLite doesn't support ALTER COLUMN, and its type affinity handles this automatically
	// Skip early to avoid GORM parsing the existing table DDL which may cause issues
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return
	}

	tableName := "subscription_plans"
	columnName := "price_amount"

	// Check if table exists first
	if !DB.Migrator().HasTable(tableName) {
		return
	}

	// Check if column exists
	if !DB.Migrator().HasColumn(&SubscriptionPlan{}, columnName) {
		return
	}

	var alterSQL string
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		// PostgreSQL: Check if already decimal/numeric
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if dataType == "numeric" {
			return // Already decimal/numeric
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE decimal(10,6) USING %s::decimal(10,6)`,
			tableName, columnName, columnName)
	} else if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		// MySQL: Check if already decimal
		var columnType string
		if err := DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if strings.HasPrefix(strings.ToLower(columnType), "decimal") {
			return // Already decimal
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s decimal(10,6) NOT NULL DEFAULT 0",
			tableName, columnName)
	} else {
		return
	}

	if alterSQL != "" {
		if err := DB.Exec(alterSQL).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to migrate %s.%s to decimal: %v", tableName, columnName, err))
		} else {
			common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to decimal(10,6)", tableName, columnName))
		}
	}
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}

func CloseDB() error {
	if LOG_DB != DB {
		err := closeDB(LOG_DB)
		if err != nil {
			return err
		}
	}
	return closeDB(DB)
}

// checkMySQLChineseSupport ensures the MySQL connection and current schema
// default charset/collation can store Chinese characters. It allows common
// Chinese-capable charsets (utf8mb4, utf8, gbk, big5, gb18030) and panics otherwise.
func checkMySQLChineseSupport(db *gorm.DB) error {
	// 仅检测：当前库默认字符集/排序规则 + 各表的排序规则（隐含字符集）

	// Read current schema defaults
	var schemaCharset, schemaCollation string
	err := db.Raw("SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = DATABASE()").Row().Scan(&schemaCharset, &schemaCollation)
	if err != nil {
		return fmt.Errorf("读取当前库默认字符集/排序规则失败 / Failed to read schema default charset/collation: %v", err)
	}

	toLower := func(s string) string { return strings.ToLower(s) }
	// Allowed charsets that can store Chinese text
	allowedCharsets := map[string]string{
		"utf8mb4": "utf8mb4_",
		"utf8":    "utf8_",
		"gbk":     "gbk_",
		"big5":    "big5_",
		"gb18030": "gb18030_",
	}
	isChineseCapable := func(cs, cl string) bool {
		csLower := toLower(cs)
		clLower := toLower(cl)
		if prefix, ok := allowedCharsets[csLower]; ok {
			if clLower == "" {
				return true
			}
			return strings.HasPrefix(clLower, prefix)
		}
		// 如果仅提供了排序规则，尝试按排序规则前缀判断
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(clLower, prefix) {
				return true
			}
		}
		return false
	}

	// 1) 当前库默认值必须支持中文
	if !isChineseCapable(schemaCharset, schemaCollation) {
		return fmt.Errorf("当前库默认字符集/排序规则不支持中文：schema(%s/%s)。请将库设置为 utf8mb4/utf8/gbk/big5/gb18030 / Schema default charset/collation is not Chinese-capable: schema(%s/%s). Please set to utf8mb4/utf8/gbk/big5/gb18030",
			schemaCharset, schemaCollation, schemaCharset, schemaCollation)
	}

	// 2) 所有物理表的排序规则（隐含字符集）必须支持中文
	type tableInfo struct {
		Name      string
		Collation *string
	}
	var tables []tableInfo
	if err := db.Raw("SELECT TABLE_NAME, TABLE_COLLATION FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'").Scan(&tables).Error; err != nil {
		return fmt.Errorf("读取表排序规则失败 / Failed to read table collations: %v", err)
	}

	var badTables []string
	for _, t := range tables {
		// NULL 或空表示继承库默认设置，已在上面校验库默认，视为通过
		if t.Collation == nil || *t.Collation == "" {
			continue
		}
		cl := *t.Collation
		// 仅凭排序规则判断是否中文可用
		ok := false
		lower := strings.ToLower(cl)
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(lower, prefix) {
				ok = true
				break
			}
		}
		if !ok {
			badTables = append(badTables, fmt.Sprintf("%s(%s)", t.Name, cl))
		}
	}

	if len(badTables) > 0 {
		// 限制输出数量以避免日志过长
		maxShow := 20
		shown := badTables
		if len(shown) > maxShow {
			shown = shown[:maxShow]
		}
		return fmt.Errorf(
			"存在不支持中文的表，请修复其排序规则/字符集。示例（最多展示 %d 项）：%v / Found tables not Chinese-capable. Please fix their collation/charset. Examples (showing up to %d): %v",
			maxShow, shown, maxShow, shown,
		)
	}
	return nil
}

var (
	lastPingTime time.Time
	pingMutex    sync.Mutex
)

func PingDB() error {
	pingMutex.Lock()
	defer pingMutex.Unlock()

	if time.Since(lastPingTime) < time.Second*10 {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Printf("Error getting sql.DB from GORM: %v", err)
		return err
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Printf("Error pinging DB: %v", err)
		return err
	}

	lastPingTime = time.Now()
	common.SysLog("Database pinged successfully")
	return nil
}
