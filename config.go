package main

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	ProjectsDir string
	ProxyPort   int
	DNSPort     int
	PortmapFile string
	PidFile     string
}

func loadConfig() Config {
	dir := os.Getenv("DOT_TEST_DIR")
	if dir == "" {
		dir, _ = os.Getwd()
	}

	proxyPort := 80
	if p := os.Getenv("DOT_TEST_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			proxyPort = v
		}
	}

	dnsPort := 15353
	if p := os.Getenv("DOT_TEST_DNS_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			dnsPort = v
		}
	}

	return Config{
		ProjectsDir: dir,
		ProxyPort:   proxyPort,
		DNSPort:     dnsPort,
		PortmapFile: filepath.Join(dir, ".dot-test"),
		PidFile:     filepath.Join(dir, ".dot-test-pid"),
	}
}
