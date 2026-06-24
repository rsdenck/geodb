package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oschwald/maxminddb-golang/v2"
)

type PGStore struct {
	pool *pgxpool.Pool
}

func NewPGStore() *PGStore {
	dsn := os.Getenv("GEOIP_DSN")
	if dsn == "" {
		dsn = "postgres://geoip:geoip123@127.0.0.1:5432/geoip"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERRO conectando PostgreSQL: %v\n", err)
		os.Exit(1)
	}

	if err := pool.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ERRO ping PostgreSQL: %v\n", err)
		os.Exit(1)
	}

	return &PGStore{pool: pool}
}

func (s *PGStore) Close() {
	s.pool.Close()
}

func lastAddr(p netip.Prefix) netip.Addr {
	addr := p.Addr()
	ones := p.Bits()
	bytes := addr.AsSlice()
	if len(bytes) == 4 {
		mask := net.CIDRMask(ones, 32)
		network := addr.As4()
		for i := range network {
			network[i] |= ^mask[i]
		}
		return netip.AddrFrom4(network)
	}
	mask := net.CIDRMask(ones, 128)
	network := addr.As16()
	for i := range network {
		network[i] |= ^mask[i]
	}
	return netip.AddrFrom16(network)
}

func migrateGeoIP() {
	db := NewPGStore()
	defer db.Close()

	ctx := context.Background()
	fmt.Println("=== POPULANDO TABELAS ===")

	importGeoCity(db, filepath.Join(geoDBPath(), "city.mmdb"))
	importGeoASN(db, filepath.Join(geoDBPath(), "asn.mmdb"))
	importRangeStore(db)
	importRIPEASNs(db)

	fmt.Println("\n=== VERIFICANDO ===")
	tables := []string{"asn", "asn_org", "ip_prefix", "asn_prefix_map", "ip_geo", "asn_geo"}
	for _, t := range tables {
		var count int
		db.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", t)).Scan(&count)
		fmt.Printf("  %s: %d registros\n", t, count)
	}
	fmt.Println("=== DATABASE PRONTO ===")
}

func importGeoCity(db *PGStore, path string) {
	fmt.Printf("City geo: %s\n", path)

	reader, err := maxminddb.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERRO abrindo %s: %v\n", path, err)
		return
	}
	defer reader.Close()

	batchSize := 500
	asnBatch := make([]struct {
		asn  uint
		org  string
		cc   string
	}, 0, batchSize)
	geoBatch := make([][]any, 0, batchSize)
	prefixBatch := make([][]any, 0, batchSize)
	count := 0
	start := time.Now()

	seenASN := map[uint]bool{}
	seenPrefix := map[string]bool{}

	for result := range reader.Networks() {
		prefix := result.Prefix()
		network := prefix.String()

		var record struct {
			City struct {
				Names map[string]string `maxminddb:"names"`
			} `maxminddb:"city"`
			Subdivisions []struct {
				Names   map[string]string `maxminddb:"names"`
				IsoCode string            `maxminddb:"iso_code"`
			} `maxminddb:"subdivisions"`
			Country struct {
				Names   map[string]string `maxminddb:"names"`
				IsoCode string            `maxminddb:"iso_code"`
			} `maxminddb:"country"`
			Location struct {
				Latitude  float64 `maxminddb:"latitude"`
				Longitude float64 `maxminddb:"longitude"`
			} `maxminddb:"location"`
			Postal struct {
				Code string `maxminddb:"code"`
			} `maxminddb:"postal"`
		}

		if err := result.Decode(&record); err != nil {
			continue
		}

		country := record.Country.Names["en"]
		if country == "" {
			continue
		}
		countryCode := record.Country.IsoCode
		city := record.City.Names["en"]
		region := ""
		if len(record.Subdivisions) > 0 {
			region = record.Subdivisions[0].Names["en"]
		}
		lat := record.Location.Latitude
		lon := record.Location.Longitude
		postal := record.Postal.Code
		version := 4
		if prefix.Addr().Is6() {
			version = 6
		}

		if !seenPrefix[network] {
			seenPrefix[network] = true
			prefixBatch = append(prefixBatch, []any{
				network, version, nil, countryCode, nil,
			})
			geoBatch = append(geoBatch, []any{
				network, countryCode, region, city, lat, lon, postal, "",
			})
		}

		count++

		if len(prefixBatch) >= batchSize {
			bulkInsertIPPrefix(db, prefixBatch)
			bulkInsertIPGeo(db, geoBatch)
			prefixBatch = prefixBatch[:0]
			geoBatch = geoBatch[:0]
		}

		if count%100000 == 0 {
			fmt.Printf("  City: %d em %v\n", count, time.Since(start).Round(time.Second))
		}
	}

	if len(prefixBatch) > 0 {
		bulkInsertIPPrefix(db, prefixBatch)
		bulkInsertIPGeo(db, geoBatch)
	}

	_ = asnBatch
	_ = seenASN
	fmt.Printf("  City geo concluido: %d registros em %v\n", count, time.Since(start).Round(time.Second))
}

func importGeoASN(db *PGStore, path string) {
	fmt.Printf("ASN geo: %s\n", path)

	reader, err := maxminddb.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERRO abrindo %s: %v\n", path, err)
		return
	}
	defer reader.Close()

	batchSize := 500
	asnBatch := make([][]any, 0, batchSize)
	geoBatch := make([][]any, 0, batchSize)
	ipPrefixBatch := make([][]any, 0, batchSize)
	asnPrefixBatch := make([][]any, 0, batchSize)
	count := 0
	start := time.Now()

	for result := range reader.Networks() {
		prefix := result.Prefix()
		network := prefix.String()

		var record struct {
			ASN   uint   `maxminddb:"autonomous_system_number"`
			ASOrg string `maxminddb:"autonomous_system_organization"`
		}

		if err := result.Decode(&record); err != nil {
			continue
		}

		asn := record.ASN
		org := record.ASOrg
		version := 4
		if prefix.Addr().Is6() {
			version = 6
		}

		var asnPtr any
		if asn > 0 {
			asnPtr = asn
			asnBatch = append(asnBatch, []any{asn, org, nil, nil})
			geoBatch = append(geoBatch, []any{asn, nil, org})
			asnPrefixBatch = append(asnPrefixBatch, []any{asn, network})
		}
		ipPrefixBatch = append(ipPrefixBatch, []any{network, version, asnPtr, nil, nil})
		count++

		if len(asnBatch) >= batchSize {
			bulkInsertASN(db, asnBatch)
			bulkInsertASNGeo(db, geoBatch)
			bulkInsertIPPrefix(db, ipPrefixBatch)
			bulkInsertASNPrefixMap(db, asnPrefixBatch)
			asnBatch = asnBatch[:0]
			geoBatch = geoBatch[:0]
			ipPrefixBatch = ipPrefixBatch[:0]
			asnPrefixBatch = asnPrefixBatch[:0]
		}

		if count%100000 == 0 {
			fmt.Printf("  ASN: %d em %v\n", count, time.Since(start).Round(time.Second))
		}
	}

	if len(asnBatch) > 0 {
		bulkInsertASN(db, asnBatch)
		bulkInsertASNGeo(db, geoBatch)
		bulkInsertIPPrefix(db, ipPrefixBatch)
		bulkInsertASNPrefixMap(db, asnPrefixBatch)
	}

	fmt.Printf("  ASN geo concluido: %d registros em %v\n", count, time.Since(start).Round(time.Second))
}

func importRangeStore(db *PGStore) {
	store := NewRangeStore()
	ranges := store.List()
	fmt.Printf("Range store: %d ranges\n", len(ranges))

	batchSize := 5000
	asnSeen := map[uint]bool{}
	var allASNs [][]any

	for _, r := range ranges {
		if r.ASN > 0 && !asnSeen[r.ASN] {
			asnSeen[r.ASN] = true
			allASNs = append(allASNs, []any{r.ASN, r.ASOrg, nil, nil})
		}
	}

	for i := 0; i < len(allASNs); i += batchSize {
		end := i + batchSize
		if end > len(allASNs) {
			end = len(allASNs)
		}
		bulkInsertASN(db, allASNs[i:end])
	}
	fmt.Printf("  ASNs: %d\n", len(allASNs))

	ipBatch := make([][]any, 0, batchSize)
	asnPrefixBatch := make([][]any, 0, batchSize)
	for _, r := range ranges {
		network := rangeToCIDR(r.Network)
		version := 4
		if strings.Contains(network, ":") {
			version = 6
		}
		var asnPtr any
		if r.ASN > 0 {
			asnPtr = r.ASN
		}
		ipBatch = append(ipBatch, []any{network, version, asnPtr, nil, nil})
		if r.ASN > 0 {
			asnPrefixBatch = append(asnPrefixBatch, []any{r.ASN, network})
		}

		if len(ipBatch) >= batchSize {
			bulkInsertIPPrefix(db, ipBatch)
			if len(asnPrefixBatch) > 0 {
				bulkInsertASNPrefixMap(db, asnPrefixBatch)
			}
			ipBatch = ipBatch[:0]
			asnPrefixBatch = asnPrefixBatch[:0]
		}
	}
	if len(ipBatch) > 0 {
		bulkInsertIPPrefix(db, ipBatch)
		bulkInsertASNPrefixMap(db, asnPrefixBatch)
	}
	fmt.Printf("  Prefixos: %d\n", len(ranges))
}

func importRIPEASNs(db *PGStore) {
	store := NewRangeStore()
	ranges := store.List()

	asnSeen := map[uint]bool{}
	asnBatch := make([][]any, 0, 500)
	geoBatch := make([][]any, 0, 500)

	for _, r := range ranges {
		if r.ASN > 0 && !asnSeen[r.ASN] {
			asnSeen[r.ASN] = true
			asnBatch = append(asnBatch, []any{r.ASN, r.ASOrg, nil, nil})
			geoBatch = append(geoBatch, []any{r.ASN, nil, r.ASOrg})
		}
	}

	if len(asnBatch) > 0 {
		bulkInsertASN(db, asnBatch)
		bulkInsertASNGeo(db, geoBatch)
	}
}

func bulkInsertASN(db *PGStore, batch [][]any) {
	if len(batch) == 0 {
		return
	}
	// PostgreSQL protocol max 65535 params, with 4 cols = max ~16383 rows per batch
	const maxParams = 60000
	paramsPerRow := 4
	maxRows := maxParams / paramsPerRow

	for start := 0; start < len(batch); start += maxRows {
		end := start + maxRows
		if end > len(batch) {
			end = len(batch)
		}
		chunk := batch[start:end]
		vals := make([]string, len(chunk))
		args := make([]any, 0, len(chunk)*paramsPerRow)
		for i, r := range chunk {
			base := i * paramsPerRow
			vals[i] = fmt.Sprintf("($%d,$%d,$%d,$%d)", base+1, base+2, base+3, base+4)
			args = append(args, r[0], r[1], r[2], r[3])
		}
		q := fmt.Sprintf("INSERT INTO asn (asn,name,country,rir) VALUES %s ON CONFLICT DO NOTHING", strings.Join(vals, ","))
		_, err := db.pool.Exec(context.Background(), q, args...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERRO batch ASN: %v\n", err)
		}
	}
}

func bulkInsertIPPrefix(db *PGStore, batch [][]any) {
	if len(batch) == 0 {
		return
	}
	const maxParams = 60000
	paramsPerRow := 5
	maxRows := maxParams / paramsPerRow

	for start := 0; start < len(batch); start += maxRows {
		end := start + maxRows
		if end > len(batch) {
			end = len(batch)
		}
		chunk := batch[start:end]
		vals := make([]string, len(chunk))
		args := make([]any, 0, len(chunk)*paramsPerRow)
		for i, r := range chunk {
			base := i * paramsPerRow
			vals[i] = fmt.Sprintf("($%d::cidr,$%d,$%d,$%d,$%d)", base+1, base+2, base+3, base+4, base+5)
			args = append(args, r[0], r[1], r[2], r[3], r[4])
		}
		q := fmt.Sprintf("INSERT INTO ip_prefix (prefix,version,asn,country,rir) VALUES %s ON CONFLICT DO NOTHING", strings.Join(vals, ","))
		_, err := db.pool.Exec(context.Background(), q, args...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERRO batch IP prefix: %v\n", err)
		}
	}
}

func bulkInsertASNPrefixMap(db *PGStore, batch [][]any) {
	if len(batch) == 0 {
		return
	}
	const maxParams = 60000
	paramsPerRow := 2
	maxRows := maxParams / paramsPerRow

	for start := 0; start < len(batch); start += maxRows {
		end := start + maxRows
		if end > len(batch) {
			end = len(batch)
		}
		chunk := batch[start:end]
		vals := make([]string, len(chunk))
		args := make([]any, 0, len(chunk)*paramsPerRow)
		for i, r := range chunk {
			base := i * paramsPerRow
			vals[i] = fmt.Sprintf("($%d,$%d::cidr)", base+1, base+2)
			args = append(args, r[0], r[1])
		}
		q := fmt.Sprintf("INSERT INTO asn_prefix_map (asn,prefix) VALUES %s ON CONFLICT DO NOTHING", strings.Join(vals, ","))
		_, err := db.pool.Exec(context.Background(), q, args...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERRO batch ASN prefix map: %v\n", err)
		}
	}
}

func bulkInsertIPGeo(db *PGStore, batch [][]any) {
	if len(batch) == 0 {
		return
	}
	const maxParams = 60000
	paramsPerRow := 8
	maxRows := maxParams / paramsPerRow

	for start := 0; start < len(batch); start += maxRows {
		end := start + maxRows
		if end > len(batch) {
			end = len(batch)
		}
		chunk := batch[start:end]
		vals := make([]string, len(chunk))
		args := make([]any, 0, len(chunk)*paramsPerRow)
		for i, r := range chunk {
			base := i * paramsPerRow
			vals[i] = fmt.Sprintf("($%d::cidr,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
				base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8)
			args = append(args, r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7])
		}
		q := fmt.Sprintf("INSERT INTO ip_geo (prefix,country,region,city,latitude,longitude,postal,timezone) VALUES %s ON CONFLICT DO NOTHING",
			strings.Join(vals, ","))
		_, err := db.pool.Exec(context.Background(), q, args...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERRO batch IP geo: %v\n", err)
		}
	}
}

func bulkInsertASNGeo(db *PGStore, batch [][]any) {
	if len(batch) == 0 {
		return
	}
	const maxParams = 60000
	paramsPerRow := 3
	maxRows := maxParams / paramsPerRow

	for start := 0; start < len(batch); start += maxRows {
		end := start + maxRows
		if end > len(batch) {
			end = len(batch)
		}
		chunk := batch[start:end]
		vals := make([]string, len(chunk))
		args := make([]any, 0, len(chunk)*paramsPerRow)
		for i, r := range chunk {
			base := i * paramsPerRow
			vals[i] = fmt.Sprintf("($%d,$%d,$%d)", base+1, base+2, base+3)
			args = append(args, r[0], r[1], r[2])
		}
		q := fmt.Sprintf("INSERT INTO asn_geo (asn,country,org) VALUES %s ON CONFLICT DO NOTHING", strings.Join(vals, ","))
		_, err := db.pool.Exec(context.Background(), q, args...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERRO batch ASN geo: %v\n", err)
		}
	}
}

// unused but keep for reference
var _ = binary.BigEndian
