package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"writekit/internal/httplog"
)

const (
	dbMaxRows    = 500
	dbQueryLimit = 5 * time.Second
)

type dbTableInfo struct {
	Name    string `json:"name"`
	Rows    int64  `json:"rows"`
	Columns int    `json:"columns"`
	Type    string `json:"type"`
}

type dbRowsResponse struct {
	Columns []string          `json:"columns"`
	Types   []string          `json:"types"`
	Rows    [][]any           `json:"rows"`
	Total   int64             `json:"total"`
	Limit   int               `json:"limit"`
	Offset  int               `json:"offset"`
	Schema  []dbColumnInfo    `json:"schema,omitempty"`
}

type dbColumnInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	NotNull bool   `json:"not_null"`
	PK      bool   `json:"pk"`
}

type dbQueryResponse struct {
	Columns  []string `json:"columns"`
	Rows     [][]any  `json:"rows"`
	Truncated bool    `json:"truncated"`
}

func (h *Handler) resolveViewerTenantID(r *http.Request) (string, bool) {
	if h.Config.Local {
		site := h.localSite()
		if site == nil {
			return "", false
		}
		return site.ID, true
	}
	user := userFromContext(r.Context())
	if user == nil {
		return "", false
	}
	site, err := h.DB.GetTenantByUser(r.Context(), user.ID)
	if err != nil || site == nil {
		return "", false
	}
	return site.ID, true
}

func (h *Handler) DBTables(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	tenantID, ok := h.resolveViewerTenantID(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no site"})
		return
	}
	db, err := h.Pool.Get(tenantID)
	if err != nil {
		log.Error("db viewer: open", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	rows, err := db.DB.QueryContext(r.Context(),
		`SELECT name, type FROM sqlite_master WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := []dbTableInfo{}
	for rows.Next() {
		var info dbTableInfo
		if err := rows.Scan(&info.Name, &info.Type); err != nil {
			continue
		}
		out = append(out, info)
	}
	rows.Close()

	for i := range out {
		_ = db.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM \""+out[i].Name+"\"").Scan(&out[i].Rows)
		colRows, err := db.DB.QueryContext(r.Context(), "PRAGMA table_info(\""+out[i].Name+"\")")
		if err == nil {
			n := 0
			for colRows.Next() {
				n++
			}
			colRows.Close()
			out[i].Columns = n
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) DBTableRows(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	tenantID, ok := h.resolveViewerTenantID(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no site"})
		return
	}
	db, err := h.Pool.Get(tenantID)
	if err != nil {
		log.Error("db viewer: open", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	name := chi.URLParam(r, "name")
	if !isSafeIdentifier(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid table name"})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > dbMaxRows {
		limit = 100
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	schema, err := readTableSchema(r, db.DB, name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var total int64
	_ = db.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM \""+name+"\"").Scan(&total)

	rows, err := db.DB.QueryContext(r.Context(), "SELECT * FROM \""+name+"\" LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	cols, types, data, err := scanAllRows(rows)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, dbRowsResponse{
		Columns: cols,
		Types:   types,
		Rows:    data,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		Schema:  schema,
	})
}

func (h *Handler) DBQuery(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	tenantID, ok := h.resolveViewerTenantID(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no site"})
		return
	}
	db, err := h.Pool.Get(tenantID)
	if err != nil {
		log.Error("db viewer: open", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var body struct {
		SQL string `json:"sql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	q := strings.TrimSpace(body.SQL)
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty query"})
		return
	}
	if !isReadOnlyQuery(q) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only SELECT / WITH / PRAGMA / EXPLAIN queries are allowed"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), dbQueryLimit)
	defer cancel()

	rows, err := db.DB.QueryContext(ctx, q)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	cols, _, data, err := scanAllRowsLimited(rows, dbMaxRows)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, dbQueryResponse{
		Columns:   cols,
		Rows:      data,
		Truncated: len(data) >= dbMaxRows,
	})
}

func (h *Handler) DBExport(w http.ResponseWriter, r *http.Request) {
	log := httplog.FromContext(r.Context())
	tenantID, ok := h.resolveViewerTenantID(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no site"})
		return
	}
	db, err := h.Pool.Get(tenantID)
	if err != nil {
		log.Error("db export: open", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	tmp, err := os.CreateTemp("", "writekit-export-*.db")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	tmpPath := tmp.Name()
	tmp.Close()
	os.Remove(tmpPath)
	defer os.Remove(tmpPath)

	escaped := strings.ReplaceAll(tmpPath, "'", "''")
	if _, err := db.DB.ExecContext(r.Context(), fmt.Sprintf("VACUUM INTO '%s'", escaped)); err != nil {
		log.Error("db export: vacuum", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "snapshot failed: " + err.Error()})
		return
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("%s-%s.db", filepath.Base(tenantID), time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, f); err != nil {
		log.Warn("db export: stream", "err", err)
	}
}

func readTableSchema(r *http.Request, db *sql.DB, name string) ([]dbColumnInfo, error) {
	rows, err := db.QueryContext(r.Context(), "PRAGMA table_info(\""+name+"\")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dbColumnInfo{}
	for rows.Next() {
		var cid int
		var colName, colType string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &colName, &colType, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out = append(out, dbColumnInfo{Name: colName, Type: colType, NotNull: notnull == 1, PK: pk > 0})
	}
	if len(out) == 0 {
		return nil, &dbErr{msg: "table not found"}
	}
	return out, nil
}

type dbErr struct{ msg string }

func (e *dbErr) Error() string { return e.msg }

func scanAllRows(rows *sql.Rows) ([]string, []string, [][]any, error) {
	return scanAllRowsLimited(rows, dbMaxRows)
}

func scanAllRowsLimited(rows *sql.Rows, limit int) ([]string, []string, [][]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, nil, err
	}
	colTypes, _ := rows.ColumnTypes()
	types := make([]string, len(cols))
	for i, t := range colTypes {
		if t != nil {
			types[i] = t.DatabaseTypeName()
		}
	}
	out := [][]any{}
	for rows.Next() && len(out) < limit {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, nil, err
		}
		for i, v := range vals {
			switch b := v.(type) {
			case []byte:
				vals[i] = string(b)
			default:
				_ = b
			}
		}
		out = append(out, vals)
	}
	return cols, types, out, nil
}

func isSafeIdentifier(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !(r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

func isReadOnlyQuery(q string) bool {
	trimmed := strings.TrimSpace(q)
	trimmed = strings.TrimRight(trimmed, ";")
	if strings.Contains(trimmed, ";") {
		return false
	}
	upper := strings.ToUpper(trimmed)
	for _, p := range []string{"SELECT", "WITH", "PRAGMA", "EXPLAIN"} {
		if strings.HasPrefix(upper, p+" ") || upper == p {
			return true
		}
	}
	return false
}

