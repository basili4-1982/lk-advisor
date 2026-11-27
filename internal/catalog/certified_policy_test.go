package catalog

import "testing"

func TestIsRegulatedIndustry(t *testing.T) {
	for _, industry := range []string{
		"Медицинские организации",
		"Образование и наука",
		"Органы власти",
		"Финансы (банки, страхование)",
	} {
		if !IsRegulatedIndustry(industry) {
			t.Fatalf("%q must be regulated", industry)
		}
	}
	if IsRegulatedIndustry("ИТ (разработчики)") {
		t.Fatal("IT must not be regulated")
	}
}
