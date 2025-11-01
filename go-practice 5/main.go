package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Product struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Price    int64  `json:"price"`
}

func main() {
	db, err := sql.Open("sqlite", "file:products.db?cache=shared&mode=rwc")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		log.Fatalf("enable fkeys: %v", err)
	}

	if err := ensureSchema(db); err != nil {
		log.Fatalf("schema: %v", err)
	}
	if err := seedIfEmpty(db); err != nil {
		log.Fatalf("seed: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /products", func(w http.ResponseWriter, r *http.Request) {
		handleGetProducts(w, r, db)
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", withJSONHeader(mux)))
}

func withJSONHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

func ensureSchema(db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS categories (
  id   INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS products (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL,
  category_id INTEGER NOT NULL,
  price       INTEGER NOT NULL,
  FOREIGN KEY(category_id) REFERENCES categories(id) ON UPDATE CASCADE ON DELETE RESTRICT
);
`
	_, err := db.Exec(schema)
	return err
}

func seedIfEmpty(db *sql.DB) error {
	var cnt int
	if err := db.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO categories(name) VALUES ('phones'), ('laptops'), ('accessories')`); err != nil {
		return err
	}

	rows, err := tx.Query(`SELECT id, name FROM categories`)
	if err != nil {
		return err
	}
	defer rows.Close()

	cat := map[string]int64{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		cat[name] = id
	}
	if err := rows.Err(); err != nil {
		return err
	}

	type item struct {
		n string
		c string
		p int64
	}
	items := []item{
		{"iPhone", "phones", 400000},
		{"Galaxy", "phones", 300000},
		{"Pixel", "phones", 250000},
		{"MacBook Air", "laptops", 550000},
		{"ThinkPad", "laptops", 450000},
		{"USB-C Cable", "accessories", 5000},
		{"Power Bank", "accessories", 15000},
		{"Gaming Laptop", "laptops", 750000},
	}
	for _, it := range items {
		if _, err := tx.Exec(`INSERT INTO products(name, category_id, price) VALUES (?, ?, ?)`,
			it.n, cat[it.c], it.p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func handleGetProducts(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	q := r.URL.Query()

	category := strings.TrimSpace(q.Get("category"))
	minStr := strings.TrimSpace(q.Get("min_price"))
	maxStr := strings.TrimSpace(q.Get("max_price"))
	sort := strings.TrimSpace(q.Get("sort"))
	limitStr := strings.TrimSpace(q.Get("limit"))
	offsetStr := strings.TrimSpace(q.Get("offset"))

	var (
		minPtr, maxPtr *int64
		limitPtr       *int
		offsetPtr      *int
	)
	if minStr != "" {
		v, err := strconv.ParseInt(minStr, 10, 64)
		if err != nil {
			httpBadRequest(w, "invalid min_price"); return
		}
		minPtr = &v
	}
	if maxStr != "" {
		v, err := strconv.ParseInt(maxStr, 10, 64)
		if err != nil {
			httpBadRequest(w, "invalid max_price"); return
		}
		maxPtr = &v
	}
	if limitStr != "" {
		v, err := strconv.Atoi(limitStr)
		if err != nil || v < 0 {
			httpBadRequest(w, "invalid limit"); return
		}
		limitPtr = &v
	}
	if offsetStr != "" {
		v, err := strconv.Atoi(offsetStr)
		if err != nil || v < 0 {
			httpBadRequest(w, "invalid offset"); return
		}
		offsetPtr = &v
	}

	sqlStr := `
SELECT p.id, p.name, c.name AS category, p.price
FROM products p
JOIN categories c ON c.id = p.category_id
`
	where := []string{}
	args := []any{}

	if category != "" {
		where = append(where, `c.name = ?`)
		args = append(args, category)
	}
	if minPtr != nil {
		where = append(where, `p.price >= ?`)
		args = append(args, *minPtr)
	}
	if maxPtr != nil {
		where = append(where, `p.price <= ?`)
		args = append(args, *maxPtr)
	}
	if len(where) > 0 {
		sqlStr += "WHERE " + strings.Join(where, " AND ") + "\n"
	}

	orderBy := "p.id ASC"
	switch sort {
	case "price_asc":
		orderBy = "p.price ASC"
	case "price_desc":
		orderBy = "p.price DESC"
	case "", "id", "id_asc":
	default:
		httpBadRequest(w, "invalid sort (use price_asc or price_desc)"); return
	}
	sqlStr += "ORDER BY " + orderBy + "\n"

	if limitPtr != nil {
		sqlStr += "LIMIT ?\n"
		args = append(args, *limitPtr)
	}
	if offsetPtr != nil {
		sqlStr += "OFFSET ?\n"
		args = append(args, *offsetPtr)
	}

	start := time.Now()
	rows, err := db.Query(sqlStr, args...)
	elapsed := time.Since(start)
	w.Header().Set("X-Query-Time", elapsed.String())

	if err != nil {
		httpServerError(w, "query error"); log.Println("query:", err); return
	}
	defer rows.Close()

	var out []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Category, &p.Price); err != nil {
			httpServerError(w, "scan error"); log.Println("scan:", err); return
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		httpServerError(w, "rows error"); log.Println("rows:", err); return
	}

	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func httpBadRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": msg})
}

func httpServerError(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": msg})
}
