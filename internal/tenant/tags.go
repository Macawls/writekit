package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type TagCount struct {
	Name  string
	Slug  string
	Count int
}

func ParseTags(raw string) []string {
	if raw == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	out := tags[:0]
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func SlugifyTag(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	prevDash := true
	for _, r := range strings.ToLower(name) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	s := b.String()
	return strings.Trim(s, "-")
}

func (db *DB) ListTagCounts(ctx context.Context, includeNonPublic bool) ([]TagCount, error) {
	query := `SELECT tags FROM pages WHERE status = 'published' AND visibility = 'public'`
	if includeNonPublic {
		query = `SELECT tags FROM pages WHERE (status = 'published' OR status = 'draft')`
	}
	rows, err := db.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list tag counts: %w", err)
	}
	defer rows.Close()

	counts := map[string]*TagCount{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		for _, name := range ParseTags(raw) {
			slug := SlugifyTag(name)
			if slug == "" {
				continue
			}
			if tc, ok := counts[slug]; ok {
				tc.Count++
			} else {
				counts[slug] = &TagCount{Name: name, Slug: slug, Count: 1}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]TagCount, 0, len(counts))
	for _, tc := range counts {
		out = append(out, *tc)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (db *DB) PagesByTagSlug(ctx context.Context, slug string, includeNonPublic bool) ([]Page, string, error) {
	query := pageSelect + ` WHERE status = 'published' AND visibility = 'public' ORDER BY COALESCE(published_at, created_at) DESC`
	if includeNonPublic {
		query = pageSelect + ` WHERE (status = 'published' OR status = 'draft') ORDER BY COALESCE(published_at, created_at, updated_at) DESC`
	}
	rows, err := db.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, "", fmt.Errorf("pages by tag %s: %w", slug, err)
	}
	defer rows.Close()

	var pages []Page
	var displayName string
	for rows.Next() {
		p, err := scanPageRow(rows)
		if err != nil {
			return nil, "", err
		}
		for _, name := range ParseTags(p.Tags) {
			if SlugifyTag(name) == slug {
				if displayName == "" {
					displayName = name
				}
				pages = append(pages, *p)
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	return pages, displayName, nil
}
