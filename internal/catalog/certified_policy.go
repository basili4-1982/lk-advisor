package catalog

import "strings"

// RegulatedIndustries are the client industries where certified software is
// expected by default. The policy is a product decision and can be disabled
// per request with certified_only=false.
var RegulatedIndustries = map[string]bool{
	"Медицинские организации":      true,
	"Образование и наука":          true,
	"Органы власти":                true,
	"Финансы (банки, страхование)": true,
}

// IsRegulatedIndustry reports whether the industry should default to
// certified-only recommendations.
func IsRegulatedIndustry(industry string) bool {
	return RegulatedIndustries[strings.TrimSpace(industry)]
}
