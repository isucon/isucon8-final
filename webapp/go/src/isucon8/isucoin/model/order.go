package model

import (
	"database/sql"
	"isucon8/isubank"
	"time"

	"github.com/pkg/errors"
)

const (
	OrderTypeBuy  = "buy"
	OrderTypeSell = "sell"
)

//go:generate scanner
type Order struct {
	ID        int64      `json:"id"`
	Type      string     `json:"type"`
	UserID    int64      `json:"user_id"`
	Amount    int64      `json:"amount"`
	Price     int64      `json:"price"`
	ClosedAt  *time.Time `json:"closed_at"`
	TradeID   int64      `json:"trade_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	User      *User      `json:"user,omitempty"`
	Trade     *Trade     `json:"trade,omitempty"`
}

func GetOrdersByUserID(d QueryExecutor, userID int64) ([]*Order, error) {
	return scanOrders(d.Query("SELECT * FROM orders WHERE user_id = ? AND (closed_at IS NULL OR trade_id IS NOT NULL) ORDER BY created_at ASC", userID))
}

func GetOrdersByUserIDAndLastTradeId(d QueryExecutor, userID int64, tradeID int64) ([]*Order, error) {
	return scanOrders(d.Query(`SELECT * FROM orders WHERE user_id = ? AND trade_id IS NOT NULL AND trade_id > ? ORDER BY created_at ASC`, userID, tradeID))
}

func getOpenOrderByID(tx *sql.Tx, id int64) (*Order, error) {
	order, err := getOrderByIDWithLock(tx, id)
	if err != nil {
		return nil, errors.Wrap(err, "getOrderByIDWithLock sell_order")
	}
	if order.ClosedAt != nil {
		return nil, ErrOrderAlreadyClosed
	}
	order.User, err = getUserByIDWithLock(tx, order.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "getUserByIDWithLock sell user")
	}
	return order, nil
}

func GetOrderByID(d QueryExecutor, id int64) (*Order, error) {
	return scanOrder(d.Query("SELECT * FROM orders WHERE id = ?", id))
}

func getOrderByIDWithLock(tx *sql.Tx, id int64) (*Order, error) {
	return scanOrder(tx.Query("SELECT * FROM orders WHERE id = ? FOR UPDATE", id))
}

func GetLowestSellOrder(d QueryExecutor) (*Order, error) {
	return scanOrder(d.Query("SELECT * FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price ASC, created_at ASC LIMIT 1", OrderTypeSell))
}

func GetHighestBuyOrder(d QueryExecutor) (*Order, error) {
	return scanOrder(d.Query("SELECT * FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price DESC, created_at ASC LIMIT 1", OrderTypeBuy))
}

func FetchOrderRelation(d QueryExecutor, order *Order) error {
	var err error
	order.User, err = GetUserByID(d, order.UserID)
	if err != nil {
		return errors.Wrapf(err, "GetUserByID failed. id")
	}
	if order.TradeID > 0 {
		order.Trade, err = GetTradeByID(d, order.TradeID)
		if err != nil {
			return errors.Wrapf(err, "GetTradeByID failed. id")
		}
	}
	return nil
}

func AddOrder(tx *sql.Tx, ot string, userID, amount, price int64) (*Order, error) {
	if amount <= 0 || price <= 0 {
		return nil, ErrParameterInvalid
	}
	user, err := getUserByIDWithLock(tx, userID)
	if err != nil {
		return nil, errors.Wrapf(err, "getUserByIDWithLock failed. id:%d", userID)
	}
	bank, err := Isubank(tx)
	if err != nil {
		return nil, errors.Wrap(err, "newIsubank failed")
	}
	switch ot {
	case OrderTypeBuy:
		totalPrice := price * amount
		if err = bank.Check(user.BankID, totalPrice); err != nil {
			sendLog(tx, "buy.error", map[string]interface{}{
				"error":   err.Error(),
				"user_id": user.ID,
				"amount":  amount,
				"price":   price,
			})
			if err == isubank.ErrCreditInsufficient {
				return nil, ErrCreditInsufficient
			}
			return nil, errors.Wrap(err, "isubank check failed")
		}
	case OrderTypeSell:
		// TODO 椅子の保有チェック
	default:
		return nil, ErrParameterInvalid
	}
	res, err := tx.Exec(`INSERT INTO orders (type, user_id, amount, price, created_at) VALUES (?, ?, ?, ?, NOW(6))`, ot, user.ID, amount, price)
	if err != nil {
		return nil, errors.Wrap(err, "insert order failed")
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, errors.Wrap(err, "get order_id failed")
	}
	sendLog(tx, ot+".order", map[string]interface{}{
		"order_id": id,
		"user_id":  user.ID,
		"amount":   amount,
		"price":    price,
	})
	return GetOrderByID(tx, id)
}

func DeleteOrder(tx *sql.Tx, userID, orderID int64, reason string) error {
	user, err := getUserByIDWithLock(tx, userID)
	if err != nil {
		return errors.Wrapf(err, "getUserByIDWithLock failed. id:%d", userID)
	}
	order, err := getOrderByIDWithLock(tx, orderID)
	switch {
	case err == sql.ErrNoRows:
		return ErrOrderNotFound
	case err != nil:
		return errors.Wrapf(err, "getOrderByIDWithLock failed. id")
	case order.UserID != user.ID:
		return ErrOrderNotFound
	case order.ClosedAt != nil:
		return ErrOrderAlreadyClosed
	}
	return cancelOrder(tx, order, reason)
}

func cancelOrder(d QueryExecutor, order *Order, reason string) error {
	if _, err := d.Exec(`UPDATE orders SET closed_at = NOW(6) WHERE id = ?`, order.ID); err != nil {
		return errors.Wrap(err, "update orders for cancel")
	}
	sendLog(d, order.Type+".delete", map[string]interface{}{
		"order_id": order.ID,
		"user_id":  order.UserID,
		"reason":   reason,
	})
	return nil
}
