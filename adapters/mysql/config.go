package mysql

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

var allowedConnectionOptions = map[string]struct{}{
	"tls":          {},
	"timeout":      {},
	"readTimeout":  {},
	"writeTimeout": {},
}

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

	query := u.Query()
	safeQuery := url.Values{}
	for key, values := range query {
		if _, allowed := allowedConnectionOptions[key]; !allowed {
			return Config{}, fmt.Errorf("unsupported MySQL connection option")
		}
		if len(values) != 1 {
			return Config{}, fmt.Errorf("MySQL connection option must be specified once")
		}
		safeQuery.Set(key, values[0])
	}
	if err := validateTLSMode(host, query); err != nil {
		return Config{}, err
	}

	addr := net.JoinHostPort(host, port)
	base := mysqldriver.NewConfig()
	base.User = user
	base.Passwd = password
	base.Net = "tcp"
	base.Addr = addr
	base.DBName = dbName

	canonical := base.FormatDSN()
	if encoded := safeQuery.Encode(); encoded != "" {
		canonical += "?" + encoded
	}
	parsed, err := mysqldriver.ParseDSN(canonical)
	if err != nil {
		return Config{}, fmt.Errorf("invalid MySQL connection options")
	}
	if parsed.Timeout == 0 {
		parsed.Timeout = 5 * time.Second
	}
	if parsed.ReadTimeout == 0 {
		parsed.ReadTimeout = 10 * time.Second
	}
	if parsed.WriteTimeout == 0 {
		parsed.WriteTimeout = 10 * time.Second
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

func validateTLSMode(host string, query url.Values) error {
	values, ok := query["tls"]
	if !ok || len(values) != 1 || strings.TrimSpace(values[0]) == "" {
		return fmt.Errorf("MySQL target requires an explicit tls mode")
	}

	switch strings.ToLower(strings.TrimSpace(values[0])) {
	case "true":
		return nil
	case "false":
		if isLoopbackHost(host) {
			return nil
		}
		return fmt.Errorf("remote MySQL target requires tls=true")
	default:
		return fmt.Errorf("MySQL tls mode must be true, or false for loopback targets")
	}
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
