package utils

import (
	"strings"

	"golang.org/x/net/idna"
)

// DomainToSystemName converts a domain name to a valid system name
// Used for usernames, directory names, and container names
// Example: linkedingue.com -> linkedingue-com
// Example: sub.domain.fr -> sub-domain-fr
// Example: tiéuntigre.fr -> xn--tiuntigre-c4a-fr (IDN domains are converted to Punycode first)
func DomainToSystemName(domain string) string {
	// Convert internationalized domains (IDN) to Punycode/ASCII first
	// This ensures filesystem and Docker compatibility
	asciiDomain, err := idna.ToASCII(domain)
	if err != nil {
		// If conversion fails, use original domain
		asciiDomain = domain
	}

	// Replace dots with hyphens
	name := strings.ReplaceAll(asciiDomain, ".", "-")

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

// DomainToPunycode converts an internationalized domain name (IDN) to Punycode/ASCII
// Returns the Punycode version, or the original domain if conversion fails or not needed
// Example: tiéuntigre.fr -> xn--tiuntigre-c4a.fr
func DomainToPunycode(domain string) string {
	ascii, err := idna.ToASCII(domain)
	if err != nil {
		return domain
	}
	return ascii
}

// DomainToUnicode converts a Punycode domain to Unicode (display format)
// Returns the Unicode version, or the original domain if conversion fails or not needed
// Example: xn--tiuntigre-c4a.fr -> tiéuntigre.fr
func DomainToUnicode(domain string) string {
	unicode, err := idna.ToUnicode(domain)
	if err != nil {
		return domain
	}
	return unicode
}

// IsPunycodeDomain checks if a domain is in Punycode format (contains xn--)
func IsPunycodeDomain(domain string) bool {
	return strings.Contains(domain, "xn--")
}

// DomainDisplayName returns a user-friendly display name for a domain
// For IDN domains, returns: "tiéuntigre.fr (xn--tiuntigre-c4a.fr)"
// For regular domains, returns: "example.com"
func DomainDisplayName(domain string) string {
	// Convert to Unicode first (handles both Punycode and Unicode input)
	unicode := DomainToUnicode(domain)
	punycode := DomainToPunycode(unicode)

	// If they're different, it's an IDN domain - show both
	if unicode != punycode {
		return unicode + " (" + punycode + ")"
	}

	// Regular domain, just return as-is
	return unicode
}
