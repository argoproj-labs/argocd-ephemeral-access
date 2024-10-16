package generator

import (
	"math"
	"regexp"
	"strings"
)

var (
	DNS1123SubdomainInvalidCharSet       = regexp.MustCompile("[^a-z0-9-.]")
	DNS1123SubdomainInvalidStartChartSet = regexp.MustCompile("^[^a-z0-9]+")
)

// ToDNS1123Subdomain removes all invalid char for a DNS1123Subdomain string and transform it to a valid format
func ToDNS1123Subdomain(data string) string {
	data = strings.ToLower(data)
	data = DNS1123SubdomainInvalidCharSet.ReplaceAllString(data, "")
	data = DNS1123SubdomainInvalidStartChartSet.ReplaceAllString(data, "")
	return data
}

// ToMaxLength reduce the total length of the strings, balancing the length as evently as possible
func ToMaxLength(a, b string, max int) (string, string) {
	aLength := len(a)
	bLength := len(b)
	halfLength := max / 2
	remainder := max % 2
	minLength := int(math.Min(float64(aLength), float64(bLength)))
	available := int(math.Max(0, float64(halfLength-minLength)))

	if aLength >= halfLength+available+remainder {
		a = a[:halfLength+available+remainder]
		remainder = 0
	}
	if bLength >= halfLength+available+remainder {
		b = b[:halfLength+available+remainder]
		remainder = 0
	}
	return a, b
}
