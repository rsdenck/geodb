# Contexto do Projeto: Internet Global Mapping Database

## Proposito

Este projeto constroi um banco de dados geografico global de IPs (Internet Global Mapping Database) usando PostgreSQL + TimescaleDB. Ele ingere dados de multiplas fontes publicas para mapear prefixos IP, ASNs, geolocalizacao, rotas BGP, alocacoes RIR, informacoes de DNS e reputacao.

## Arquitetura

### Stack
- **Linguagem:** Go 1.26
- **Banco:** PostgreSQL + TimescaleDB (hypertables para dados temporais)
- **Fontes de dados:**
  - MaxMind GeoLite2 (City e ASN) - geolocalizacao e ASN mapping
  - RIPEstat (country-resource-list + announced-prefixes) - bulk de prefixos/ASNs por pais
  - RIPE IP map (range store) - full internet IP space mapping
- **Driver BD:** pgx v5

### Estrutura do Projeto

```
/opt/geoip/
  src/           # Codigos fonte Go
    main.go      # Entrypoint: cria schema, importa dados, finaliza
    schema.go    # 20 CREATE TABLE + 11 hypertables + indexes
    migrate.go   # Importacao: MMDB City, MMDB ASN, range store
    scraper.go   # Coleta RIPEstat: ASNs por pais, prefixos por ASN
    mmdb.go      # Engine de lookup MaxMind (legado, mantido para API futura)
    range.go     # RangeStore, rangeToCIDR(), parsePrefix()
    contexto.md  # Este documento
  db/
    geoip.sql    # Schema completo do banco
  bin/
    geoip        # Binario compilado
```

### Modulos Go (go.mod)

```
module geoip
require (
    github.com/jackc/pgx/v5 v5.10.0
    github.com/oschwald/maxminddb-golang/v2 v2.4.0
)
```

### Conexao com o Banco

Lida pela funcao `NewPGStore()` em `migrate.go`. Le a env var `GEOIP_DSN` (formato `postgres://user:pass@host:port/dbname`). Se nao definida, usa default:
```
postgres://geoip:geoip123@127.0.0.1:5432/geoip
```

## Schema do Banco (20 tabelas, 11 hypertables)

### Camada ASN
- `asn` (PK: asn) - cadastro de Autonomous System Numbers
- `asn_org` (FK -> asn) - organizacoes por ASN
- `asn_contact` (FK -> asn) - contatos/abuse por ASN
- `asn_geo` (PK: asn) - geolocalizacao por ASN
- `asn_prefix_map` (PK: asn+prefix, FK -> asn) - mapeamento ASN <-> prefixo
- `asn_prefix_history` (hypertable) - historico de mudancas ASN-prefixo

### Camada IP Prefix
- `ip_prefix` (PK: prefix, FK -> asn ON DELETE SET NULL) - todos os prefixos IP
- `prefix_history` (hypertable) - historico de mudancas de prefixos

### Camada RIR
- `rir_allocation` (PK: prefix) - alocacoes dos Regional Internet Registries
- `rir_assignment_history` (hypertable) - historico de atribuicoes RIR

### Camada Geolocalizacao
- `ip_geo` (PK: prefix) - dados de geolocalizacao por prefixo (MaxMind City)
- `asn_geo` (PK: asn) - dados de geo por ASN

### Camada BGP
- `bgp_update` (hypertable) - atualizacoes BGP (prefixo, AS path, next-hop)
- `bgp_rib_snapshot` (hypertable) - snapshots da RIB BGP por coletor
- `bgp_as_path` (PK: id) - tabela de AS paths
- `bgp_event` (hypertable) - eventos BGP (withdraws, hijacks, etc.)

### Camada Routing Anomalies
- `routing_anomaly` (hypertable) - deteccao de anomalias de roteamento
- `prefix_flap` (hypertable) - flapeamento de prefixos

### Camada DNS
- `dns_resolution` (hypertable) - resolucoes DNS historicas
- `rdns_history` (hypertable) - historico de registros reversos

### Camada Reputacao
- `reputation_score` (hypertable) - scores de reputacao por prefixo

## Dados Importados

### Fontes e Quantidades

| Fonte | Tabela | Registros | Descricao |
|-------|--------|----------:|-----------|
| MaxMind City | ip_geo | 5.863.490 | Geo IP por prefixo |
| MaxMind ASN | asn | 85.748 | ASNs com org/geo |
| MaxMind ASN | asn_prefix_map | ~800K | Mapa ASN-prefixo da MMDB |
| MaxMind ASN | ip_prefix | ~1.1M | Prefixos do ASN DB |
| Range Store | ip_prefix | +5.6M | Full internet scan RIPE |
| Range Store | asn_prefix_map | +700K | Mapa ASN-prefixo RIPE |
| Range Store | asn | +70K | ASNs extraidos |

### Totais Consolidados

| Metrica | IPv4 | IPv6 |
|---------|------|------|
| Total prefixos | 4.433.689 | 2.310.832 |
| Total IPs (com overlap) | 7.316.945.834 | ~4.8e37 |
| Cobertura unica estimada | ~465M (/24 aggr) | N/A (universo) |
| Prefixos com ASN | 858.883 | N/A |
| Prefixos com geo | 5.863.490 | N/A |

### Paises Importados (por pais)

Via country-resource-list + announced-prefixes:
- **Brasil:** 21.679 prefixos, 9.129 ASNs
- **LATAM (AR,CL,CO,PE,UY,PY,BO,EC,VE,MX):** ~10.441 prefixos, 4.205 ASNs
- **Asia (CN,JP,KR,IN,TW,HK,SG,MY,ID,TH,VN,PH,PK,BD,LK,NP,MM,KH,LA,MN):** ~87.000 prefixos, 27.604 ASNs
- **Oriente Medio (AE,SA,IL,IR,QA,KW,OM):** ~8.350 prefixos
- **Russia, Australia, Nova Zelandia, Africa do Sul:** ~29.800 prefixos

### Paises Faltantes (precisam ser importados)

- **Europa:** GB, DE, FR, IT, ES, NL, SE, NO, FI, DK, PT, IE, CH, AT, BE, PL, CZ, HU, RO, GR, BG, HR, SK, LT, SI, LV, EE, IS, LU, MT, CY, AL, MK, BA, RS, ME, UA, BY, MD
- **America do Norte:** US, CA
- **Africa:** restante (~45 paises)
- **Asia Central / MENA:** restante (~15 paises)
- **Oceania:** ilhas do Pacifico restantes

## Pipeline de Importacao

### Fluxo Completo (`main.go`)

1. `runSchema()` - Cria tabelas + hypertables + indexes
2. `migrateGeoIP()` - Importa dados:
   a. `importCityMMDB()` - MaxMind GeoLite2-City -> ip_geo, ip_prefix (5.9M records, ~3m42s)
   b. `importASNMMDB()` - MaxMind GeoLite2-ASN -> asn, asn_geo, asn_prefix_map, ip_prefix (1.1M records, ~53s)
   c. `importRangeStore()` - data/ranges.json -> asn, ip_prefix, asn_prefix_map (600K+ ranges)
3. `runStats()` - Exibe contagem de registros por tabela

### Scraper (`scraper.go`)

Funcoes disponiveis para continuar importando dados:
- `fetchCountryPrefixes(country_code)` - Busca prefixos + ASNs de um pais via country-resource-list
- `fetchASNPrefixes(asn)` - Busca prefixos anunciados por um ASN especifico (usado internamente pelo mapper)
- `fetchBRASNs()` - Busca lista de ASNs brasileiros
- `runScraperImportCountry(country_code)` - Importa pais completo (prefixos + ASN mapping)
- `runScraperMapASNs(org_filter, country_filter)` - Mapeia prefixos por ASN em lote (16 workers)

### Configuracoes Importantes

- **Batch insert:** maximo de 60.000 parametros por batch (limite do protocolo estendido PostgreSQL de 65.535)
- **Range format:** API as vezes retorna `a.b.c.d-e.f.g.h`; `rangeToCIDR()` converte para CIDR
- **ASN=0:** Mapeado para NULL (nao existem ASNs com valor 0)
- **Timeout API RIPEstat:** 30s por requisicao; cerca de 1.871 ASNs brasileiros falharam por timeout
- **DSN:** Via `GEOIP_DSN` env var ou fallback para `postgres://geoip:geoip123@127.0.0.1:5432/geoip`
- **Data path:** Via `GEOIP_DATA` env var, fallback para `./data`

## Como Construir e Executar

### Build
```bash
cd src
go build -o ../bin/geoip .
```

### Executar (construtor do banco)
```bash
./bin/geoip
```

### Executar apenas o schema
```bash
PGOPTIONS='-c search_path=public' psql -h localhost -U geoip -d geoip -f db/geoip.sql
```

## Proximos Passos Planejados

1. **Importar paises restantes** - Europa + America do Norte + Africa via `scraper.go`
2. **Pipeline de enrichment** - Preencher bgp_as_path, prefix_history, asn_prefix_history de RIPEstat ou RouteViews
3. **BGP feeds** - Conectar a coletores RouteViews / RIPE RIS para bgp_update, bgp_rib_snapshot
4. **RIR delegation** - Importar arquivos de delegação RIR (afrinic, apnic, arin, lacnic, ripe) para rir_allocation
5. **DNS/Reputation** - Alimentar dns_resolution, rdns_history, reputation_score com dados externos
6. **Anomaly detection** - Routinator / BGPStream para routing_anomaly, prefix_flap
7. **API HTTP** - Criar endpoint de lookup contra o schema normalizado
8. **Reprocessar ASNs com timeout** - ~1.871 ASNs brasileiros falharam; retentar com timeout maior

## Observacoes Tecnicas

- O limite de parametros do PostgreSQL (65535 para modo estendido) exige chunking em batches. A constante usada e 60000 params/batch.
- ASN `0` nao existe na tabela `asn`; prefixos sem ASN usam NULL e sao ignorados no `asn_prefix_map` por causa da FK.
- `ip_prefix.asn` tem FK `ON DELETE SET NULL` (nao CASCADE) porque o prefixo deve sobreviver a delecao do ASN.
- `asn_prefix_map.asn` tem FK `ON DELETE CASCADE` porque o mapa e um subproduto do ASN.
- `rangeToCIDR()` em `range.go` normaliza ranges `a.b.c.d-e.f.g.h` para CIDR; usada por `migrate.go` e `scraper.go`.
- O arquivo `data/ranges.json` (133MB) contem o full IP space scan do RIPE armazenado em formato JSON.
- A tabela `geo_cache` foi removida (artefato de versao anterior).
- Versao anterior tinha `geoip_all` (hypertable unica com 8.8M registros) e `abusive_ips`; ambas removidas.
