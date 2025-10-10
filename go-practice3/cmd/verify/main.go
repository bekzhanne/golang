package main

import (
"database/sql"
"fmt"
"log"

_ "modernc.org/sqlite"
)

func main() {
db, err := sql.Open("sqlite", "./expense.db")
if err != nil {
log.Fatal("open:", err)
}
defer db.Close()

if err := db.Ping(); err != nil {
log.Fatal("ping:", err)
}

var cnt int
err = db.QueryRow(`
SELECT COUNT(*)
FROM sqlite_master
WHERE type='table'
  AND name IN ('users','categories','expenses')
`).Scan(&cnt)
if err != nil {
log.Fatal("query:", err)
}

fmt.Printf("OK. Tables found: %d (users, categories, expenses)\n", cnt)
}
