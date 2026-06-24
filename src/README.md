# src/ - Go Source Code

## Files

| File | Purpose |
|------|---------|
| `main.go` | Entrypoint: schema creation + data import orchestration |
| `schema.go` | 20 CREATE TABLE statements, 11 hypertables, 8 indexes |
| `migrate.go` | Bulk data import: MaxMind City, MaxMind ASN, RIPE range store |
| `scraper.go` | RIPEstat HTTP scraper: country prefixes, ASN prefixes, ASN mapping |
| `mmdb.go` | MaxMind MMDB lookup engine (legacy, kept for future HTTP API) |
| `range.go` | Range store, CIDR normalization, prefix parsing |
| `contexto.md` | Full project context for LLM continuation |

## Build

```bash
go build -o ../bin/geoip .
```

## Dependencies

- Go 1.26+
- `github.com/jackc/pgx/v5` - PostgreSQL driver
- `github.com/oschwald/maxminddb-golang/v2` - MaxMind MMDB reader

## Environment Variables

- `GEOIP_DSN` - PostgreSQL connection string (default: `postgres://geoip:geoip123@127.0.0.1:5432/geoip`)
- `GEOIP_DATA` - Data directory path (default: `./data`)

## Data Sources

- `data/GeoLite2-City.mmdb` - MaxMind City database
- `data/GeoLite2-ASN.mmdb` - MaxMind ASN database
- `data/ranges.json` - RIPE full IP space scan
- `data/br_asns.json` - Brazilian ASN list
