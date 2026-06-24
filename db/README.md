# db/ - Database Schema

## Files

| File | Description |
|------|-------------|
| `geoip.sql` | Full PostgreSQL schema: 20 tables, 11 hypertables, indexes, foreign keys |

## How to Apply the Schema

### Create a fresh database

```bash
createdb -U geoip geoip
psql -U geoip -d geoip -f db/geoip.sql
```

### Add TimescaleDB extensions

The schema includes `CREATE EXTENSION IF NOT EXISTS timescaledb`. Ensure the extension is installed:

```bash
# As superuser
psql -U postgres -d geoip -c "CREATE EXTENSION IF NOT EXISTS timescaledb"
```

## Schema Overview

### Layer 1: ASN
- `asn` - ASN registry (PK: asn)
- `asn_org` - ASN organizations
- `asn_contact` - ASN abuse/email contacts
- `asn_geo` - ASN geolocation

### Layer 2: IP Prefix
- `ip_prefix` - All IP prefixes with ASN/country/RIR
- `prefix_history` - Prefix change history (hypertable)
- `asn_prefix_map` - ASN-to-prefix mapping (PK: asn+prefix)
- `asn_prefix_history` - ASN-prefix change history (hypertable)

### Layer 3: RIR
- `rir_allocation` - RIR allocation records
- `rir_assignment_history` - RIR assignment history (hypertable)

### Layer 4: Geolocation
- `ip_geo` - IP geolocation data (MaxMind)
- `asn_geo` - ASN geolocation data (MaxMind)

### Layer 5: BGP
- `bgp_update` - BGP route updates (hypertable)
- `bgp_rib_snapshot` - BGP RIB snapshots (hypertable)
- `bgp_as_path` - AS path records
- `bgp_event` - BGP events (hypertable)

### Layer 6: Routing Anomalies
- `routing_anomaly` - Routing anomaly detections (hypertable)
- `prefix_flap` - Prefix flap events (hypertable)

### Layer 7: DNS
- `dns_resolution` - Historical DNS resolutions (hypertable)
- `rdns_history` - Reverse DNS history (hypertable)

### Layer 8: Reputation
- `reputation_score` - Prefix reputation scores (hypertable)

## Current Data Population

| Table | Records |
|-------|--------:|
| asn | 85,748 |
| ip_prefix | 6,744,521 |
| asn_prefix_map | 1,591,878 |
| ip_geo | 5,863,490 |
| asn_geo | 85,748 |

Empty tables (awaiting data feeds): asn_org, asn_contact, prefix_history, asn_prefix_history, rir_allocation, rir_assignment_history, all BGP tables, all DNS tables, reputation_score.
