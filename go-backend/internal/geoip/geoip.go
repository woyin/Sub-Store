package geoip

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

var (
	mu            sync.RWMutex
	countryReader *geoip2.Reader
	asnReader     *geoip2.Reader
)

// InitMMDB initializes the GeoIP MMDB readers from the given file paths.
// If a path is empty or the file does not exist, that reader is skipped.
func InitMMDB(countryPath, asnPath string) error {
	mu.Lock()
	defer mu.Unlock()

	if countryPath != "" {
		if _, err := os.Stat(countryPath); err == nil {
			reader, err := geoip2.Open(countryPath)
			if err != nil {
				return fmt.Errorf("failed to open country MMDB %s: %w", countryPath, err)
			}
			if countryReader != nil {
				countryReader.Close()
			}
			countryReader = reader
		}
	}

	if asnPath != "" {
		if _, err := os.Stat(asnPath); err == nil {
			reader, err := geoip2.Open(asnPath)
			if err != nil {
				return fmt.Errorf("failed to open ASN MMDB %s: %w", asnPath, err)
			}
			if asnReader != nil {
				asnReader.Close()
			}
			asnReader = reader
		}
	}

	return nil
}

// GeoIP returns the ISO country code for the given IP address.
// Returns empty string if the lookup fails or no reader is available.
func GeoIP(ipStr string) string {
	mu.RLock()
	defer mu.RUnlock()

	if countryReader == nil {
		return ""
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}

	record, err := countryReader.Country(ip)
	if err != nil {
		return ""
	}

	return record.Country.IsoCode
}

// IPASN returns the ASN number for the given IP address.
// Returns 0 if the lookup fails or no reader is available.
func IPASN(ipStr string) int {
	mu.RLock()
	defer mu.RUnlock()

	if asnReader == nil {
		return 0
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return 0
	}

	record, err := asnReader.ASN(ip)
	if err != nil {
		return 0
	}

	return int(record.AutonomousSystemNumber)
}

// IPASO returns the ASN organization name for the given IP address.
// Returns empty string if the lookup fails or no reader is available.
func IPASO(ipStr string) string {
	mu.RLock()
	defer mu.RUnlock()

	if asnReader == nil {
		return ""
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}

	record, err := asnReader.ASN(ip)
	if err != nil {
		return ""
	}

	return record.AutonomousSystemOrganization
}

// IsMMDBReady returns true if the country MMDB reader is loaded.
func IsMMDBReady() bool {
	mu.RLock()
	defer mu.RUnlock()
	return countryReader != nil
}

// CloseMMDB closes all MMDB readers.
func CloseMMDB() {
	mu.Lock()
	defer mu.Unlock()

	if countryReader != nil {
		countryReader.Close()
		countryReader = nil
	}
	if asnReader != nil {
		asnReader.Close()
		asnReader = nil
	}
}
