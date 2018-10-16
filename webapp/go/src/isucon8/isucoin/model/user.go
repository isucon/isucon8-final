package model

import (
	"database/sql"
	"time"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

//go:generate scanner
type User struct {
	ID        int64     `json:"id"`
	BankID    string    `json:"-"`
	Name      string    `json:"name"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"-"`
}

func GetUserByID(d QueryExecutor, id int64) (*User, error) {
	return scanUser(d.Query("SELECT * FROM user WHERE id = ?", id))
}

func getUserByIDWithLock(tx *sql.Tx, id int64) (*User, error) {
	return scanUser(tx.Query("SELECT * FROM user WHERE id = ? FOR UPDATE", id))
}

func UserSignup(tx *sql.Tx, name, bankID, password string) error {
	bank, err := Isubank(tx)
	if err != nil {
		return err
	}
	// bankIDの検証
	if err = bank.Check(bankID, 0); err != nil {
		return ErrBankUserNotFound
	}
	pass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if res, err := tx.Exec(`INSERT INTO user (bank_id, name, password, created_at) VALUES (?, ?, ?, NOW(6))`, bankID, name, pass); err != nil {
		if mysqlError, ok := err.(*mysql.MySQLError); ok {
			if mysqlError.Number == 1062 {
				return ErrBankUserConflict
			}
		}
		return err
	} else {
		userID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		sendLog(tx, "signup", map[string]interface{}{
			"bank_id": bankID,
			"user_id": userID,
			"name":    name,
		})
	}
	return nil
}

func UserLogin(d QueryExecutor, bankID, password string) (*User, error) {
	user, err := scanUser(d.Query("SELECT * FROM user WHERE bank_id = ?", bankID))
	switch {
	case err == sql.ErrNoRows:
		return nil, ErrUserNotFound
	case err != nil:
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	sendLog(d, "signin", map[string]interface{}{
		"user_id": user.ID,
	})
	return user, nil
}
