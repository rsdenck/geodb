package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type IPRange struct {
	Network string `json:"network"`
	FromIP  string `json:"from_ip"`
	ToIP    string `json:"to_ip"`
	Count   uint64 `json:"count"`
	Type    string `json:"type"`
	ASN     uint   `json:"asn"`
	ASOrg   string `json:"as_org"`
}

type RangeStore struct {
	mu      sync.RWMutex
	ranges  []*IPRange
	path    string
	asn     uint
	asOrg   string
}

func NewRangeStore() *RangeStore {
	s := &RangeStore{
		path: filepath.Join(geoDBPath(), "ranges.json"),
		asn:  28343,
		asOrg: "UNIFIQUE TELECOMUNICACOES S/A",
	}
	s.load()
	return s
}

func (s *RangeStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		s.ranges = []*IPRange{}
		return
	}

	var ranges []*IPRange
	if err := json.Unmarshal(data, &ranges); err != nil {
		s.ranges = []*IPRange{}
		return
	}
	s.ranges = ranges
}

func (s *RangeStore) save() {
	data, err := json.MarshalIndent(s.ranges, "", "  ")
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(s.path), 0755)
	os.WriteFile(s.path, data, 0644)
}

func (s *RangeStore) Add(r *IPRange) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.ranges {
		if existing.Network == r.Network {
			s.ranges[i] = r
			s.save()
			return
		}
	}
	s.ranges = append(s.ranges, r)
	s.save()
}

func (s *RangeStore) AddBatch(ranges []*IPRange) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := map[string]bool{}
	for _, r := range s.ranges {
		existing[r.Network] = true
	}

	for _, r := range ranges {
		if !existing[r.Network] {
			s.ranges = append(s.ranges, r)
			existing[r.Network] = true
		}
	}
	s.save()
}

func (s *RangeStore) List() []*IPRange {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*IPRange, len(s.ranges))
	copy(result, s.ranges)
	return result
}

func (s *RangeStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.ranges)
}

func (s *RangeStore) Contains(ipStr string) (*IPRange, bool) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.ranges {
		_, cidr, err := net.ParseCIDR(r.Network)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return r, true
		}
	}
	return nil, false
}

func parsePrefix(network string) (netip.Prefix, error) {
	return netip.ParsePrefix(network)
}

// Convert a range like "154.8.0.0-154.8.47.255" to the smallest CIDR that covers it
func rangeToCIDR(network string) string {
	if !strings.Contains(network, "-") {
		return network
	}

	parts := strings.SplitN(network, "-", 2)
	if len(parts) != 2 {
		return network
	}

	startIP, endIP := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	start := net.ParseIP(startIP)
	end := net.ParseIP(endIP)
	if start == nil || end == nil {
		return network
	}

	start4 := start.To4()
	end4 := end.To4()
	if start4 == nil || end4 == nil {
		// IPv6: use start/128 as best effort
		return startIP + "/128"
	}

	s := uint32(start4[0])<<24 | uint32(start4[1])<<16 | uint32(start4[2])<<8 | uint32(start4[3])
	e := uint32(end4[0])<<24 | uint32(end4[1])<<16 | uint32(end4[2])<<8 | uint32(end4[3])

	if s == e {
		return startIP + "/32"
	}

	diff := s ^ e
	prefixLen := 32
	for i := 31; i >= 0; i-- {
		if diff&(1<<i) != 0 {
			prefixLen = 31 - i
			break
		}
	}

	mask := uint32(0xFFFFFFFF << (32 - prefixLen))
	networkStart := s & mask
	networkEnd := networkStart | (0xFFFFFFFF >> prefixLen)

	if s >= networkStart && e <= networkEnd {
		return fmt.Sprintf("%d.%d.%d.%d/%d",
			byte(networkStart>>24), byte(networkStart>>16), byte(networkStart>>8), byte(networkStart), prefixLen)
	}

	// Need wider prefix
	prefixLen--
	mask = uint32(0xFFFFFFFF << (32 - prefixLen))
	networkStart = s & mask
	return fmt.Sprintf("%d.%d.%d.%d/%d",
		byte(networkStart>>24), byte(networkStart>>16), byte(networkStart>>8), byte(networkStart), prefixLen)
}

// Get all active ranges as prefixes
func getRangesAsPrefixes() []netip.Prefix {
	store := NewRangeStore()
	var prefixes []netip.Prefix
	for _, r := range store.List() {
		p, err := netip.ParsePrefix(r.Network)
		if err == nil {
			prefixes = append(prefixes, p)
		}
	}
	return prefixes
}
