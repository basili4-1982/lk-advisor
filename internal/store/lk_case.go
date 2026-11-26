package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

// SeedLKCase adds the real test-cabinet case from lk.softline.ru.
// It is separate from the synthetic generator so unit tests keep 200 products.
func SeedLKCase(ctx context.Context, dbc *sql.DB) error {
	req := model.Requirements{OS: "Windows 10+", RAMGB: 8, CPU: "4 ядра", GPU: "integrated", DiskGB: 16}
	license := func(id, sku, title, category, subcategory, vendor string, price int64, seats int, status, audience string) model.Product {
		return model.Product{
			ID: id, SKU: sku, Title: title, Category: category, Subcategory: subcategory,
			Vendor: vendor, Type: "license", Price: price, TermMonths: 12, Seats: seats,
			Version: "2026", Status: status, Audience: audience, Requirements: req,
		}
	}

	products := []model.Product{
		license("prod-lk-kaspersky", "LK-17142", "Kaspersky Endpoint Security для бизнеса — Стандартный, 25–49 Node, 1 year Renewal", "security", "endpoint_security", "Kaspersky", 2500, 45, "active", "business"),
		license("prod-lk-adobe", "LK-96", "Adobe Creative Cloud for enterprise All Apps, Education Device, Renewal", "graphics", "design", "Adobe", 0, 1, "active", "business"),
		license("prod-lk-drweb", "LK-22", "Dr.Web Desktop Security Suite + Центр Управления, 12 мес., Продление", "security", "endpoint_security", "DrWeb", 1200, 10, "active", "business"),
		license("prod-lk-m365", "LK-16979", "Microsoft 365 Personal, 1YR", "office", "office_suite", "Microsoft", 7685, 1, "active", "consumer"),
	}
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	client := model.Client{
		ID: "lk-36612", Name: "Компания 4 (тестовый ЛК)", Segment: "B2B",
		Industry: "IT", Size: "50-200", Region: "RU", Audience: "b2b",
		Shard: model.Shard("lk-36612", 8),
	}
	licenses := []model.License{
		{ID: "lk-lic-kaspersky", ClientID: client.ID, ProductID: "prod-lk-kaspersky", Seats: 45, Start: day(2026, 5, 12), End: day(2027, 5, 11), Status: "active"},
		{ID: "lk-lic-adobe", ClientID: client.ID, ProductID: "prod-lk-adobe", Seats: 1, Start: day(2025, 11, 24), End: day(2026, 11, 23), Status: "expired"},
		{ID: "lk-lic-drweb", ClientID: client.ID, ProductID: "prod-lk-drweb", Seats: 10, Start: day(2026, 5, 12), End: day(2027, 5, 11), Status: "active"},
		{ID: "lk-lic-m365", ClientID: client.ID, ProductID: "prod-lk-m365", Seats: 1, Start: day(2026, 5, 1), End: day(2027, 4, 30), Status: "active"},
	}
	purchases := []model.Purchase{
		{ID: "lk-pur-kaspersky", ClientID: client.ID, ProductID: "prod-lk-kaspersky", Date: day(2026, 5, 12), Amount: 125000, Quantity: 1, Seats: 45},
		{ID: "lk-pur-adobe", ClientID: client.ID, ProductID: "prod-lk-adobe", Date: day(2025, 11, 24), Amount: 0, Quantity: 1, Seats: 1},
		{ID: "lk-pur-drweb", ClientID: client.ID, ProductID: "prod-lk-drweb", Date: day(2026, 5, 12), Amount: 0, Quantity: 1, Seats: 10},
		{ID: "lk-pur-m365", ClientID: client.ID, ProductID: "prod-lk-m365", Date: day(2026, 5, 1), Amount: 7685, Quantity: 1, Seats: 1},
	}
	relations := []model.Relation{
		{FromID: "prod-lk-kaspersky", ToID: "prod-backup", Type: "adjacent_task", Direction: "forward", Confidence: 0.9, Status: "confirmed", Source: "lk_demo"},
		{FromID: "prod-lk-kaspersky", ToID: "prod-sl-13", Type: "substitute", Direction: "forward", Confidence: 0.9, Status: "confirmed", Source: "lk_demo"},
		{FromID: "prod-lk-adobe", ToID: "prod-sl-43", Type: "alternative", Direction: "forward", Confidence: 0.9, Status: "confirmed", Source: "lk_demo"},
		{FromID: "prod-lk-drweb", ToID: "prod-backup", Type: "adjacent_task", Direction: "forward", Confidence: 0.85, Status: "confirmed", Source: "lk_demo"},
		{FromID: "prod-lk-drweb", ToID: "prod-sl-13", Type: "substitute", Direction: "forward", Confidence: 0.9, Status: "confirmed", Source: "lk_demo"},
		{FromID: "prod-lk-m365", ToID: "prod-pdf", Type: "adjacent_task", Direction: "forward", Confidence: 0.9, Status: "confirmed", Source: "lk_demo"},
		{FromID: "prod-lk-m365", ToID: "prod-admin-course", Type: "adjacent_task", Direction: "forward", Confidence: 0.85, Status: "confirmed", Source: "lk_demo"},
		{FromID: "prod-lk-m365", ToID: "prod-office-business", Type: "upgrade", Direction: "forward", Confidence: 0.95, Status: "confirmed", Source: "lk_demo"},
		{FromID: "prod-lk-kaspersky", ToID: "prod-security-course", Type: "adjacent_task", Direction: "forward", Confidence: 0.85, Status: "confirmed", Source: "lk_demo"},
	}
	orders := make([]model.Order, 0, len(purchases))
	for _, p := range purchases {
		orders = append(orders, model.Order{
			ID: "lk-order-" + p.ID, ClientID: client.ID, Status: "delivered",
			CreatedAt: p.Date, Items: []model.OrderItem{{ProductID: p.ProductID, Quantity: 1}},
		})
	}
	ds := Dataset{
		Clients:     []model.Client{client},
		Products:    products,
		Environment: []model.Environment{{ClientID: client.ID, OS: "Windows 11", RAMGB: 16, CPU: "8 ядер", GPU: "integrated", Source: "purchase", Confidence: 0.9}},
		Relations:   relations,
		Purchases:   purchases,
		Licenses:    licenses,
		Orders:      orders,
	}
	return Load(ctx, dbc, ds)
}
