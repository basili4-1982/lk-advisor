package recommend

// Intent returns the primary intent for the profile.
//
// Priority: renewal -> upsell -> cross_sell -> analog -> none.
func Intent(in Input) string {
	for _, l := range in.Profile.Licenses {
		if l.Status != "active" {
			continue
		}
		d := daysUntil(l.End, in.Now)
		if d >= 0 && d <= 90 {
			return "renewal"
		}
	}

	owned := ownedIDs(in.Profile)
	for _, r := range in.Relations {
		if in.RelationBehavior(r.Type) == "upsell" && owned[r.FromID] {
			return "upsell"
		}
	}

	for _, p := range in.Profile.Purchases {
		if d := daysSince(p.Date, in.Now); d >= 0 && d <= 30 {
			return "cross_sell"
		}
	}

	for id := range owned {
		if prod, ok := in.Product(id); ok && prod.Status == "discontinued" {
			return "analog"
		}
	}

	return "none"
}
