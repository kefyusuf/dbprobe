package main

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"strings"
)

const maxMySQLPasswordBytes = 4096

func resolveCommandTarget(raw string, passwordStdin bool, in io.Reader) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return raw, nil
	}
	if u.Scheme != "mysql" {
		if passwordStdin {
			return "", fmt.Errorf("--password-stdin is only supported for MySQL targets")
		}
		return raw, nil
	}
	if u.User != nil {
		if _, present := u.User.Password(); present {
			return "", fmt.Errorf("MySQL password must not be included in target URL; use --password-stdin")
		}
	}
	if !passwordStdin {
		return raw, nil
	}
	if u.User == nil || strings.TrimSpace(u.User.Username()) == "" {
		return "", fmt.Errorf("MySQL username is required with --password-stdin")
	}
	if in == nil {
		return "", fmt.Errorf("MySQL password input is required")
	}

	password, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read MySQL password from stdin")
	}
	password = strings.TrimSuffix(password, "\n")
	password = strings.TrimSuffix(password, "\r")
	if password == "" {
		return "", fmt.Errorf("MySQL password from stdin is empty")
	}
	if len(password) > maxMySQLPasswordBytes {
		return "", fmt.Errorf("MySQL password from stdin exceeds %d bytes", maxMySQLPasswordBytes)
	}

	u.User = url.UserPassword(u.User.Username(), password)
	return u.String(), nil
}
