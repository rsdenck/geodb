package main

import (
	"fmt"
	"net/netip"
	"path/filepath"

	"github.com/oschwald/maxminddb-golang/v2"
)

type GeoResult struct {
	City    string
	Region  string
	Country string
	ASN     uint
	ASOrg   string
	Lat     float64
	Lon     float64
}

type GeoEngine struct {
	cityDB    *maxminddb.Reader
	asnDB     *maxminddb.Reader
	countryDB *maxminddb.Reader
	dataDir   string
}

func NewGeoEngine(dataDir string) (*GeoEngine, error) {
	e := &GeoEngine{dataDir: dataDir}
	var err error

	cityPath := filepath.Join(dataDir, "city.mmdb")
	e.cityDB, err = maxminddb.Open(cityPath)
	if err != nil {
		return nil, fmt.Errorf("erro abrindo City DB: %w", err)
	}

	asnPath := filepath.Join(dataDir, "asn.mmdb")
	e.asnDB, err = maxminddb.Open(asnPath)
	if err != nil {
		e.cityDB.Close()
		return nil, fmt.Errorf("erro abrindo ASN DB: %w", err)
	}

	countryPath := filepath.Join(dataDir, "country.mmdb")
	e.countryDB, err = maxminddb.Open(countryPath)
	if err != nil {
		e.cityDB.Close()
		e.asnDB.Close()
		return nil, fmt.Errorf("erro abrindo Country DB: %w", err)
	}

	return e, nil
}

func (e *GeoEngine) Close() {
	if e.cityDB != nil {
		e.cityDB.Close()
	}
	if e.asnDB != nil {
		e.asnDB.Close()
	}
	if e.countryDB != nil {
		e.countryDB.Close()
	}
}

func (e *GeoEngine) Lookup(ipStr string) (*GeoResult, error) {
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return nil, fmt.Errorf("IP invalido: %s", ipStr)
	}

	res := &GeoResult{}

	var city struct {
		City struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"city"`
		Subdivisions []struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"subdivisions"`
		Country struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"country"`
		Location struct {
			Latitude  float64 `maxminddb:"latitude"`
			Longitude float64 `maxminddb:"longitude"`
		} `maxminddb:"location"`
	}
	if result := e.cityDB.Lookup(addr); result.Found() {
		if err := result.Decode(&city); err == nil {
			if name, ok := city.City.Names["pt-BR"]; ok {
				res.City = name
			} else if name, ok := city.City.Names["en"]; ok {
				res.City = name
			}
			if len(city.Subdivisions) > 0 {
				if name, ok := city.Subdivisions[0].Names["pt-BR"]; ok {
					res.Region = name
				} else if name, ok := city.Subdivisions[0].Names["en"]; ok {
					res.Region = name
				}
			}
			if name, ok := city.Country.Names["pt-BR"]; ok {
				res.Country = name
			} else if name, ok := city.Country.Names["en"]; ok {
				res.Country = name
			}
			res.Lat = city.Location.Latitude
			res.Lon = city.Location.Longitude
		}
	}

	var asn struct {
		AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
		AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
	}
	if result := e.asnDB.Lookup(addr); result.Found() {
		if err := result.Decode(&asn); err == nil {
			res.ASN = asn.AutonomousSystemNumber
			res.ASOrg = asn.AutonomousSystemOrganization
		}
	}

	if res.Country == "" {
		var country struct {
			Country struct {
				Names map[string]string `maxminddb:"names"`
			} `maxminddb:"country"`
		}
		if result := e.countryDB.Lookup(addr); result.Found() {
			if err := result.Decode(&country); err == nil {
				if name, ok := country.Country.Names["pt-BR"]; ok {
					res.Country = name
				} else if name, ok := country.Country.Names["en"]; ok {
					res.Country = name
				}
			}
		}
	}

	return res, nil
}
