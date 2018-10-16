<?php

use Isucon8\PDOWrapper;
use Isucon8\PDOWrapperTxn;
use Isucon8\Isubank\CreditInsufficientException as IsubankCreditInsufficientException;

class JsonableTrade extends ArrayObject implements JsonSerializable {
    public function jsonSerialize() {
        return [
            'id'         => (int)$this['id'],
            'amount'     => (int)$this['amount'],
            'price'      => (int)$this['price'],
            'created_at' => (string)(new DateTime($this['created_at']))->format('Y-m-d\TH:i:sP'),
        ];
    }
}

class JsonableCandlestickData extends ArrayObject implements JsonSerializable {
    public function jsonSerialize() {
        return [
            'time'  => (string)(new DateTime($this['t']))->format('Y-m-d\TH:i:sP'),
            'open'  => (int)$this['open'],
            'close' => (int)$this['close'],
            'high'  => (int)$this['h'],
            'low'   => (int)$this['l'],
        ];
    }
}

function reformTrade(array $trade): JsonableTrade {
    return new JsonableTrade($trade);
}

function GetTradeByID(PDOWrapper $dbh, int $id): JsonableTrade {
    return reformTrade($dbh->select_row('SELECT * FROM trade WHERE id = ?', $id));
}

function GetLatestTrade(PDOWrapper $dbh): JsonableTrade {
    return reformTrade($dbh->select_row('SELECT * FROM trade ORDER BY id DESC'));
}

function GetCandlestickData(PDOWrapper $dbh, DateTime $mt, string $tf): array {
    $sql = <<< SQL
SELECT m.t, a.price AS open, b.price AS close, m.h, m.l
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
SQL;

    $query = sprintf($sql, $tf, '%Y-%m-%d %H:%i:%s');

    $rows = [];
    foreach ($dbh->select_all($query, $mt->format('Y-m-d H:i:s')) as $row) {
        $rows[] = new JsonableCandlestickData($row);
    }
    return $rows;
}

function HasTradeChanceByOrder(PDOWrapper $dbh, int $order_id): bool {
    $order = null;
    try {
        $order = GetOrderByID($dbh, $order_id);
    } catch(\Throwable $throwable) {
        throw $throwable;
    }

    $lowest = null;
    try {
        $lowest = GetLowestSellOrder($dbh);
    } catch(Isucon8\NoRowsException $e) {
        return false;
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: GetLowestSellOrder failed', $throwable->getMessage()));
        throw $throwable;
    }

    $highest = null;
    try {
        $highest = GetHighestBuyOrder($dbh);
    } catch(Isucon8\NoRowsException $e) {
        return false;
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: GetHighestBuyOrder failed', $throwable->getMessage()));
        throw $throwable;
    }


    switch ($order['type']) {
    case ORDER_TYPE_BUY:
        if ($lowest['price'] <= $order['price']) {
            return true;
        }
        break;
    case ORDER_TYPE_SELL:
        if ($order['price'] <= $highest['price']) {
            return true;
        }
        break;
    default:
        throw new Exception('other type [%s]', $order['type']);
    }
    return false;
}

function reserveOrder(PDOWrapper $dbh, JsonableOrder $order, int $price): int {
    $bank = null;
    try {
        $bank = Isubank($dbh);
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: isubank init failed', $throwable->getMessage()));
        throw $throwable;
    }

    $p = $order['amount'] * $price;
    if ($order['type'] === ORDER_TYPE_BUY) {
        $p *= -1;
    }

    $id = null;
    try {
        $id = $bank->reserve($order['user']['bank_id'], $p);
    } catch(IsubankCreditInsufficientException $e) {
        try {
            cancelOrder($dbh, $order, 'reserve_failed');
        } catch(\Throwable $throwable) {
            error_log(sprintf('%s: cancelOrder failed', $throwable->getMessage()));
            throw $throwable;
        }
        sendLog($dbh, $order['type'].'.error', [
            'error' => (string)$e->getMessage(),
            'user_id' => (int)$order['user_id'],
            'amount' => (int)$order['amount'],
            'price' => (int)$price,
        ]);
        throw $e;
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: isubank.Reserve', $throwable->getMessage()));
        throw $throwable;
    }

    return $id;
}

function commitReservedOrder(PDOWrapperTxn $tx, JsonableOrder $order, array $targets, array $reserves): void {
    try {
        $tx->execute('INSERT INTO trade (amount, price, created_at) VALUES (?, ?, NOW(6))', $order['amount'], $order['price']);
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: insert trade', $throwable->getMessage()));
        throw $throwable;
    }

    $trade_id = $tx->last_insert_id();

    sendLog($tx, 'trade', [
        'trade_id' => (int)$trade_id,
        'price' => (int)$order['price'],
        'amount' => (int)$order['amount'],
    ]);

    $targets[] = $order;

    foreach ($targets as $o) {
        try {
            $tx->execute('UPDATE orders SET trade_id = ?, closed_at = NOW(6) WHERE id = ?', $trade_id, $o['id']);
        } catch(\Throwable $throwable) {
            error_log(sprintf('%s: update order for trade', $throwable->getMessage()));
            throw $throwable;
        }
        sendLog($tx, $o['type'].'.trade', [
            'order_id' => (int)$o['id'],
            'price' => (int)$order['price'],
            'amount' => (int)$o['amount'],
            'user_id' => (int)$o['user_id'],
            'trade_id' => (int)$trade_id,
        ]);
    }

    $bank = null;
    try {
        $bank = Isubank($tx);
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: isubank init failed', $throwable->getMessage()));
        throw $throwable;
    }

    try {
        $bank->commit($reserves);
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: commit', $throwable->getMessage()));
        throw $throwable;
    }
}

function tryTrade(PDOWrapperTxn $tx, int $order_id): void {
    $order = null;
    try {
        $order = getOpenOrderByID($tx, $order_id);
    } catch(\Throwable $throwable) {
        throw $throwable;
    }

    $rest_amount = $order['amount'];
    $unit_price = $order['price'];
    $reserves = [0];
    $targets = [];

    try {
        $reserves[0] = reserveOrder($tx, $order, $unit_price);

        $target_orders = [];
        try {
            switch ($order['type']) {
            case ORDER_TYPE_BUY:
                $target_orders = queryOrders($tx, 'SELECT * FROM orders WHERE type = ? AND closed_at IS NULL AND price <= ? ORDER BY price ASC, created_at ASC, id ASC', ORDER_TYPE_SELL, $order['price']);
                break;
            case ORDER_TYPE_SELL:
                $target_orders = queryOrders($tx, 'SELECT * FROM orders WHERE type = ? AND closed_at IS NULL AND price >= ? ORDER BY price DESC, created_at ASC, id ASC', ORDER_TYPE_BUY, $order['price']);
                break;
            }
        } catch(\Throwable $throwable) {
            error_log(sprintf('%s: find target orders', $throwable->getMessage()));
            throw $throwable;
        }
        if (count($target_orders) === 0) {
            throw new NoOrderForTradeException();
        }

        foreach ($target_orders as $to) {
            try {
                $to = getOpenOrderByID($tx, $to['id']);
            } catch(OrderAlreadyClosedException $e) {
                continue;
            } catch(\Throwable $throwable) {
                error_log(sprintf('%s: getOpenOrderByID buy_order', $throwable->getMessage()));
                throw $throwable;
            }

            if ($to['amount'] > $rest_amount) {
                continue;
            }

            $rid = null;
            try {
                $rid = reserveOrder($tx, $to, $unit_price);
            } catch(IsubankCreditInsufficientException $e) {
                continue;
            } catch(\Throwable $throwable) {
                throw $throwable;
            }
            $reserves[] = $rid;
            $targets[] = $to;
            $rest_amount -= $to['amount'];
            if ($rest_amount === 0) {
                break;
            }
        }

        if (0 < $rest_amount) {
            throw new NoOrderForTradeException();
        }

        try {
            commitReservedOrder($tx, $order, $targets, $reserves);
        } catch(\Throwable $throwable) {
            throw $throwable;
        }
        $reserves = [];
    } finally {
        if (0 < count($reserves)) {
            $bank = null;
            try {
                $bank = Isubank($tx);
            } catch(\Throwable $throwable) {
                error_log(sprintf('[WARN] isubank init failed. err:%s', $throwable->getMessage()));
            }
            if ($bank !== null) {
                try {
                    $bank->cancel($reserves);
                } catch(\Throwable $throwable) {
                    error_log(sprintf('[WARN] isubank cancel failed. err:%s', $throwable->getMessage()));
                }
            }
        }
    }
}

function RunTrade(PDOWrapper $dbh): void {
    $lowest_sell_order = null;
    try {
        $lowest_sell_order = GetLowestSellOrder($dbh);
    } catch(Isucon8\NoRowsException $e) {
        // 売り注文が無いため成立しない
        return;
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: GetLowestSellOrder failed', $throwable->getMessage()));
        throw $throwable;
    }

    $highest_buy_order = null;
    try {
        $highest_buy_order = GetHighestBuyOrder($dbh);
    } catch(Isucon8\NoRowsException $e) {
        // 買い注文が無いため成立しない
        return;
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: GetHighestBuyOrder failed', $throwable->getMessage()));
        throw $throwable;
    }

    if ( $lowest_sell_order['price'] > $highest_buy_order['price']) {
        // 最安の売値が最高の買値よりも高いため成立しない
        return;
    }

    $candidates = null;
    if ($lowest_sell_order['amount'] > $highest_buy_order['amount']) {
        $candidates = [$lowest_sell_order['id'], $highest_buy_order['id']];
    } else {
        $candidates = [$highest_buy_order['id'], $lowest_sell_order['id']];
    }

    foreach ($candidates as $order_id) {
        try {
            $tx = null;
            try {
                $tx = $dbh->beginTxn();
            } catch(\Throwable $throwable) {
                error_log(sprintf('%s: begin transaction failed', $throwable->getMessage()));
                throw $throwable;
            }

            try {
                tryTrade($tx, $order_id);
                $tx->commit();
            } catch(NoOrderForTradeException | OrderAlreadyClosedException | IsubankCreditInsufficientException $e) {
                $tx->commit();
                throw $e;
            } catch(\Throwable $throwable) {
                $tx->rollback();
                throw $throwable;
            }
        } catch(NoOrderForTradeException | OrderAlreadyClosedException $e) {
            // 注文個数の多い方で成立しなかったので少ない方で試す
            continue;
        } catch(\Throwable $throwable) {
            throw $throwable;
        }
        // トレード成立したため次の取引を行う
        RunTrade($dbh);
    }
    // 個数のが不足していて不成立
    return;
}
