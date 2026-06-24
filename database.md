# Internet Global Mapping Database Schema

## Layer 1: ASN Core

| Table | Type | Records | Description |
|-------|------|--------:|-------------|
| asn | standard | 85.748 | ASN registry (PK: asn, FK targets: ip_prefix, asn_prefix_map, asn_org, asn_contact) |
| asn_org | standard | 0 | ASN organizations (FK -> asn ON DELETE CASCADE) |
| asn_contact | standard | 0 | ASN contact/abuse info (FK -> asn ON DELETE CASCADE) |

Columns in `asn`:
- `asn` BIGINT PK - Autonomous System Number
- `name` TEXT - ASN name/holder
- `country` CHAR(2) - Country code
- `rir` TEXT - Regional Internet Registry (RIPE, ARIN, LACNIC, APNIC, AFRINIC)
- `created_at` TIMESTAMPTZ - First seen timestamp

## Layer 2: IP Prefix

| Table | Type | Records | Description |
|-------|------|--------:|-------------|
| ip_prefix | standard | 6.744.521 | All IP prefixes (4.4M IPv4 + 2.3M IPv6) |
| asn_prefix_map | standard | 1.591.878 | ASN-to-prefix mapping |
| prefix_history | hypertable | 0 | Prefix change history |
| asn_prefix_history | hypertable | 0 | ASN-prefix relationship history |

Columns in `ip_prefix`:
- `prefix` CIDR PK - Network prefix
- `version` SMALLINT - 4 for IPv4, 6 for IPv6
- `asn` BIGINT - Nullable FK -> asn(asn) ON DELETE SET NULL
- `country` CHAR(2) - Country code
- `rir` TEXT - RIR source
- `first_seen` TIMESTAMPTZ
- `last_seen` TIMESTAMPTZ

Columns in `asn_prefix_map`:
- `asn` BIGINT - FK -> asn(asn) ON DELETE CASCADE
- `prefix` CIDR
- `active` BOOLEAN - Whether this mapping is currently active
- `first_seen` TIMESTAMPTZ
- `last_seen` TIMESTAMPTZ

## Layer 3: RIR (Regional Internet Registry)

| Table | Type | Records | Description |
|-------|------|--------:|-------------|
| rir_allocation | standard | 0 | RIR allocation records |
| rir_assignment_history | hypertable | 0 | RIR assignment history |

## Layer 4: Geolocation

| Table | Type | Records | Description |
|-------|------|--------:|-------------|
| ip_geo | standard | 5.863.490 | IP geolocation by prefix |
| asn_geo | standard | 85.748 | ASN geolocation |

Columns in `ip_geo`:
- `prefix` CIDR PK - Network prefix
- `country` CHAR(2) - Country code
- `region` TEXT - State/region name
- `city` TEXT - City name
- `latitude` DOUBLE PRECISION
- `longitude` DOUBLE PRECISION
- `postal` TEXT - Postal code
- `timezone` TEXT - Timezone string
- `isp` TEXT - ISP/org name
- `updated_at` TIMESTAMPTZ

Columns in `asn_geo`:
- `asn` BIGINT PK - FK -> asn(asn)
- `country` CHAR(2) - Country code
- `org` TEXT - Organization name
- `updated_at` TIMESTAMPTZ

## Layer 5: BGP (Border Gateway Protocol)

| Table | Type | Records | Description |
|-------|------|--------:|-------------|
| bgp_update | hypertable | 0 | BGP route updates |
| bgp_rib_snapshot | hypertable | 0 | BGP RIB snapshots by collector |
| bgp_as_path | standard | 0 | AS path records |
| bgp_event | hypertable | 0 | BGP events (announce, withdraw, hijack) |

## Layer 6: Routing Anomalies

| Table | Type | Records | Description |
|-------|------|--------:|-------------|
| routing_anomaly | hypertable | 0 | Anomaly detection (hijack, leak, path changes) |
| prefix_flap | hypertable | 0 | Prefix flap events |

## Layer 7: DNS

| Table | Type | Records | Description |
|-------|------|--------:|-------------|
| dns_resolution | hypertable | 0 | Historical DNS resolutions |
| rdns_history | hypertable | 0 | Reverse DNS (PTR) history |

## Layer 8: Reputation / Threat Intelligence

| Table | Type | Records | Description |
|-------|------|--------:|-------------|
| reputation_score | hypertable | 0 | Prefix reputation scores |

## Totals

| Metric | Value |
|--------|------:|
| Total tables | 21 |
| Hypertables | 11 |
| Total records | 14.285.637 |
| Total ASNs | 85.748 |
| Total IP prefixes | 6.744.521 |
| Total geo records | 5.863.490 |
| Total ASN-prefix mappings | 1.591.878 |
| IPv4 prefixes | 4.433.689 |
| IPv6 prefixes | 2.310.832 |
| IPv4 sum (w/ overlap) | 7.316.945.834 |
| Unique IPv4 coverage | ~465 million IPs |

## Indexes

```sql
CREATE INDEX idx_bgp_update_prefix ON bgp_update(prefix);
CREATE INDEX idx_bgp_update_asn ON bgp_update(asn);
CREATE INDEX idx_bgp_event_prefix ON bgp_event(prefix);
CREATE INDEX idx_asn_prefix_map_prefix ON asn_prefix_map(prefix);
CREATE INDEX idx_asn_prefix_map_asn ON asn_prefix_map(asn);
CREATE INDEX idx_ip_geo_country ON ip_geo(country);
CREATE INDEX idx_ip_prefix_asn ON ip_prefix(asn);
CREATE INDEX idx_ip_prefix_country ON ip_prefix(country);
```
