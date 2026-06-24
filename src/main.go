package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("  INTERNET GLOBAL MAPPING DATABASE")
	fmt.Println("  TimescaleDB + PostgreSQL")
	fmt.Println(strings.Repeat("=", 72))

	start := time.Now()

	runSchema()
	migrateGeoIP()

	fmt.Printf("\nDuracao total: %v\n", time.Since(start).Round(time.Second))
	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("  DATABASE PRONTO!")
	fmt.Println(strings.Repeat("=", 72))
}

func geoDBPath() string {
	dir := os.Getenv("GEOIP_DATA")
	if dir == "" {
		dir = filepath.Join(".", "data")
	}
	return dir
}
