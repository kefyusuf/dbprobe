package mysql

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

type Config struct {
	DriverDSN   string
	Host        string
	Port        string
	Database    string
	DisplayName string
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
		DriverDSN:   driverDSN,
		Host:        host,
		Port:        port,
		Database:    dbName,
		DisplayName: addr + "/" + dbName,
	}, nil
}
