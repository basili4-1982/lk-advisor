package store

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/basili4-1982/lk-advisor/internal/model"
)

const (
	shardCount       = 8
	clientCount      = 50
	productCount     = 200
	relationCount    = 300
	purchaseCount    = 500
	licenseCount     = 100
	orderCount       = 200
	eventCount       = 1000
	ruleCount        = 20
	promotionCount   = 40
	groundTruthCount = 50
)

var baseDate = time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

// Dataset is a deterministic synthetic data set.
type Dataset struct {
	Clients     []model.Client
	Products    []model.Product
	Environment []model.Environment
	Relations   []model.Relation
	Purchases   []model.Purchase
	Licenses    []model.License
	Orders      []model.Order
	Events      []model.Event
	Rules       []model.Rule
	Promotions  []model.Promotion
	GroundTruth []model.GroundTruthCase
}

// Generate builds the synthetic data set for a seed.
func Generate(seed int64) (Dataset, error) {
	r := rand.New(rand.NewSource(seed))
	ds := Dataset{}

	ds.Clients = generateClients()
	products, named := generateProducts()
	ds.Products = products

	for _, c := range ds.Clients {
		ds.Environment = append(ds.Environment, model.Environment{
			ClientID:   c.ID,
			OS:         "Windows 11",
			RAMGB:      8,
			CPU:        "4 ядра",
			GPU:        "integrated",
			Source:     "purchase",
			Confidence: 0.8,
		})
	}

	ds.Relations = generateRelations(named, ds.Products)
	ds.Purchases = generatePurchases(r, ds.Clients, ds.Products)
	ds.Licenses = generateLicenses(r, ds.Clients, ds.Products)
	ds.Orders = generateOrders(r, ds.Clients, ds.Products)
	ds.Events = generateEvents(r, ds.Clients)
	ds.Rules = generateRules(r, ds.Products)
	ds.Promotions = generatePromotions(r, ds.Products)
	ds.GroundTruth = generateGroundTruth(ds.Clients, ds.Purchases, ds.Licenses, ds.Relations)

	return ds, nil
}

func generateClients() []model.Client {
	out := make([]model.Client, 0, clientCount)
	industries := []string{"e-commerce", "IT", "services", "manufacturing", "logistics"}
	for i := 1; i <= clientCount; i++ {
		id := fmt.Sprintf("smb-%03d", i)
		out = append(out, model.Client{
			ID:       id,
			Name:     fmt.Sprintf("Клиент %02d", i),
			Segment:  "SMB",
			Industry: industries[i%len(industries)],
			Size:     "50-200",
			Region:   "RU",
			Audience: "b2b",
			Shard:    model.Shard(id, shardCount),
		})
	}
	return out
}

type namedProducts map[string]model.Product

func generateProducts() ([]model.Product, namedProducts) {
	req := model.Requirements{OS: "Windows 10+", RAMGB: 4, CPU: "2 ядра", GPU: "integrated", DiskGB: 8}
	base := func(id, sku, title, cat string, price int64, seats int, status string) model.Product {
		return model.Product{
			ID: id, SKU: sku, Title: title, Category: cat,
			Vendor: "Sarnas", Type: "license", Price: price,
			TermMonths: 12, Seats: seats, Version: "2026.1",
			Status: status, Requirements: req,
		}
	}

	named := namedProducts{}
	var namedList []model.Product
	add := func(p model.Product) {
		named[p.ID] = p
		namedList = append(namedList, p)
	}

	add(base("prod-office-suite", "SL-OFF-001", "Microsoft Office 2024 для дома и учёбы", "office", 16551, 10, "active"))
	add(base("prod-office-business", "SL-OFF-002", "Microsoft 365 бизнес стандарт", "office", 36592, 10, "active"))
	add(base("prod-pdf", "SL-PDF-001", "PDF Toolkit", "office", 10000, 10, "active"))
	add(base("prod-esign", "SL-ESG-001", "КриптоАРМ ГОСТ версии 3", "office", 1450, 10, "active"))
	add(base("prod-office-pack", "SL-OPK-001", "Microsoft 365 бизнес премиум", "office", 50000, 10, "active"))
	add(base("prod-raster", "SL-RAS-001", "Microsoft Visio Standard 2024", "graphics", 49392, 5, "active"))
	add(base("prod-raster-2", "SL-RAS-002", "Raster Editor Plus", "graphics", 24000, 5, "active"))
	add(base("prod-vector", "SL-VEC-001", "Microsoft Visio Professional 2024", "graphics", 98467, 5, "active"))
	add(base("prod-vector-pro", "SL-VEC-002", "Microsoft Project Standard 2024", "graphics", 101575, 10, "active"))
	add(base("prod-photo", "SL-PHO-001", "Microsoft 365 Personal (Персональный)", "office", 9107, 5, "active"))
	add(base("prod-video", "SL-VID-001", "Microsoft 365 Family (для семьи)", "office", 11252, 5, "active"))
	add(base("prod-antivirus", "SL-ANT-001", "Kaspersky Endpoint Security Cloud", "security", 12591, 10, "active"))
	add(base("prod-edr", "SL-EDR-001", "PRO32 Endpoint Security Advanced", "security", 9450, 10, "active"))
	add(base("prod-backup", "SL-BKP-001", "Кибер Бэкап", "backup", 18000, 10, "active"))
	add(base("prod-dr", "SL-DRC-001", "Disaster Recovery", "backup", 40000, 10, "active"))
	add(base("prod-ide", "SL-IDE-001", "IDE", "devtools", 16000, 5, "active"))
	add(base("prod-ci", "SL-CIC-001", "CI Runner", "devtools", 14000, 5, "active"))
	add(base("prod-bi", "SL-BIS-001", "BI Studio", "analytics", 28000, 5, "active"))
	add(base("prod-reports", "SL-RPT-001", "Report Builder", "analytics", 12000, 5, "active"))
	add(base("prod-mail", "SL-MAL-001", "Mail Server", "communication", 11000, 10, "active"))
	add(base("prod-messenger", "SL-MSG-001", "Messenger", "communication", 9000, 10, "active"))
	add(base("prod-hypervisor", "SL-HYP-001", "Hypervisor", "virtualization", 45000, 10, "active"))
	add(base("prod-containers", "SL-CNT-001", "Container Platform", "virtualization", 38000, 10, "active"))
	add(base("prod-casual-game", "SL-GAM-001", "Casual Game Pack", "games", 3000, 1, "active"))
	add(base("prod-game-sub", "SL-GAM-002", "Game Subscription", "games", 6000, 1, "active"))
	add(base("prod-admin-course", "SL-CRS-001", "Курс по Office 2024", "training", 8000, 1, "active"))
	add(base("prod-security-course", "SL-CRS-002", "Курс по информационной безопасности", "training", 9000, 1, "active"))
	add(base("prod-pm-course", "SL-CRS-003", "Курс по управлению проектами", "training", 9000, 1, "active"))

	// Real items from the public Softline catalog (store.softline.ru, 2026-11-25).
	// Illustrative demo catalog, not a partner data feed.
	realExtras := []struct {
		title string
		cat   string
		price int64
	}{
		{"Microsoft Windows 11 Home (Домашняя)", "office", 20187},
		{"Microsoft Windows 11 Pro (Профессиональная)", "office", 30454},
		{"Microsoft Project Professional 2024", "office", 176677},
		{"PRO32 Endpoint Security Standard", "security", 7500},
		{"Программа КриптоПро CSP 5.0", "security", 1476},
		{"Код безопасности: Secret Net Studio 8", "security", 599},
		{"Dr.Web Security Space", "security", 680},
		{"Криптоком «МагПро КриптоПакет»", "security", 680},
		{"Kaspersky Plus + Who Calls", "security", 12591},
		{"Эшелон «СКАНЕР-ВС»", "security", 1440},
		{"Программа КриптоПро TSP", "security", 450},
		{"Kaspersky Small Office Security", "security", 1129},
		{"Kaspersky Standard", "security", 1129},
		{"Код безопасности: ПАК «Соболь» 4.0", "security", 730},
		{"Kaspersky Premium + Who Calls", "security", 2900},
		{"КриптоАРМ Стандарт Плюс версии 5", "security", 2900},
		{"Kaspersky Password Manager", "security", 1499},
		{"RedCheck", "security", 1400},
		{"Платформа nanoCAD 26", "graphics", 24600},
		{"КОМПАС-3D V25", "graphics", 1950},
		{"nanoCAD GeoniCS 26", "graphics", 8900},
		{"ГИС «Аксиома»", "graphics", 22000},
		{"КОМПАС-График V25", "graphics", 4140},
		{"КОМПАС-3D HOME", "graphics", 1252},
		{"КОМПАС-Электрик V25", "graphics", 28000},
		{"GEOBRIDGE Условные Знаки", "graphics", 3221},
		{"PROCAD Plant Design Suite", "graphics", 19567},
		{"ЛИРА", "graphics", 3660},
		{"Geo Cartograma", "graphics", 5612},
		{"ГИС Панорама Мини", "graphics", 840},
		{"Лира-САПФИР 2026", "graphics", 15200},
		{"Renga Professional", "graphics", 16500},
		{"VetCAD++", "graphics", 4880},
		{"Панорама АРМ кадастрового инженера", "graphics", 840},
		{"AKSON-vent", "graphics", 30500},
		{"nanoCAD Стройплощадка 26", "graphics", 51400},
		{"nanoCAD Металлоконструкции 26", "graphics", 51400},
		{"Еврософт Одиссей", "graphics", 6000},
		{"nanoCAD BIM СКС 26", "graphics", 57900},
		{"nanoCAD BIM Вентиляция", "graphics", 65600},
		{"nanoCAD комплект Инженерия", "graphics", 126700},
		{"CorelDRAW Graphics Suite. Подписка 365-Day", "graphics", 66379},
		{"CorelDRAW Graphics Suite 2026", "graphics", 124470},
		{"ГрафБорд", "graphics", 700},
		{"Scripting Module", "graphics", 57775},
		{"Relational Module", "graphics", 57775},
		{"EasyCatalog Lite", "graphics", 6924},
		{"XML Data Provider", "graphics", 57775},
		{"EasyCatalog", "graphics", 4608},
		{"Pagination Module", "graphics", 69353},
		{"ODBC Data Provider", "graphics", 23041},
		{"Автограф", "graphics", 7970},
		{"CST CAD Navigator", "graphics", 3950},
		{"«Юнион Лейбл»", "graphics", 4500},
		{"AliveColors", "graphics", 5000},
		{"3D-IO Unwrella", "graphics", 23560},
		{"ФотоМАСТЕР", "graphics", 1980},
		{"D5 Render", "graphics", 44978},
		{"Corel PaintShop Pro 2023", "graphics", 14240},
		{"CorelDRAW Essentials 2024", "graphics", 26027},
		{"Inventory", "graphics", 11750},
		{"«План Эвакуации»", "graphics", 3000},
		{"cebas finalToon 4.0", "graphics", 21682},
		{"cebas PSD-Manager 5.0", "graphics", 9395},
		{"cebas Phoenix Fluid Dynamics 4.0", "graphics", 56374},
		{"ContentReader PDF 15 Business", "office", 11760},
		{"ContentReader PDF 16 Business", "office", 11760},
		{"PDF Commander", "office", 2800},
		{"Аудит и восстановление кадрового учета и делопроизводства в организации ( за 1 сотрудника)", "analytics", 12200},
		{"Сайн.Клик", "analytics", 150},
		{"VR CONCEPT", "analytics", 15000},
		{"PrintStore Pro", "office", 6000},
		{"Spider Project", "analytics", 90000},
		{"GStarCAD", "graphics", 13900},
		{"ELMA365", "analytics", 7200},
		{"Окуляр ГОСТ", "analytics", 720},
		{"TeslaSCADA2", "graphics", 990},
		{"ГосJava", "devtools", 5612},
		{"Help & Manual", "office", 9222},
		{"ИС Заготовка 36и6", "analytics", 8050},
		{"«Волна»", "analytics", 264740},
		{"«Потенциал»", "analytics", 70760},
		{"BrowserStack App Automate", "devtools", 379696},
		{"Система управления проектами Яндекс Трекер", "analytics", 485},
		{"Astra Monitoring", "devtools", 1200},
		{"Kaiten", "analytics", 580},
		{"Business Studio", "analytics", 700},
		{"Pixso Enterprise", "graphics", 13373},
		{"HelpDeskEddy", "analytics", 2440},
		{"ЦБИ ФИКС", "devtools", 500},
		{"Proxyman", "devtools", 12864},
		{"EMS SQL Manager for PostgreSQL", "devtools", 800},
		{"BrowserStack App Live", "devtools", 286203},
		{"ИСУБД CronosPRO", "devtools", 5000},
		{"Postgres Pro Standard", "devtools", 17481},
		{"Моно", "devtools", 11200},
		{"Altova XMLSpy 2026", "devtools", 22097},
		{"FastReport .NET Ultimate", "devtools", 179890},
		{"FastReport .NET Ultimate. Продления", "devtools", 89945},
		{"BrowserStack Live", "devtools", 286203},
		{"EMS SQL Management Studio for PostgreSQL", "devtools", 12480},
		{"Navicat for Oracle", "devtools", 14418},
		{"Navicat Premium", "devtools", 28874},
		{"ДСПИ Cros", "devtools", 2500},
		{"Postgres Pro Enterprise для 1С", "devtools", 15285},
		{"EMS Data Pump for PostgreSQL", "devtools", 3221},
		{"Altova UModel 2026", "devtools", 7076},
		{"ActiveXperts Network Component", "devtools", 26880},
		{"ActiveXperts Serial Port Component", "devtools", 26880},
		{"Advosol OPCDA.NET", "devtools", 30103},
		{"Advosol OPCHDA.NET", "devtools", 47470},
		{"Advosol UA Server Toolkit", "devtools", 86836},
		{"Advosol EasyUA Client SDK", "devtools", 46312},
	}
	for i, x := range realExtras {
		add(base(fmt.Sprintf("prod-sl-%02d", i+1), fmt.Sprintf("SL-MS-%03d", i+1),
			x.title, x.cat, x.price, 1, "active"))
	}

	out := make([]model.Product, 0, productCount)
	out = append(out, namedList...)

	tiers := []string{"Basic", "Pro", "Business", "Enterprise"}
	categories := []string{"office", "graphics", "security", "backup", "devtools",
		"analytics", "communication", "virtualization", "games", "training"}
	i := 0
	for len(out) < productCount {
		cat := categories[i%len(categories)]
		tier := tiers[(i/len(categories))%len(tiers)]
		n := i/len(categories)/len(tiers) + 1
		id := fmt.Sprintf("prod-%s-%03d", cat, i+1)
		status := "active"
		if i%10 == 9 {
			status = "discontinued"
		}
		p := base(id, fmt.Sprintf("SKU-%s-%03d", cat, i+1),
			fmt.Sprintf("%s %s %d", cat, tier, n), cat,
			int64(5000+1000*(i%20)+n*500), 5+5*(i%4), status)
		out = append(out, p)
		i++
	}

	for i := range out {
		out[i].Audience = audienceFor(out[i].Title, out[i].Category)
		out[i].Subcategory = subcategoryFor(out[i].ID, out[i].Title)
	}
	return out, named
}

// subcategoryFor gives key products a task label for diversity.
func subcategoryFor(id, title string) string {
	switch id {
	case "prod-office-suite", "prod-office-business", "prod-office-pack":
		return "office_suite"
	case "prod-pdf":
		return "pdf"
	case "prod-esign":
		return "esign"
	case "prod-raster", "prod-vector":
		return "diagramming"
	case "prod-vector-pro":
		return "project_management"
	case "prod-antivirus", "prod-edr":
		return "endpoint_security"
	case "prod-backup":
		return "backup"
	case "prod-admin-course", "prod-security-course", "prod-pm-course":
		return "training"
	case "prod-sl-84", "prod-sl-86":
		return "project_management"
	case "prod-bi", "prod-reports":
		return "analytics"
	case "prod-ide", "prod-ci":
		return "devtools"
	case "prod-mail", "prod-messenger":
		return "communication"
	case "prod-hypervisor", "prod-containers":
		return "virtualization"
	case "prod-sl-95":
		return "database"
	case "prod-casual-game", "prod-game-sub":
		return "games"
	}

	t := strings.ToLower(title)
	switch {
	case strings.Contains(t, "visio"):
		return "diagramming"
	case strings.Contains(t, "kaiten") || strings.Contains(t, "трекер"):
		return "project_management"
	case strings.Contains(t, "project"):
		return "project_management"
	case strings.Contains(t, "scada"):
		return "scada"
	case strings.Contains(t, "гис") || strings.Contains(t, "панорама") ||
		strings.Contains(t, "аксиома") || strings.Contains(t, "geobridge") ||
		strings.Contains(t, "geoni") || strings.Contains(t, "картограм"):
		return "gis"
	case strings.Contains(t, "inventory") || strings.Contains(t, "module") ||
		strings.Contains(t, "provider") || strings.Contains(t, "pagination") ||
		strings.Contains(t, "scripting") || strings.Contains(t, "easycatalog"):
		return "plugin"
	case strings.Contains(t, "cad") || strings.Contains(t, "компас") ||
		strings.Contains(t, "renga") || strings.Contains(t, "лира") ||
		strings.Contains(t, "строй") || strings.Contains(t, "инженер") ||
		strings.Contains(t, "вентиляц") || strings.Contains(t, "скс") ||
		strings.Contains(t, "металлоконструкц"):
		return "cad"
	case strings.Contains(t, "corel") || strings.Contains(t, "paint") ||
		strings.Contains(t, "design") || strings.Contains(t, "графборд") ||
		strings.Contains(t, "pixso"):
		return "design"
	default:
		return ""
	}
}

// audienceFor tags a product as consumer, business or any.
func audienceFor(title, category string) string {
	t := strings.ToLower(title)
	for _, k := range []string{
		"учёб", "учеб", "personal", "family", "home", "домашняя",
		"essentials", "student", "study", "education",
	} {
		if strings.Contains(t, k) {
			return "consumer"
		}
	}
	if strings.Contains(t, "visio standard") || strings.Contains(t, "компас-график") {
		return "consumer"
	}
	if category == "graphics" {
		return "any"
	}
	for _, k := range []string{
		"business", "pro", "professional", "enterprise", "server",
		"endpoint", "cloud", "corporate", "бизнес", "bim", "cad",
		"криптоарм", "криптопро", "visio", "project", "bi studio", "small office",
		"kaiten", "трекер",
	} {
		if strings.Contains(t, k) {
			return "business"
		}
	}
	return "any"
}

func generateRelations(named namedProducts, products []model.Product) []model.Relation {
	seen := map[string]bool{}
	out := make([]model.Relation, 0, relationCount)
	add := func(from, to, typ string, confidence float64, status, source string) {
		if from == "" || to == "" || from == to {
			return
		}
		key := from + "|" + to + "|" + typ
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, model.Relation{
			FromID: from, ToID: to, Type: typ, Direction: "forward",
			Confidence: confidence, Status: status, Source: source,
		})
	}

	// Named relations used by the demo and acceptance cases.
	add("prod-office-suite", "prod-pdf", "adjacent_task", 0.9, "confirmed", "dataset")
	add("prod-office-suite", "prod-esign", "workflow_next_step", 0.9, "confirmed", "dataset")
	add("prod-office-suite", "prod-admin-course", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-office-suite", "prod-security-course", "adjacent_task", 0.8, "confirmed", "dataset")
	add("prod-office-suite", "prod-sl-13", "adjacent_task", 0.8, "confirmed", "dataset")
	add("prod-office-business", "prod-pdf", "adjacent_task", 0.9, "confirmed", "dataset")
	add("prod-office-business", "prod-esign", "workflow_next_step", 0.9, "confirmed", "dataset")
	add("prod-office-business", "prod-office-pack", "bundle", 0.95, "confirmed", "dataset")
	add("prod-office-business", "prod-office-pack", "upgrade", 0.9, "confirmed", "dataset")
	add("prod-pdf", "prod-office-pack", "bundle", 0.95, "confirmed", "dataset")
	add("prod-esign", "prod-office-pack", "bundle", 0.95, "confirmed", "dataset")
	add("prod-raster", "prod-vector", "upgrade", 0.9, "confirmed", "dataset")
	add("prod-raster", "prod-vector-pro", "adjacent_task", 0.9, "confirmed", "dataset")
	add("prod-raster", "prod-office-business", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-photo", "prod-video", "workflow_next_step", 0.85, "confirmed", "dataset")
	add("prod-antivirus", "prod-backup", "adjacent_task", 0.8, "llm_suggested", "llm")
	add("prod-antivirus", "prod-edr", "excludes", 0.9, "confirmed", "dataset")
	add("prod-vector", "prod-vector-pro", "adjacent_task", 0.9, "confirmed", "dataset")
	add("prod-vector", "prod-office-business", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-ide", "prod-ci", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-bi", "prod-reports", "adjacent_task", 0.8, "confirmed", "dataset")
	add("prod-mail", "prod-messenger", "adjacent_task", 0.7, "confirmed", "dataset")
	add("prod-hypervisor", "prod-containers", "adjacent_task", 0.75, "confirmed", "dataset")
	add("prod-sl-60", "prod-sl-43", "upgrade", 0.95, "confirmed", "dataset")
	add("prod-sl-24", "prod-sl-20", "upgrade", 0.95, "confirmed", "dataset")
	add("prod-sl-23", "prod-sl-20", "upgrade", 0.9, "confirmed", "dataset")
	add("prod-sl-13", "prod-backup", "adjacent_task", 0.9, "confirmed", "dataset")
	add("prod-sl-74", "prod-sl-19", "substitute", 0.9, "confirmed", "dataset")
	add("prod-sl-19", "prod-sl-74", "substitute", 0.9, "confirmed", "dataset")
	add("prod-sl-74", "prod-sl-20", "substitute", 0.9, "confirmed", "dataset")
	add("prod-sl-20", "prod-sl-74", "substitute", 0.9, "confirmed", "dataset")
	add("prod-sl-19", "prod-sl-20", "substitute", 0.9, "confirmed", "dataset")
	add("prod-sl-20", "prod-sl-19", "substitute", 0.9, "confirmed", "dataset")
	add("prod-sl-74", "prod-sl-23", "substitute", 0.9, "confirmed", "dataset")
	add("prod-sl-23", "prod-sl-74", "substitute", 0.9, "confirmed", "dataset")
	add("prod-sl-19", "prod-sl-23", "substitute", 0.9, "confirmed", "dataset")
	add("prod-sl-23", "prod-sl-19", "substitute", 0.9, "confirmed", "dataset")
	add("prod-sl-20", "prod-sl-25", "adjacent_task", 0.9, "confirmed", "dataset")
	add("prod-sl-19", "prod-sl-21", "adjacent_task", 0.9, "confirmed", "dataset")
	add("prod-sl-19", "prod-sl-39", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-sl-19", "prod-sl-40", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-sl-19", "prod-sl-41", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-sl-19", "prod-sl-37", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-sl-19", "prod-sl-36", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-sl-95", "prod-backup", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-antivirus", "prod-sl-13", "substitute", 0.9, "confirmed", "dataset")
	add("prod-ide", "prod-antivirus", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-ide", "prod-backup", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-ide", "prod-security-course", "adjacent_task", 0.85, "confirmed", "dataset")
	add("prod-ci", "prod-backup", "adjacent_task", 0.8, "confirmed", "dataset")
	add("prod-sl-86", "prod-vector-pro", "adjacent_task", 0.9, "confirmed", "dataset")
	add("prod-sl-86", "prod-pm-course", "adjacent_task", 0.9, "confirmed", "dataset")
	add("prod-sl-86", "prod-sl-84", "substitute", 0.9, "confirmed", "dataset")
	add("prod-casual-game", "prod-game-sub", "adjacent_task", 0.8, "confirmed", "dataset")
	add("prod-admin-course", "prod-security-course", "adjacent_task", 0.7, "confirmed", "dataset")
	add("prod-raster-2", "prod-vector", "substitute", 0.85, "rejected", "dataset")

	// Fill within the same task first, then within the category as a fallback.
	byTask := map[string][]model.Product{}
	for _, p := range products {
		task := p.Subcategory
		if task == "" {
			task = p.Category
		}
		byTask[task] = append(byTask[task], p)
	}
	tasks := make([]string, 0, len(byTask))
	for t := range byTask {
		tasks = append(tasks, t)
	}
	sort.Strings(tasks)
	for _, off := range []int{1, 2} {
		for _, t := range tasks {
			list := byTask[t]
			for i := 0; i < len(list) && len(out) < relationCount; i++ {
				add(list[i].ID, list[(i+off)%len(list)].ID,
					"adjacent_task", 0.6+float64(i%3)*0.1, "confirmed", "dataset")
			}
		}
	}

	if len(out) < relationCount {
		byCat := map[string][]model.Product{}
		for _, p := range products {
			byCat[p.Category] = append(byCat[p.Category], p)
		}
		cats := make([]string, 0, len(byCat))
		for c := range byCat {
			cats = append(cats, c)
		}
		sort.Strings(cats)
		for _, off := range []int{1, 2} {
			for _, c := range cats {
				list := byCat[c]
				for i := 0; i < len(list) && len(out) < relationCount; i++ {
					add(list[i].ID, list[(i+off)%len(list)].ID,
						"adjacent_task", 0.5, "confirmed", "demo_filler")
				}
			}
		}
	}

	for i, p := range products {
		if p.Status != "discontinued" {
			continue
		}
		for k := 1; k <= len(products); k++ {
			target := products[(i+k)%len(products)]
			if target.Status == "active" {
				add(p.ID, target.ID, "analog", 0.8, "confirmed", "dataset")
				break
			}
		}
	}

	return out
}

func productIndex(products []model.Product) map[string]model.Product {
	out := make(map[string]model.Product, len(products))
	for _, p := range products {
		out[p.ID] = p
	}
	return out
}

func generatePurchases(r *rand.Rand, clients []model.Client, products []model.Product) []model.Purchase {
	byID := productIndex(products)
	out := make([]model.Purchase, 0, purchaseCount)
	add := func(clientID, productID string, daysAgo int) {
		p := byID[productID]
		out = append(out, model.Purchase{
			ID:        fmt.Sprintf("pur-%04d", len(out)+1),
			ClientID:  clientID,
			ProductID: productID,
			Date:      baseDate.AddDate(0, 0, -daysAgo),
			Amount:    p.Price,
			Quantity:  1,
			Seats:     p.Seats,
		})
	}

	add("smb-001", "prod-antivirus", 350)
	add("smb-002", "prod-raster", 10)
	add("smb-003", "prod-office-business", 60)
	add("smb-004", "prod-antivirus", 30)
	add("smb-005", "prod-vector", 20)

	for len(out) < purchaseCount {
		c := clients[r.Intn(len(clients))]
		if c.ID <= "smb-006" {
			continue
		}
		if len(out)%5 == 0 {
			add(c.ID, "prod-office-suite", r.Intn(365))
			continue
		}
		p := products[r.Intn(len(products))]
		add(c.ID, p.ID, r.Intn(365))
	}
	return out
}

func generateLicenses(r *rand.Rand, clients []model.Client, products []model.Product) []model.License {
	byID := productIndex(products)
	out := make([]model.License, 0, licenseCount)
	seen := map[string]bool{}
	add := func(clientID, productID string, endDays int) {
		key := clientID + "|" + productID
		if seen[key] {
			return
		}
		seen[key] = true
		end := baseDate.AddDate(0, 0, endDays)
		status := "active"
		if end.Before(baseDate) {
			status = "expired"
		}
		p := byID[productID]
		out = append(out, model.License{
			ID:        fmt.Sprintf("lic-%04d", len(out)+1),
			ClientID:  clientID,
			ProductID: productID,
			Seats:     p.Seats,
			Start:     end.AddDate(-1, 0, 0),
			End:       end,
			Status:    status,
		})
	}

	add("smb-001", "prod-antivirus", 14)
	add("smb-003", "prod-office-business", 200)
	add("smb-005", "prod-vector", 120)

	for len(out) < licenseCount {
		c := clients[r.Intn(len(clients))]
		if c.ID <= "smb-006" {
			continue
		}
		p := products[r.Intn(len(products))]
		add(c.ID, p.ID, r.Intn(200)-30)
	}
	return out
}

func generateOrders(r *rand.Rand, clients []model.Client, products []model.Product) []model.Order {
	out := make([]model.Order, 0, orderCount)
	for i := 0; i < orderCount; i++ {
		c := clients[r.Intn(len(clients))]
		created := baseDate.AddDate(0, 0, -r.Intn(120))
		var delivered *time.Time
		status := "processing"
		if r.Intn(10) < 7 {
			d := created.AddDate(0, 0, r.Intn(10))
			delivered = &d
			status = "delivered"
		}
		items := []model.OrderItem{{ProductID: products[r.Intn(len(products))].ID, Quantity: 1}}
		out = append(out, model.Order{
			ID:          fmt.Sprintf("ord-%04d", i+1),
			ClientID:    c.ID,
			Status:      status,
			CreatedAt:   created,
			DeliveredAt: delivered,
			Items:       items,
		})
	}
	return out
}

func generateEvents(r *rand.Rand, clients []model.Client) []model.Event {
	types := []string{"license_view", "order_view", "catalog_view"}
	out := make([]model.Event, 0, eventCount)
	for i := 0; i < eventCount; i++ {
		c := clients[r.Intn(len(clients))]
		out = append(out, model.Event{
			ID:       fmt.Sprintf("evt-%04d", i+1),
			ClientID: c.ID,
			Type:     types[r.Intn(len(types))],
			TS:       baseDate.AddDate(0, 0, -r.Intn(60)),
			Context:  map[string]string{},
		})
	}
	return out
}

func generateRules(r *rand.Rand, products []model.Product) []model.Rule {
	out := []model.Rule{
		{ID: "rule-01", Type: "excludes", Condition: "pair=prod-antivirus:prod-edr", Action: "exclude"},
		{ID: "rule-02", Type: "export_control", Condition: "product=prod-edr", Action: "exclude"},
		{ID: "rule-03", Type: "manager_only", Condition: "product=prod-vector-pro", Action: "approval"},
		{ID: "rule-04", Type: "max_discount", Condition: "discount>10", Action: "cap"},
		{ID: "rule-05", Type: "out_of_stock", Condition: "product=prod-casual-game", Action: "exclude"},
	}
	seen := map[string]bool{}
	for _, rule := range out {
		seen[rule.Condition] = true
	}
	for len(out) < ruleCount {
		a := products[r.Intn(len(products))].ID
		b := products[r.Intn(len(products))].ID
		cond := "pair=" + a + ":" + b
		if a == b || seen[cond] {
			continue
		}
		seen[cond] = true
		out = append(out, model.Rule{
			ID:        fmt.Sprintf("rule-%02d", len(out)+1),
			Type:      "excludes",
			Condition: cond,
			Action:    "exclude",
		})
	}
	return out
}

func generatePromotions(r *rand.Rand, products []model.Product) []model.Promotion {
	out := make([]model.Promotion, 0, promotionCount)
	add := func(productID string, discount int, from, to time.Time) {
		out = append(out, model.Promotion{
			ID:         fmt.Sprintf("promo-%03d", len(out)+1),
			ProductID:  productID,
			Discount:   float64(discount),
			ValidFrom:  from,
			ValidTo:    to,
			Conditions: "SMB",
		})
	}

	add("prod-vector", 10, baseDate.AddDate(0, 0, -1), baseDate.AddDate(0, 0, 5))
	add("prod-office-pack", 15, baseDate.AddDate(0, 0, -1), baseDate.AddDate(0, 0, 3))

	for len(out) < promotionCount {
		p := products[r.Intn(len(products))]
		var to time.Time
		switch {
		case len(out) < 17:
			to = baseDate.AddDate(0, 0, 1+r.Intn(6))
		case len(out) < 32:
			to = baseDate.AddDate(0, 0, 8+r.Intn(22))
		case len(out) < 36:
			to = baseDate.AddDate(0, 0, -1-r.Intn(30))
		default:
			to = baseDate.AddDate(0, 0, 40+r.Intn(30))
		}
		add(p.ID, 5+r.Intn(16), to.AddDate(0, 0, -30), to)
	}
	return out
}

func generateGroundTruth(clients []model.Client, purchases []model.Purchase, licenses []model.License, relations []model.Relation) []model.GroundTruthCase {
	owned := map[string]map[string]bool{}
	for _, p := range purchases {
		if owned[p.ClientID] == nil {
			owned[p.ClientID] = map[string]bool{}
		}
		owned[p.ClientID][p.ProductID] = true
	}
	for _, l := range licenses {
		if owned[l.ClientID] == nil {
			owned[l.ClientID] = map[string]bool{}
		}
		owned[l.ClientID][l.ProductID] = true
	}

	byFrom := map[string][]model.Relation{}
	for _, r := range relations {
		byFrom[r.FromID] = append(byFrom[r.FromID], r)
	}

	renewals := map[string][]string{}
	for _, l := range licenses {
		if l.Status != "active" {
			continue
		}
		d := int(l.End.Sub(baseDate).Hours() / 24)
		if d >= 0 && d <= 90 {
			renewals[l.ClientID] = append(renewals[l.ClientID], l.ProductID)
		}
	}

	out := []model.GroundTruthCase{
		{ClientID: "smb-001", ExpectedProduct: "prod-antivirus", ExpectedType: "renewal"},
		{ClientID: "smb-002", ExpectedProduct: "prod-vector", ExpectedType: "cross_sell"},
		{ClientID: "smb-003", ExpectedProduct: "prod-office-pack", ExpectedType: "upsell"},
		{ClientID: "smb-004", ExpectedProduct: "prod-backup", ExpectedType: "cross_sell"},
		{ClientID: "smb-005", ExpectedProduct: "prod-vector-pro", ExpectedType: "upsell"},
		{ClientID: "smb-006", ExpectedProduct: "prod-office-suite", ExpectedType: "fallback"},
	}

	appendRelations := func(clientID string, types map[string]string) bool {
		ids := make([]string, 0, len(owned[clientID]))
		for id := range owned[clientID] {
			ids = append(ids, id)
		}
		sort.Strings(ids)

		added := false
		for _, id := range ids {
			for _, r := range byFrom[id] {
				typ, ok := types[r.Type]
				if !ok {
					continue
				}
				out = append(out, model.GroundTruthCase{ClientID: clientID, ExpectedProduct: r.ToID, ExpectedType: typ})
				added = true
			}
		}
		return added
	}

	for _, c := range clients {
		if c.ID <= "smb-006" {
			continue
		}
		if ids := renewals[c.ID]; len(ids) > 0 {
			for _, id := range ids {
				out = append(out, model.GroundTruthCase{ClientID: c.ID, ExpectedProduct: id, ExpectedType: "renewal"})
			}
			continue
		}
		if appendRelations(c.ID, map[string]string{"upgrade": "upsell", "bundle": "upsell"}) {
			continue
		}
		if appendRelations(c.ID, map[string]string{
			"adjacent_task": "cross_sell", "workflow_next_step": "cross_sell",
			"accessory": "cross_sell", "bundle": "cross_sell",
		}) {
			continue
		}
		if appendRelations(c.ID, map[string]string{"analog": "analog"}) {
			continue
		}
		out = append(out, model.GroundTruthCase{ClientID: c.ID, ExpectedProduct: "prod-office-suite", ExpectedType: "fallback"})
	}
	return out
}
