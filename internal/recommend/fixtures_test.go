package recommend_test

import (
	"time"

	"github.com/basili4-1982/lk-advisor/internal/model"
	"github.com/basili4-1982/lk-advisor/internal/recommend"
)

var at = time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

func prod(id string, price int64, ram int, status string) model.Product {
	return model.Product{
		ID: id, Title: id, Price: price, Status: status,
		Requirements: model.Requirements{OS: "Windows 10+", RAMGB: ram, CPU: "2 ядра", GPU: "integrated"},
	}
}

func profile(licenses []model.License, purchases []model.Purchase) model.Profile {
	return model.Profile{
		ClientID:    "smb-001",
		Licenses:    licenses,
		Purchases:   purchases,
		Environment: model.Environment{OS: "Windows 11", RAMGB: 8, GPU: "integrated"},
	}
}

func input(p model.Profile, products []model.Product, rels []model.Relation, rules []model.Rule, promos []model.Promotion) recommend.Input {
	m := make(map[string]model.Product, len(products))
	for _, x := range products {
		m[x.ID] = x
	}
	return recommend.Input{
		Profile: p, Products: m, Relations: rels, Rules: rules,
		Promotions: promos, Now: at,
	}
}

func licenseFor(productID string, days int) model.License {
	return model.License{
		ID: "lic", ProductID: productID, Seats: 10, Status: "active",
		Start: at.AddDate(-1, 0, 0), End: at.AddDate(0, 0, days),
	}
}

func purchase(productID string, daysAgo int) model.Purchase {
	return model.Purchase{
		ID: "pur", ProductID: productID, Date: at.AddDate(0, 0, -daysAgo),
		Amount: 1000, Quantity: 1, Seats: 1,
	}
}
