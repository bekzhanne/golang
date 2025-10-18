package main

import (
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type User struct {
	ID      int     `db:"id"`
	Name    string  `db:"name"`
	Email   string  `db:"email"`
	Balance float64 `db:"balance"`
}

var db *sqlx.DB

func main() {
	var err error
	connStr := "host=localhost port=5430 user=user password=password dbname=mydatabase sslmode=disable"

	db, err = sqlx.Connect("postgres", connStr)
	if err != nil {
		log.Fatal("Error for connection to db:", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	fmt.Println("Connection to db is susccess")

	fmt.Println("\n--- Adding new users ---")

	user1 := User{
		Name:    "Arman",
		Email:   "arman@kbtu.kz",
		Balance: 1000.0,
	}
	err = InsertUser(db, user1)
	if err != nil {
		log.Println("Error to add user1:", err)
	} else {
		fmt.Println("user1 added to db")
	}

	user2 := User{
		Name:    "Anara",
		Email:   "anara@kbtu.kz",
		Balance: 500.0,
	}
	err = InsertUser(db, user2)
	if err != nil {
		log.Println("Error to add user1", err)
	} else {
		fmt.Println("user2 added to db")
	}

	fmt.Println("\n--- All the users ---")
	users, err := GetAllUsers(db)
	if err != nil {
		log.Fatal("error to get users:", err)
	}
	for _, u := range users {
		fmt.Printf("ID: %d, Name: %s, Email: %s, Balance: %.2f\n",
			u.ID, u.Name, u.Email, u.Balance)
	}

	fmt.Println("\n--- Get user by ID ---")
	user, err := GetUserByID(db, 1)
	if err != nil {
		log.Println("error:", err)
	} else {
		fmt.Printf("found: %s (Email: %s, balance: %.2f)\n",
			user.Name, user.Email, user.Balance)
	}

	fmt.Println("\n--- send money (ID 1 → ID 2, sum: 200) ---")
	err = TransferBalance(db, 1, 2, 200.0)
	if err != nil {
		log.Println("error transaction", err)
	} else {
		fmt.Println("tranaction succeeded!")
	}

	fmt.Println("\n--- balanca after transaction ---")
	users, _ = GetAllUsers(db)
	for _, u := range users {
		fmt.Printf("ID: %d, name: %s, balance: %.2f\n",
			u.ID, u.Name, u.Balance)
	}

	fmt.Println("\n--- try to send more than they have ---")
	err = TransferBalance(db, 1, 2, 10000.0)
	if err != nil {
		fmt.Println("transaction error:", err)
	}
}

func InsertUser(db *sqlx.DB, user User) error {
	query := `INSERT INTO users (name, email, balance) 
	          VALUES (:name, :email, :balance)`

	_, err := db.NamedExec(query, user)
	return err
}

func GetAllUsers(db *sqlx.DB) ([]User, error) {
	var users []User
	query := `SELECT id, name, email, balance FROM users`

	err := db.Select(&users, query)
	return users, err
}

func GetUserByID(db *sqlx.DB, id int) (User, error) {
	var user User
	query := `SELECT id, name, email, balance FROM users WHERE id = $1`

	err := db.Get(&user, query, id)
	return user, err
}

func TransferBalance(db *sqlx.DB, fromID int, toID int, amount float64) error {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("cannot start transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	var senderBalance float64
	err = tx.Get(&senderBalance,
		`SELECT balance FROM users WHERE id = $1`, fromID)
	if err != nil {
		return fmt.Errorf("sender not found: %w", err)
	}

	if senderBalance < amount {
		return fmt.Errorf("low money (balance: %.2f, need: %.2f)",
			senderBalance, amount)
	}

	var receiverExists bool
	err = tx.Get(&receiverExists,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, toID)
	if err != nil || !receiverExists {
		return fmt.Errorf("user not found")
	}

	_, err = tx.Exec(
		`UPDATE users SET balance = balance - $1 WHERE id = $2`,
		amount, fromID)
	if err != nil {
		return fmt.Errorf("cannot take your money: %w", err)
	}

	_, err = tx.Exec(
		`UPDATE users SET balance = balance + $1 WHERE id = $2`,
		amount, toID)
	if err != nil {
		return fmt.Errorf("cannot zachislit' money: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit error: %w", err)
	}

	return nil
}
