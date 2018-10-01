package model

import (
	"database/sql"
	"fmt"
	"isucon8/isubank"
	"log"
	"time"

	"github.com/pkg/errors"
)

type Trade struct {
	ID        int64     `json:"id"`
	Amount    int64     `json:"amount"`
	Price     int64     `json:"price"`
	CreatedAt time.Time `json:"created_at"`
}

type CandlestickData struct {
	Time  time.Time `json:"time"`
	Open  int64     `json:"open"`
	Close int64     `json:"close"`
	High  int64     `json:"high"`
	Low   int64     `json:"low"`
}

func scanTrade(r RowScanner) (*Trade, error) {
	var v Trade
	if err := r.Scan(&v.ID, &v.Amount, &v.Price, &v.CreatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}

func GetTradeByID(d QueryExecuter, id int64) (*Trade, error) {
	return scanTrade(d.QueryRow("SELECT * FROM trade WHERE id = ?", id))
}

func GetTradesByLastID(d QueryExecuter, lastID int64) ([]*Trade, error) {
	rows, err := d.Query("SELECT * FROM trade WHERE id > ? ORDER BY id ASC", lastID)
	if err != nil {
		return nil, errors.Wrapf(err, "GetTradesByLastID Query failed. lastID:%d", lastID)
	}
	defer rows.Close()
	trades := []*Trade{}
	for rows.Next() {
		trade, err := scanTrade(rows)
		if err != nil {
			return nil, errors.Wrapf(err, "Scan failed.")
		}
		trades = append(trades, trade)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "rows.Err failed.")
	}
	return trades, nil
}

func GetCandlestickData(d QueryExecuter, mt time.Time, tf string) ([]CandlestickData, error) {
	query := fmt.Sprintf(`
		SELECT m.t, a.price, b.price, m.h, m.l
		FROM (
			SELECT
				STR_TO_DATE(DATE_FORMAT(created_at, '%s'), '%s') AS t,
				MIN(id) AS min_id,
				MAX(id) AS max_id,
				MAX(price) AS h,
				MIN(price) AS l
			FROM trade
			WHERE created_at >= ?
			GROUP BY t
		) m
		JOIN trade a ON a.id = m.min_id
		JOIN trade b ON b.id = m.max_id
		ORDER BY m.t
	`, tf, "%Y-%m-%d %H:%i:%s")
	rows, err := d.Query(query, mt)
	if err != nil {
		return nil, errors.Wrapf(err, "Query failed. query:%s, starttime:%s", query, mt)
	}
	defer rows.Close()
	datas := []CandlestickData{}
	for rows.Next() {
		var cd CandlestickData
		if err = rows.Scan(&cd.Time, &cd.Open, &cd.Close, &cd.High, &cd.Low); err != nil {
			return nil, errors.Wrapf(err, "Scan failed.")
		}
		datas = append(datas, cd)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "rows.Err failed.")
	}
	return datas, nil
}

func HasTradeChanceByOrder(d QueryExecuter, orderID int64) (bool, error) {
	order, err := getOrderByID(d, orderID)
	if err != nil {
		return false, err
	}

	lowest, err := GetLowestSellOrder(d)
	switch {
	case err == sql.ErrNoRows:
		return false, nil
	case err != nil:
		return false, errors.Wrap(err, "GetLowestSellOrder")
	}

	highest, err := GetHighestBuyOrder(d)
	switch {
	case err == sql.ErrNoRows:
		return false, nil
	case err != nil:
		return false, errors.Wrap(err, "GetHighestBuyOrder")
	}

	switch order.Type {
	case OrderTypeBuy:
		if lowest.Price <= order.Price {
			return true, nil
		}
	case OrderTypeSell:
		if order.Price <= highest.Price {
			return true, nil
		}
	default:
		return false, errors.Errorf("other type [%s]", order.Type)
	}
	return false, nil
}

func reserveOrder(d QueryExecuter, order *Order, price int64) (int64, error) {
	bank, err := Isubank(d)
	if err != nil {
		return 0, errors.Wrap(err, "isubank init failed")
	}
	logger, err := Logger(d)
	if err != nil {
		return 0, errors.Wrap(err, "logger init failed")
	}
	p := order.Amount * price
	if order.Type == OrderTypeBuy {
		p *= -1
	}

	id, err := bank.Reserve(order.User.BankID, p)
	if err != nil {
		if err == isubank.ErrCreditInsufficient {
			// 与信確保失敗した場合はorderを破棄する
			if _, derr := d.Exec(`UPDATE orders SET closed_at = ? WHERE id = ?`, time.Now(), order.ID); derr != nil {
				return 0, errors.Wrap(derr, "update buy_order for cancel")
			}
			lerr := logger.Send(order.Type+".delete", map[string]interface{}{
				"order_id": order.ID,
				"user_id":  order.UserID,
				"reason":   "reserve_failed",
			})
			if lerr != nil {
				log.Printf("[WARN] logger.Send failed. err:%s", lerr)
			}
			lerr = logger.Send(order.Type+".error", map[string]interface{}{
				"error":   err.Error(),
				"user_id": order.UserID,
				"amount":  order.Amount,
				"price":   price,
			})
			if lerr != nil {
				log.Printf("[WARN] logger.Send failed. err:%s", lerr)
			}
			return 0, err
		}
		return 0, errors.Wrap(err, "isubank.Reserve")
	}

	return id, nil
}

func commitReservedOrder(tx *sql.Tx, order *Order, targets []*Order, reserves []int64) error {
	bank, err := Isubank(tx)
	if err != nil {
		return errors.Wrap(err, "isubank init failed")
	}
	logger, err := Logger(tx)
	if err != nil {
		return errors.Wrap(err, "logger init failed")
	}
	defer func() {
		if len(reserves) > 0 {
			if err = bank.Cancel(reserves); err != nil {
				log.Printf("[WARN] isubank cancel failed. err:%s", err)
			}
		}
	}()
	res, err := tx.Exec(`INSERT INTO trade (amount, price, created_at) VALUES (?, ?, ?)`, order.Amount, order.Price, time.Now())
	if err != nil {
		return errors.Wrap(err, "insert trade")
	}
	tradeID, err := res.LastInsertId()
	if err != nil {
		return errors.Wrap(err, "lastInsertID for trade")
	}
	le := logger.Send("trade", map[string]interface{}{
		"trade_id": tradeID,
		"price":    order.Price,
		"amount":   order.Amount,
	})
	if le != nil {
		log.Printf("[WARN] logger.Send failed. err:%s", le)
	}
	for _, o := range append(targets, order) {
		if _, err = tx.Exec(`UPDATE orders SET trade_id = ?, closed_at = ? WHERE id = ?`, tradeID, time.Now(), o.ID); err != nil {
			return errors.Wrap(err, "update order for trade")
		}
		le := logger.Send(o.Type+".trade", map[string]interface{}{
			"order_id": o.ID,
			"price":    order.Price,
			"amount":   o.Amount,
			"user_id":  o.UserID,
			"trade_id": tradeID,
		})
		if le != nil {
			log.Printf("[WARN] logger.Send failed. err:%s", le)
		}
	}
	if err = bank.Commit(reserves); err != nil {
		return errors.Wrap(err, "commit")
	}
	reserves = reserves[:0]
	return nil
}

func tryTrade(tx *sql.Tx, orderID int64) error {
	order, err := getOpenOrderByID(tx, orderID)
	if err != nil {
		return err
	}

	restAmount := order.Amount
	unitPrice := order.Price
	reserves := make([]int64, 1, order.Amount+1)
	targets := make([]*Order, 0, order.Amount)

	reserves[0], err = reserveOrder(tx, order, unitPrice)
	if err != nil {
		return err
	}
	defer func() {
		if len(reserves) > 0 {
			bank, err := Isubank(tx)
			if err != nil {
				log.Printf("[WARN] isubank init failed. err:%s", err)
				return
			}
			if err = bank.Cancel(reserves); err != nil {
				log.Printf("[WARN] isubank cancel failed. err:%s", err)
			}
		}
	}()

	var targetOrders []*Order
	switch order.Type {
	case OrderTypeBuy:
		targetOrders, err = queryOrders(tx, `SELECT * FROM orders WHERE type = ? AND closed_at IS NULL AND price <= ? ORDER BY price ASC, created_at ASC, id ASC`, OrderTypeSell, order.Price)
	case OrderTypeSell:
		targetOrders, err = queryOrders(tx, `SELECT * FROM orders WHERE type = ? AND closed_at IS NULL AND price >= ? ORDER BY price DESC, created_at ASC, id ASC`, OrderTypeBuy, order.Price)
	}
	if err != nil {
		return errors.Wrap(err, "find target orders")
	}
	if len(targetOrders) == 0 {
		return ErrNoOrderForTrade
	}

	for _, to := range targetOrders {
		to, err = getOpenOrderByID(tx, to.ID)
		if err != nil {
			if err == ErrOrderAlreadyClosed {
				continue
			}
			return errors.Wrap(err, "GetOrderByIDWithLock  buy_order")
		}
		if to.Amount > restAmount {
			continue
		}
		rid, err := reserveOrder(tx, to, unitPrice)
		if err != nil {
			if err == isubank.ErrCreditInsufficient {
				continue
			}
			return err
		}
		reserves = append(reserves, rid)
		targets = append(targets, to)
		restAmount -= to.Amount
		if restAmount == 0 {
			break
		}
	}
	if restAmount > 0 {
		return ErrNoOrderForTrade
	}
	// cancelをしたいので
	r := make([]int64, len(reserves))
	copy(r, reserves)
	reserves = reserves[:0]
	return commitReservedOrder(tx, order, targets, r)
}

func RunTrade(db *sql.DB) error {
	lowestSellOrder, err := GetLowestSellOrder(db)
	switch {
	case err == sql.ErrNoRows:
		// 売り注文が無いため成立しない
		return nil
	case err != nil:
		return errors.Wrap(err, "GetLowestSellOrder")
	}

	highestBuyOrder, err := GetHighestBuyOrder(db)
	switch {
	case err == sql.ErrNoRows:
		// 買い注文が無いため成立しない
		return nil
	case err != nil:
		return errors.Wrap(err, "GetHighestBuyOrder")
	}

	if lowestSellOrder.Price > highestBuyOrder.Price {
		// 最安の売値が最高の買値よりも高いため成立しない
		return nil
	}

	candidates := make([]int64, 0, 2)
	if lowestSellOrder.Amount > highestBuyOrder.Amount {
		candidates = append(candidates, lowestSellOrder.ID, highestBuyOrder.ID)
	} else {
		candidates = append(candidates, highestBuyOrder.ID, lowestSellOrder.ID)
	}

	for _, orderID := range candidates {
		err := func() error {
			tx, err := db.Begin()
			if err != nil {
				return errors.Wrap(err, "begin transaction failed")
			}
			err = tryTrade(tx, orderID)
			switch err {
			case nil, ErrNoOrderForTrade, ErrOrderAlreadyClosed, isubank.ErrCreditInsufficient:
				tx.Commit()
			default:
				tx.Rollback()
			}
			return err
		}()
		switch err {
		case nil:
			// トレード成立したため次の取引を行う
			return RunTrade(db)
		case ErrNoOrderForTrade, ErrOrderAlreadyClosed:
			// 注文個数の多い方で成立しなかったので少ない方で試す
			continue
		default:
			return err
		}
	}
	// 個数のが不足していて不成立
	return nil
}
