package utils

import "strings"

// DomainToSystemName converts a domain name to a valid system name
// Used for usernames, directory names, and container names
// Example: linkedingue.com -> linkedingue-com
// Example: sub.domain.fr -> sub-domain-fr
func DomainToSystemName(domain string) string {
	// Replace dots with hyphens
	name := strings.ReplaceAll(domain, ".", "-")

	// Ensure lowercase
	name = strings.ToLower(name)

	// Limit length to 32 characters (Linux username limit)
	if len(name) > 32 {
		name = name[:32]
	}

	return name
}

// DomainToPath returns the /srv path for a domain
// Example: linkedingue.com -> /srv/linkedingue-com
func DomainToPath(domain string) string {
	return "/srv/" + DomainToSystemName(domain)
}
