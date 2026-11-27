package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type xmlNode struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Text     string
	Children []xmlNode
}

// ImportRegistryXML imports a software registry export. The registry has no
// stable public API, so the parser maps known field names (Russian and English)
// and tolerates nested records.
func ImportRegistryXML(ctx context.Context, dbc *sql.DB, src io.Reader) (VendorImportResult, error) {
	var res VendorImportResult
	dec := xml.NewDecoder(src)
	var root xmlNode
	stack := []*xmlNode{&root}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return res, fmt.Errorf("parse xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, xmlNode{Name: t.Name, Attrs: t.Attr})
			stack = append(stack, &parent.Children[len(parent.Children)-1])
		case xml.CharData:
			cur := stack[len(stack)-1]
			if text := strings.TrimSpace(string(t)); text != "" {
				if cur.Text != "" {
					cur.Text += " "
				}
				cur.Text += text
			}
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		}
	}

	records := collectRegistryRecords(root)
	seen := map[string]bool{}
	for _, rec := range records {
		product := strings.TrimSpace(rec["name"])
		vendor := strings.TrimSpace(rec["vendor"])
		if product == "" {
			res.Skipped++
			continue
		}
		key := strings.ToLower(vendor + "|" + product)
		if seen[key] {
			res.Skipped++
			continue
		}
		seen[key] = true
		aliases := splitAliases(rec["aliases"])
		aliasJSON, _ := json.Marshal(aliases)
		platformJSON, _ := json.Marshal(splitAliases(rec["platforms"]))
		var exists bool
		if err := dbc.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM vendor_products
			WHERE vendor = $1 AND product = $2 AND edition = '')`,
			vendor, product).Scan(&exists); err != nil {
			return res, err
		}
		if _, err := dbc.ExecContext(ctx, `
			INSERT INTO vendor_products
				(vendor, product, family, edition, platforms, registry_number, source_url, aliases)
			VALUES ($1, $2, '', '', $3::jsonb, $4, $5, $6::jsonb)
			ON CONFLICT (vendor, product, edition) DO UPDATE SET
				platforms = EXCLUDED.platforms,
				registry_number = EXCLUDED.registry_number,
				source_url = EXCLUDED.source_url,
				aliases = EXCLUDED.aliases,
				updated_at = now()`,
			vendor, product, string(platformJSON), strings.TrimSpace(rec["registry"]),
			strings.TrimSpace(rec["url"]), string(aliasJSON)); err != nil {
			return res, fmt.Errorf("upsert registry product: %w", err)
		}
		if exists {
			res.Updated++
		} else {
			res.Added++
		}
	}
	return res, nil
}

func collectRegistryRecords(root xmlNode) []map[string]string {
	var out []map[string]string
	var walk func(n xmlNode, depth int)
	walk = func(n xmlNode, depth int) {
		fields := map[string]string{}
		collectFields(n, &fields, 0)
		if fields["name"] != "" && (fields["vendor"] != "" || fields["registry"] != "") {
			out = append(out, fields)
			return
		}
		for _, child := range n.Children {
			walk(child, depth+1)
		}
	}
	for _, child := range root.Children {
		walk(child, 0)
	}
	return out
}

func collectFields(n xmlNode, fields *map[string]string, depth int) {
	if depth > 4 {
		return
	}
	key := normalizeField(n.Name.Local)
	if key != "" && n.Text != "" {
		if (*fields)[key] == "" {
			(*fields)[key] = n.Text
		}
	}
	for _, a := range n.Attrs {
		if k := normalizeField(a.Name.Local); k != "" && a.Value != "" {
			if (*fields)[k] == "" {
				(*fields)[k] = a.Value
			}
		}
	}
	for _, child := range n.Children {
		collectFields(child, fields, depth+1)
	}
}

func normalizeField(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.NewReplacer("_", "", "-", "", " ", "", ".", "").Replace(n)
	switch {
	case strings.Contains(n, "реестр") && (strings.Contains(n, "номер") || strings.Contains(n, "запис")):
		return "registry"
	case n == "number" || n == "regnumber" || n == "registrynumber":
		return "registry"
	case strings.Contains(n, "наимен") || strings.Contains(n, "назван"):
		return "name"
	case n == "name" || n == "product" || n == "productname" || n == "software":
		return "name"
	case strings.Contains(n, "правооблад") || strings.Contains(n, "вендор"):
		return "vendor"
	case n == "vendor" || n == "owner" || n == "company" || n == "rightsholder":
		return "vendor"
	case strings.Contains(n, "класс") || n == "class" || n == "category":
		return "category"
	case strings.Contains(n, "ссылк") || strings.Contains(n, "адрес") || strings.Contains(n, "страниц") || strings.Contains(n, "url"):
		return "url"
	case n == "url" || n == "link" || n == "sourceurl" || n == "page":
		return "url"
	case strings.Contains(n, "платформ") || n == "os" || n == "platforms" || n == "platform":
		return "platforms"
	case strings.Contains(n, "альтернатив") || strings.Contains(n, "предыдущ") || n == "aliases":
		return "aliases"
	}
	return ""
}

func splitAliases(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	raw = strings.Trim(raw, "[]")
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';' || r == '|' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
