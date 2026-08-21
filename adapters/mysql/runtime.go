package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	mysqlcollectors "github.com/kefyusuf/dbprobe/adapters/mysql/collectors"
	mysqlfindings "github.com/kefyusuf/dbprobe/adapters/mysql/findings"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/finding"
)

const serverIdentityQuery = `SELECT
	VERSION(),
	@@version_comment,
	COALESCE(DATABASE(), ''),
	COALESCE(@@server_uuid, ''),
	IF(@@performance_schema, 1, 0),
	IF(@@read_only, 1, 0),
	IF(@@super_read_only, 1, 0)`

type serverIdentity struct {
	Version           string
	VersionComment    string
	Database          string
	ServerUUID        string
	PerformanceSchema bool
	ReadOnly          bool
	SuperReadOnly     bool
}

type runtime struct {
	db       *sql.DB
	database string
	target   adapter.TargetMetadata
	caps     capability.Set
	security adapter.SecurityProfile

	closeOnce sync.Once
	closeErr  error
}

func openRuntime(ctx context.Context, cfg Config) (*runtime, error) {
	driverCfg := cfg.driverConfig.Clone()
	connector, err := mysqldriver.NewConnector(driverCfg)
	if err != nil {
		return nil, sanitizeError(err, cfg)
	}

	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(4 * time.Minute)
	db.SetConnMaxIdleTime(time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, sanitizeError(err, cfg)
	}

	identity, err := readServerIdentity(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, sanitizeError(err, cfg)
	}
	if identity.Database == "" {
		identity.Database = cfg.Database
	}

	caps := discoverCapabilities(ctx, identity.PerformanceSchema, func(ctx context.Context, query string) error {
		return queryProbe(ctx, db, query)
	})

	return &runtime{
		db:       db,
		database: identity.Database,
		target:   buildTarget(identity, cfg),
		caps:     caps,
		security: adapter.SecurityProfile{
			ReadOnlyGuaranteed: true,
			Required: []adapter.Privilege{{
				Name:   "SELECT",
				Scope:  "performance_schema, information_schema, sys (optional), target metadata",
				Reason: "dbprobe only reads diagnostic metadata and telemetry",
			}},
			Recommended: []adapter.Privilege{{
				Name:   "PROCESS",
				Scope:  "server",
				Reason: "improves visibility into sessions on restricted installations",
			}},
		},
	}, nil
}

func readServerIdentity(ctx context.Context, db *sql.DB) (serverIdentity, error) {
	var identity serverIdentity
	var performanceSchema, readOnly, superReadOnly int
	err := db.QueryRowContext(ctx, serverIdentityQuery).Scan(
		&identity.Version,
		&identity.VersionComment,
		&identity.Database,
		&identity.ServerUUID,
		&performanceSchema,
		&readOnly,
		&superReadOnly,
	)
	if err != nil {
		return serverIdentity{}, fmt.Errorf("read MySQL server identity: %w", err)
	}
	identity.PerformanceSchema = performanceSchema != 0
	identity.ReadOnly = readOnly != 0
	identity.SuperReadOnly = superReadOnly != 0
	return identity, nil
}

func queryProbe(ctx context.Context, db *sql.DB, query string) error {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		break
	}
	return rows.Err()
}

func buildTarget(identity serverIdentity, cfg Config) adapter.TargetMetadata {
	return adapter.TargetMetadata{
		Engine:      "mysql",
		AdapterID:   "mysql",
		Fingerprint: fingerprint(identity.ServerUUID, identity.Database, cfg.Host, cfg.Port),
		DisplayName: cfg.DisplayName,
	}
}

func fingerprint(serverUUID, database, host, port string) string {
	seed := host + "|" + port + "|" + database
	if serverUUID != "" {
		seed = serverUUID + "|" + database
	}
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:8])
}

func (r *runtime) Target() adapter.TargetMetadata { return r.target }
func (r *runtime) Capabilities() capability.Set   { return r.caps }
func (r *runtime) Collectors() []collector.Collector {
	queryer := mysqlcollectors.NewSQLQueryer(r.db)
	collectors := mysqlcollectors.NewHealth(queryer, r.target.Fingerprint)
	collectors = append(collectors,
		mysqlcollectors.NewQueries(queryer, r.database, 20),
		mysqlcollectors.NewIndexes(queryer, r.database, 100),
		mysqlcollectors.NewTables(queryer, r.database, 100),
		mysqlcollectors.NewTransactions(queryer, 50),
		mysqlcollectors.NewLocks(queryer, r.target.Fingerprint, 100),
		mysqlcollectors.NewReplication(queryer, 100),
	)
	collectors = append(collectors, mysqlcollectors.NewSchemaRisk(queryer, r.database, 100)...)
	return collectors
}
func (r *runtime) Rules() []finding.Rule {
	rules := mysqlfindings.Rules()
	rules = append(rules, mysqlfindings.RiskRules()...)
	rules = append(rules, mysqlfindings.QueryTimeRules()...)
	return append(rules, mysqlfindings.LockWaitRules()...)
}
func (r *runtime) SecurityProfile() adapter.SecurityProfile {
	return r.security
}
func (r *runtime) Close() error {
	r.closeOnce.Do(func() {
		if r.db != nil {
			r.closeErr = r.db.Close()
		}
	})
	return r.closeErr
}
