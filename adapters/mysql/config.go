package mysql

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

type Config struct {
	Host        string
	Port        string
	Database    string
	DisplayName string

	driverConfig *mysqldriver.Config
	driverDSN    string
	rawTarget    string
	password     string
}

func ParseConfig(raw string) (Config, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "mysql" {
		return Config{}, fmt.Errorf("invalid MySQL target")
	}
	host := u.Hostname()
	if host == "" {
		return Config{}, fmt.Errorf("MySQL target requires a host")
	}
	port := u.Port()
	if port == "" {
		port = "3306"
	}
	dbName, err := url.PathUnescape(strings.TrimPrefix(u.EscapedPath(), "/"))
	if err != nil || dbName == "" {
		return Config{}, fmt.Errorf("MySQL target requires a database")
	}

	user := ""
	password := ""
	if u.User != nil {
		user = u.User.Username()
		password, _ = u.User.Password()
	}

	addr := net.JoinHostPort(host, port)
	driverCfg := mysqldriver.NewConfig()
	driverCfg.User = user
	driverCfg.Passwd = password
	driverCfg.Net = "tcp"
	driverCfg.Addr = addr
	driverCfg.DBName = dbName
	driverCfg.Timeout = 5 * time.Second
	driverCfg.ReadTimeout = 10 * time.Second
	driverCfg.WriteTimeout = 10 * time.Second

	canonical := driverCfg.FormatDSN()
	if u.RawQuery != "" {
		canonical += "?" + u.RawQuery
	}
	parsed, err := mysqldriver.ParseDSN(canonical)
	if err != nil {
		return Config{}, fmt.Errorf("invalid MySQL connection options")
	}

	return Config{
		Host:         host,
		Port:         port,
		Database:     dbName,
		DisplayName:  addr + "/" + dbName,
		driverConfig: parsed,
		driverDSN:    parsed.FormatDSN(),
		rawTarget:    raw,
		password:     password,
	}, nil
}

func sanitizeError(err error, cfg Config) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if cfg.rawTarget != "" {
		message = strings.ReplaceAll(message, cfg.rawTarget, "<redacted-target>")
	}
	if cfg.driverDSN != "" {
		message = strings.ReplaceAll(message, cfg.driverDSN, "<redacted-dsn>")
	}
	if cfg.password != "" {
		message = strings.ReplaceAll(message, cfg.password, "<redacted>")
	}
	return fmt.Errorf("%s", message)
}
