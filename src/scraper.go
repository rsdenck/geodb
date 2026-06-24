package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BRASN struct {
	ASN         uint   `json:"asn"`
	Description string `json:"description"`
}

type BRASNStore struct {
	ASNs    []BRASN `json:"asns"`
	Updated string  `json:"updated"`
}

func loadBRASNs() *BRASNStore {
	path := filepath.Join(geoDBPath(), "br_asns.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return &BRASNStore{}
	}
	var s BRASNStore
	json.Unmarshal(data, &s)
	return &s
}

func saveBRASNs(s *BRASNStore) {
	path := filepath.Join(geoDBPath(), "br_asns.json")
	s.Updated = time.Now().UTC().Format(time.RFC3339)
	data, _ := json.MarshalIndent(s, "", "  ")
	os.WriteFile(path, data, 0644)
}

// List Brazilian ASNs via RIPEstat country-resource-list
func fetchBRASNs() ([]BRASN, error) {
	url := "https://stat.ripe.net/data/country-resource-list/data.json?resource=BR"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("erro criando request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro lendo resposta: %w", err)
	}

	var result struct {
		Data struct {
			Resources struct {
				Asns []string `json:"asn"`
			} `json:"resources"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("erro parseando JSON: %w", err)
	}

	var asns []BRASN
	seen := map[uint]bool{}
	for _, asnStr := range result.Data.Resources.Asns {
		var asn uint
		fmt.Sscanf(strings.TrimPrefix(asnStr, "AS"), "%d", &asn)
		if asn > 0 && !seen[asn] {
			asns = append(asns, BRASN{ASN: asn, Description: ""})
			seen[asn] = true
		}
	}

	return asns, nil
}

// Fetch description for an ASN via RIPEstat
func fetchASNDetails(asn uint) (string, error) {
	url := fmt.Sprintf("https://stat.ripe.net/data/as-overview/data.json?resource=AS%d", asn)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Data struct {
			Holder string `json:"holder"`
		} `json:"data"`
	}
	json.Unmarshal(body, &result)
	return result.Data.Holder, nil
}

// Fetch announced prefixes for an ASN
func fetchASNPrefixes(asn uint) ([]string, error) {
	url := fmt.Sprintf("https://stat.ripe.net/data/announced-prefixes/data.json?resource=AS%d&min_peers_seeing=5", asn)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Data struct {
			Prefixes []struct {
				Prefix string `json:"prefix"`
			} `json:"prefixes"`
		} `json:"data"`
	}
	json.Unmarshal(body, &result)

	var prefixes []string
	for _, p := range result.Data.Prefixes {
		if p.Prefix != "" {
			prefixes = append(prefixes, p.Prefix)
		}
	}
	return prefixes, nil
}

// ==============================================================
// CLI COMMANDS
// ==============================================================

func runScraperList() {
	store := loadBRASNs()
	if len(store.ASNs) == 0 {
		fmt.Println("Nenhum ASN BR carregado. Use: geoip scraper fetch")
		return
	}
	fmt.Printf("ASNs Brasileiros: %d\n\n", len(store.ASNs))
	fmt.Printf("  %-8s %-50s\n", "ASN", "DESCRICAO")
	fmt.Println(strings.Repeat("-", 62))
	for _, a := range store.ASNs {
		desc := a.Description
		if len(desc) > 48 {
			desc = desc[:48] + "..."
		}
		fmt.Printf("  AS%-6d %-50s\n", a.ASN, desc)
	}
}

func runScraperFetch() {
	fmt.Println("Buscando ASNs brasileiros via RIPEstat...")

	asns, err := fetchBRASNs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERRO: %v\n", err)
		return
	}

	fmt.Printf("Encontrados %d ASNs\n", len(asns))

	store := loadBRASNs()
	store.ASNs = asns
	saveBRASNs(store)
	fmt.Printf("Salvos em %s/br_asns.json\n", geoDBPath())
}

func runScraperDetails(maxAsns int) {
	store := loadBRASNs()
	if len(store.ASNs) == 0 {
		fmt.Println("Nenhum ASN BR carregado. Use: geoip scraper fetch primeiro")
		return
	}

	if maxAsns <= 0 || maxAsns > len(store.ASNs) {
		maxAsns = len(store.ASNs)
	}

	fmt.Printf("Buscando detalhes de %d ASNs...\n", maxAsns)
	for i := 0; i < maxAsns; i++ {
		a := &store.ASNs[i]
		if a.Description != "" {
			continue
		}
		desc, err := fetchASNDetails(a.ASN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  AS%d: erro - %v\n", a.ASN, err)
			continue
		}
		a.Description = desc
		fmt.Printf("  AS%d: %s\n", a.ASN, desc)
		time.Sleep(500 * time.Millisecond)
	}
	saveBRASNs(store)
	fmt.Printf("\nDetalhes salvos para %d ASNs\n", maxAsns)
}

func runScraperImport(all bool) {
	store := loadBRASNs()
	if len(store.ASNs) == 0 {
		fmt.Println("Nenhum ASN BR carregado. Use: geoip scraper fetch primeiro")
		return
	}

	var targets []BRASN
	for _, a := range store.ASNs {
		if a.Description == "" {
			continue
		}
		if !all && len(targets) >= 10 {
			break
		}
		targets = append(targets, a)
	}

	if len(targets) == 0 {
		fmt.Println("Nenhum ASN com descricao. Use: geoip scraper details primeiro")
		return
	}

	rangeStore := NewRangeStore()
	total := 0
	for _, a := range targets {
		fmt.Printf("Buscando prefixos AS%d (%s)...\n", a.ASN, a.Description)
		prefixes, err := fetchASNPrefixes(a.ASN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ERRO: %v\n", err)
			continue
		}
		if len(prefixes) == 0 {
			fmt.Printf("  0 prefixos\n")
			continue
		}

		var batch []*IPRange
		for _, p := range prefixes {
			ipType := "ipv4"
			fromIP := p
			if idx := strings.IndexByte(p, '/'); idx > 0 {
				fromIP = p[:idx]
			}
			if strings.Contains(p, ":") {
				ipType = "ipv6"
			}

			r := &IPRange{
				Network: p,
				FromIP:  fromIP,
				ToIP:    fromIP,
				Type:    ipType,
				Count:   0,
				ASN:     a.ASN,
				ASOrg:   a.Description,
			}
			batch = append(batch, r)
		}

		rangeStore.AddBatch(batch)
		total += len(batch)
		fmt.Printf("  %d prefixos importados\n", len(batch))
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("\nTotal: %d prefixos importados de %d ASNs\n", total, len(targets))
}

// Import ALL Brazilian IP prefixes via single country-resource-list API call
type CountryResourceResponse struct {
	Data struct {
		Resources struct {
			Asns []string `json:"asn"`
			IPv4 []string `json:"ipv4"`
			IPv6 []string `json:"ipv6"`
		} `json:"resources"`
	} `json:"data"`
}

func fetchCountryPrefixes(country string) (*CountryResourceResponse, error) {
	url := fmt.Sprintf("https://stat.ripe.net/data/country-resource-list/data.json?resource=%s", country)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result CountryResourceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("erro parseando JSON: %w", err)
	}
	return &result, nil
}

func runScraperImportCountry(country string) {
	fmt.Printf("Buscando todos os prefixos %s...\n", country)
	result, err := fetchCountryPrefixes(country)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERRO: %v\n", err)
		return
	}

	rangeStore := NewRangeStore()
	var batch []*IPRange
	total := len(result.Data.Resources.IPv4) + len(result.Data.Resources.IPv6)
	fmt.Printf("Total: %d IPv4 + %d IPv6 = %d prefixos\n",
		len(result.Data.Resources.IPv4), len(result.Data.Resources.IPv6), total)

	seen := map[string]bool{}
	for _, r := range rangeStore.List() {
		seen[r.Network] = true
	}

	for _, raw := range result.Data.Resources.IPv4 {
		prefix := rangeToCIDR(raw)
		if seen[prefix] {
			continue
		}
		seen[prefix] = true
		fromIP := prefix
		if idx := strings.IndexByte(prefix, '/'); idx > 0 {
			fromIP = prefix[:idx]
		}
		batch = append(batch, &IPRange{
			Network: prefix, FromIP: fromIP, ToIP: fromIP,
			Type: "ipv4", ASN: 0, ASOrg: country,
		})
	}

	for _, raw := range result.Data.Resources.IPv6 {
		prefix := rangeToCIDR(raw)
		if seen[prefix] {
			continue
		}
		seen[prefix] = true
		fromIP := prefix
		if idx := strings.IndexByte(prefix, '/'); idx > 0 {
			fromIP = prefix[:idx]
		}
		batch = append(batch, &IPRange{
			Network: prefix, FromIP: fromIP, ToIP: fromIP,
			Type: "ipv6", ASN: 0, ASOrg: country,
		})
	}

	// Fix any old entries in range store that still have range format
	for _, r := range rangeStore.List() {
		if strings.Contains(r.Network, "-") {
			r.Network = rangeToCIDR(r.Network)
		}
		if strings.Contains(r.FromIP, "-") {
			parts := strings.SplitN(r.FromIP, "-", 2)
			r.FromIP = strings.TrimSpace(parts[0])
			r.ToIP = strings.TrimSpace(parts[1])
		}
	}

	rangeStore.AddBatch(batch)
	fmt.Printf("Importados %d novos prefixos para %s!\n", len(batch), country)
}

func fetchCountryASNs(country string) ([]BRASN, error) {
	url := fmt.Sprintf("https://stat.ripe.net/data/country-resource-list/data.json?resource=%s", country)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro fetching %s ASNs: %w", country, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Data struct {
			Resources struct {
				Asns []string `json:"asn"`
			} `json:"resources"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("erro parseando %s: %w", country, err)
	}

	seen := map[uint]bool{}
	var asns []BRASN
	for _, asnStr := range result.Data.Resources.Asns {
		var asn uint
		fmt.Sscanf(strings.TrimPrefix(asnStr, "AS"), "%d", &asn)
		if asn > 0 && !seen[asn] {
			asns = append(asns, BRASN{ASN: asn, Description: ""})
			seen[asn] = true
		}
	}
	return asns, nil
}

func runScraperMapASNs(countries []string) {
	var allASNs []BRASN
	seenASN := map[uint]bool{}

	for _, country := range countries {
		fmt.Printf("Buscando ASNs %s...\n", country)
		asns, err := fetchCountryASNs(country)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERRO %s: %v\n", country, err)
			continue
		}
		for _, a := range asns {
			if !seenASN[a.ASN] {
				allASNs = append(allASNs, a)
				seenASN[a.ASN] = true
			}
		}
		fmt.Printf("  %s: %d ASNs\n", country, len(asns))
	}

	if len(allASNs) == 0 {
		fmt.Println("Nenhum ASN encontrado.")
		return
	}

	fmt.Printf("\nTotal: %d ASNs unicos. Buscando descricoes...\n", len(allASNs))
	descs := fetchASNDetailsBatch(allASNs)
	for i := range allASNs {
		if d, ok := descs[allASNs[i].ASN]; ok {
			allASNs[i].Description = d
		}
	}

	rangeStore := NewRangeStore()
	existing := map[string]bool{}
	for _, r := range rangeStore.List() {
		existing[r.Network] = true
	}

	type result struct {
		asn      uint
		asOrg    string
		prefixes []string
		err      error
	}

	asns := allASNs
	totalASNs := len(asns)
	ch := make(chan result, totalASNs)

	workers := 16
	sem := make(chan struct{}, workers)

	label := strings.Join(countries, ",")
	fmt.Printf("Mapeando %d ASNs (%s) com %d workers...\n", totalASNs, label, workers)
	start := time.Now()

	go func() {
		for _, a := range asns {
			sem <- struct{}{}
			go func(a BRASN) {
				defer func() { <-sem }()
				prefixes, err := fetchASNPrefixes(a.ASN)
				if err != nil {
					ch <- result{asn: a.ASN, err: err}
					return
				}
				if a.Description == "" {
					a.Description = fmt.Sprintf("AS%d", a.ASN)
				}
				ch <- result{asn: a.ASN, asOrg: a.Description, prefixes: prefixes}
			}(a)
		}
	}()

	var batch []*IPRange
	done := 0
	imported := 0
	for r := range ch {
		done++
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "  [%d/%d] AS%d: erro - %v\n", done, totalASNs, r.asn, r.err)
		} else {
			asOrg := r.asOrg
			if asOrg == "" {
				asOrg = fmt.Sprintf("AS%d", r.asn)
			}
			for _, raw := range r.prefixes {
				p := rangeToCIDR(raw)
				if existing[p] {
					continue
				}
				existing[p] = true
				fromIP := p
				if idx := strings.IndexByte(p, '/'); idx > 0 {
					fromIP = p[:idx]
				}
				ipType := "ipv4"
				if strings.Contains(p, ":") {
					ipType = "ipv6"
				}
				batch = append(batch, &IPRange{
					Network: p, FromIP: fromIP, ToIP: fromIP,
					Type: ipType, ASN: r.asn, ASOrg: asOrg,
				})
				imported++
			}
		}

		if done%100 == 0 {
			elapsed := time.Since(start).Round(time.Second)
			rate := float64(done) / elapsed.Seconds()
			remaining := time.Duration(float64(totalASNs-done)/rate) * time.Second
			fmt.Printf("  %d/%d ASNs (%d prefixos) - %v decorrido, ~%v restante\n",
				done, totalASNs, imported, elapsed, remaining.Round(time.Second))
		}

		if done >= totalASNs {
			close(ch)
		}
	}

	rangeStore.AddBatch(batch)

	elapsed := time.Since(start).Round(time.Second)
	fmt.Printf("\nConcluido! %d ASNs processados, %d prefixos importados em %v\n",
		done, imported, elapsed)
}

func fetchASNDetailsBatch(asns []BRASN) map[uint]string {
	results := make(map[uint]string, len(asns))
	ch := make(chan struct{ asn uint; desc string }, len(asns))

	workers := 16
	sem := make(chan struct{}, workers)

	for _, a := range asns {
		sem <- struct{}{}
		go func(a BRASN) {
			defer func() { <-sem }()
			desc, err := fetchASNDetails(a.ASN)
			if err == nil && desc != "" {
				ch <- struct{ asn uint; desc string }{a.ASN, desc}
			} else {
				ch <- struct{ asn uint; desc string }{a.ASN, ""}
			}
		}(a)
	}

	for i := 0; i < len(asns); i++ {
		r := <-ch
		if r.desc != "" {
			results[r.asn] = r.desc
		}
	}

	return results
}

func runScraper(args []string) {
	if len(args) < 1 {
		fmt.Println("USO:")
		fmt.Println("  geoip scraper list               - Lista ASNs BR carregados")
		fmt.Println("  geoip scraper fetch              - Busca lista de ASNs BR via RIPEstat")
		fmt.Println("  geoip scraper details [N]        - Busca detalhes dos N primeiros ASNs")
		fmt.Println("  geoip scraper import             - Importa prefixos de 10 ASNs (default)")
		fmt.Println("  geoip scraper import-all         - Importa prefixos de TODOS ASNs")
		fmt.Println("  geoip scraper import-br          - Importa TODOS prefixos BR (unico API call)")
		fmt.Println("  geoip scraper import-country <XX>- Importa prefixos de um pais (ex: AR, CL, CO)")
		fmt.Println("  geoip scraper map-asns           - Mapeia ASNs BR aos prefixos")
		fmt.Println("  geoip scraper map-latam          - Mapeia ASNs LATAM (AR,CL,CO,PE,UY,PY,BO,EC,VE,MX)")
		fmt.Println("  geoip scraper map-country <XX>   - Mapeia ASNs de um pais especifico")
		fmt.Println("  geoip scraper map-asia           - Mapeia ASNs da Asia (CN,JP,KR,IN,TW,HK,SG,MY,ID,TH,VN,PH,PK,BD,...)")
		return
	}

	switch args[0] {
	case "list":
		runScraperList()
	case "fetch":
		runScraperFetch()
	case "details":
		n := 0
		if len(args) > 1 {
			fmt.Sscanf(args[1], "%d", &n)
		}
		runScraperDetails(n)
	case "import":
		runScraperImport(false)
	case "import-all":
		runScraperImport(true)
	case "import-br":
		runScraperImportCountry("BR")
	case "import-country":
		if len(args) > 1 {
			runScraperImportCountry(strings.ToUpper(args[1]))
		} else {
			fmt.Println("USO: geoip scraper import-country <XX> (ex: AR, CL, CO, PE)")
		}
	case "map-asns":
		runScraperMapASNs([]string{"BR"})
	case "map-latam":
		runScraperMapASNs([]string{"AR", "CL", "CO", "PE", "UY", "PY", "BO", "EC", "VE", "MX"})
	case "map-country":
		if len(args) > 1 {
			runScraperMapASNs([]string{strings.ToUpper(args[1])})
		} else {
			fmt.Println("USO: geoip scraper map-country <XX> (ex: AR, CL, CO)")
		}
	case "map-asia":
		runScraperMapASNs([]string{"CN", "JP", "KR", "IN", "TW", "HK", "SG", "MY", "ID",
			"TH", "VN", "PH", "PK", "BD", "LK", "NP", "MM", "KH", "LA", "MN"})
	default:
		fmt.Printf("Subcomando desconhecido: %s\n", args[0])
	}
}
