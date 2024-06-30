package controller

import (
	"github.com/uthoplatforms/utho-go/utho"
	"net"
	"os"
	"regexp"
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
