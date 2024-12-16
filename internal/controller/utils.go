package controller

import (
	"fmt"
	appsv1alpha1 "github.com/uthoplatforms/utho-cloud-controller-manager/api/v1alpha1"
	"github.com/uthoplatforms/utho-go/utho"
	"net"
	"os"
	"regexp"
	"strings"
)

// getAuthenticatedClient initialises and returns an authenticated Utho Client
func GetAuthenticatedClient() (*utho.Client, error) {
	apiKey := os.Getenv("API_KEY")
	client, err := utho.NewClient(apiKey)
	if err != nil {
		return nil, err
	}
	return &client, nil
}

// containsString checks if a string contains a specific string
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// removeString removes a specific string from a string slice
func RemoveString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}

// TrueOrFalse converts a boolean value to string representations
func TrueOrFalse(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func IsValidIP(ip string) bool {
	if net.ParseIP(ip) == nil {
		return false
	}
	return true
}
func IsValidDomain(domain string) bool {
	// Regular expression to validate domain name
	regex := `^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`
	match, _ := regexp.MatchString(regex, domain)
	return match
}

func IsTargetGroupEqual(tg1 utho.TargetGroup, tg2 appsv1alpha1.TargetGroup) bool {
	// Compare the fields of tg1 and tg2
	if tg1.Name != tg2.Name {
		return false
	}
	if tg1.Protocol != strings.ToUpper(tg2.Protocol) {
		return false
	}
	if tg1.HealthCheckPath != tg2.HealthCheckPath {
		return false
	}
	if tg1.HealthCheckProtocol != strings.ToUpper(tg2.HealthCheckProtocol) {
		return false
	}
	if fmt.Sprintf("%v", tg1.Port) != fmt.Sprintf("%v", tg2.Port) {
		return false
	}
	if fmt.Sprintf("%v", tg1.HealthCheckTimeout) != fmt.Sprintf("%v", tg2.HealthCheckTimeout) {
		return false
	}
	if fmt.Sprintf("%v", tg1.HealthCheckInterval) != fmt.Sprintf("%v", tg2.HealthCheckInterval) {
		return false
	}
	if fmt.Sprintf("%v", tg1.HealthyThreshold) != fmt.Sprintf("%v", tg2.HealthyThreshold) {
		return false
	}
	if fmt.Sprintf("%v", tg1.UnhealthyThreshold) != fmt.Sprintf("%v", tg2.UnhealthyThreshold) {
		return false
	}
	// If all fields are equal, return true
	return true
}

func IsFrontendEqual(fe1 *utho.Frontends, fe2 appsv1alpha1.Frontend) bool {
	// Compare the fields of fe1 and fe2
	if fe1.Name != fe2.Name {
		return false
	}
	if fe1.Algorithm != fe2.Algorithm {
		return false
	}
	if fe1.Proto != fe2.Protocol {
		return false
	}
	if fmt.Sprintf("%v", fe1.Port) != fmt.Sprintf("%v", fe2.Port) {
		return false
	}
	if fe1.Redirecthttps != TrueOrFalse(fe2.RedirectHttps) {
		return false
	}
	if fe1.Cookie != TrueOrFalse(fe2.Cookie) {
		return false
	}
	// If all fields are equal, return true
	return true
}

// RemoveID removes a specific ID from a slice of IDs
func RemoveID(ids []string, id string) []string {
	var result []string
	for _, item := range ids {
		if item != id {
			result = append(result, item)
		}
	}
	return result
}
