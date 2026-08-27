package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	Address         string
	DatabasePath    string
	Selfcheck       bool
	ShutdownTimeout time.Duration
}

func parseConfig(args []string) (config, error) {
	set := flag.NewFlagSet("cave-sampling-permit", flag.ContinueOnError)
	address := set.String("addr", addressFromEnvironment(), "HTTP 监听地址")
	database := set.String("db", "cave-sampling-permit.db", "SQLite 数据库路径")
	selfcheck := set.Bool("selfcheck", false, "执行真实回环完整流程后退出")
	shutdown := set.Duration("shutdown-timeout", 5*time.Second, "关闭等待时间")
	if err := set.Parse(args); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(set.Args(), " "))
	}
	if err := validateAddress(*address); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*database) == "" {
		return config{}, fmt.Errorf("-db cannot be empty")
	}
	if *shutdown <= 0 || *shutdown > time.Minute {
		return config{}, fmt.Errorf("-shutdown-timeout must be between 0 and 1m")
	}
	return config{Address: *address, DatabasePath: *database, Selfcheck: *selfcheck, ShutdownTimeout: *shutdown}, nil
}

func addressFromEnvironment() string {
	portText := strings.TrimSpace(os.Getenv("PORT"))
	if portText == "" {
		return defaultAddress
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return defaultAddress
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid -addr %q: %w", address, err)
	}
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("invalid -addr %q: host is required", address)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid -addr %q: port must be 1-65535", address)
	}
	if host != "localhost" && net.ParseIP(host) == nil {
		return fmt.Errorf("invalid -addr %q: host must be an IP address or localhost", address)
	}
	return nil
}

func databasePathForRun(cfg config) string {
	if cfg.Selfcheck && cfg.DatabasePath == "cave-sampling-permit.db" {
		return ":memory:"
	}
	return cfg.DatabasePath
}
