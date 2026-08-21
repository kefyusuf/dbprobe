package mysql

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

type Config struct {
	Host        string
	Port        string
	Database    string
	DisplayName string

	driverDSN string
	rawTarget string
	password  string
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

	prefix := ""
	if user != "" {
		prefix = user
		if password != "" {
			prefix += ":" + password
		}
		prefix += "@"
	}
	addr := net.JoinHostPort(host, port)
	driverDSN := prefix + "tcp(" + addr + ")/" + url.PathEscape(dbName)
	if u.RawQuery != "" {
		driverDSN += "?" + u.RawQuery
	}

	return Config{
		Host:        host,
		Port:        port,
		Database:    dbName,
		DisplayName: addr + "/" + dbName,
		driverDSN:   driverDSN,
		rawTarget:   raw,
		password:    password,
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
