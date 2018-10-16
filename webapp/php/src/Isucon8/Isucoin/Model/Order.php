<?php

use Isucon8\PDOWrapper;
use Isucon8\PDOWrapperTxn;

const ORDER_TYPE_BUY  = "buy";
const ORDER_TYPE_SELL = "sell";

class JsonableOrder extends ArrayObject implements JsonSerializable {
    public function jsonSerialize() {
        $data = [
            'id'         => (int)$this['id'],
            'type'       => (string)$this['type'],
            'user_id'    => (int)$this['user_id'],
            'amount'     => (int)$this['amount'],
            'price'      => (int)$this['price'],
            'trade_id'   => (int)$this['trade_id'] ?? (int)0,
            'created_at' => (string)(new DateTime($this['created_at']))->format('Y-m-d\TH:i:sP'),
        ];
        if ($this['closed_at'] !== null) {
            $data['closed_at'] = (string)(new DateTime($this['closed_at']))->format('Y-m-d\TH:i:sP');
        }
        if ($this['user'] !== null) {
            $data['user'] = $this['user'];
        }
        if ($this['trade'] !== null) {
            $data['trade'] = $this['trade'];
        }
        return $data;
    }
}

function reformOrder(array $order): JsonableOrder {
    return new JsonableOrder($order);
}

function queryOrders(PDOWrapper $dbh, string $query, ...$args): array {
    $rows = [];
    try {
        foreach ($dbh->select_all($query, ...$args) as $row) {
            $rows[] = reformOrder($row);
        }
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: Query failed. query:%s, args:%v', $throwable->getMessage(), $query, var_export($args, true)));
        throw $throwable;
    }
    return $rows;
}

function GetOrdersByUserID(PDOWrapper $dbh, int $user_id): array {
    $ret = [];
    $rows = $dbh->select_all('SELECT * FROM orders WHERE user_id = ? AND (closed_at IS NULL OR trade_id IS NOT NULL) ORDER BY created_at ASC', $user_id);
    foreach ($rows as $row) {
        $ret[] = reformOrder($row);
    }
    return $ret;
}

function GetOrdersByUserIDAndLastTradeId(PDOWrapper $dbh, int $user_id, int $trade_id): array {
    $ret = [];
    $rows = $dbh->select_all('SELECT * FROM orders WHERE user_id = ? AND trade_id IS NOT NULL AND trade_id > ? ORDER BY created_at ASC', $user_id, $trade_id);
    foreach ($rows as $row) {
        $ret[] = reformOrder($row);
    }
    return $ret;
}

function getOpenOrderByID(PDOWrapper $tx, int $id): JsonableOrder {
    $order = null;
    try {
        $order = getOrderByIDWithLock($tx, $id);
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: getOrderByIDWithLock sell_order failed. id:%d', $throwable->getMessage(), $id));
        throw $throwable;
    }

    if ($order['closed_at'] !== null) {
        throw new OrderAlreadyClosedException();
    }

    try {
        $order['user'] = getUserByIDWithLock($tx, $order['user_id']);
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: getUserByIDWithLock sell user failed. id:%d', $throwable->getMessage(), $order['user_id']));
        throw $throwable;
    }

    return $order;
}

function GetOrderByID(PDOWrapper $dbh, int $id): JsonableOrder {
    return reformOrder($dbh->select_row('SELECT * FROM orders WHERE id = ?', $id));
}

function getOrderByIDWithLock(PDOWrapperTxn $tx, int $id): JsonableOrder {
    return reformOrder($tx->select_row('SELECT * FROM orders WHERE id = ? FOR UPDATE', $id));
}

function GetLowestSellOrder(PDOWrapper $dbh): JsonableOrder {
    return reformOrder($dbh->select_row('SELECT * FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price ASC, created_at ASC LIMIT 1', ORDER_TYPE_SELL));
}

function GetHighestBuyOrder(PDOWrapper $dbh): JsonableOrder {
    return reformOrder($dbh->select_row('SELECT * FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price DESC, created_at ASC LIMIT 1', ORDER_TYPE_BUY));
}

function FetchOrderRelation(PDOWrapper $dbh, JsonableOrder $order): void {
    try {
        $order['user'] = GetUserByID($dbh, $order['user_id']);
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: GetUserByID failed. id:%d', $throwable->getMessage(), $order['user_id']));
        throw $throwable;
    }

    if (0 < $order['trade_id']) {
        try {
            $order['trade'] = GetTradeByID($dbh, $order['trade_id']);
        } catch(\Throwable $throwable) {
            error_log(sprintf('%s: GetTradeByID failed. id:%d', $throwable->getMessage(), $order['trade_id']));
            throw $throwable;
        }
    }
}

function AddOrder(PDOWrapperTxn $tx, string $ot, int $user_id, int $amount, int $price): JsonableOrder {
    if ($amount <= 0 || $price <= 0) {
        throw new ParameterInvalidException();
    }
    try {
        $user = getUserByIDWithLock($tx, $user_id);
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: getUserByIDWithLock failed. id:%d', $throwable->getMessage(), $user_id));
        throw $throwable;
    }

    $bank = null;
    try {
        $bank = Isubank($tx);
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: newIsubank failed', $throwable->getMessage()));
        throw $throwable;
    }

    switch ($ot) {
    case ORDER_TYPE_BUY:
        $total_price = $price * $amount;
        try {
            $bank->check($user['bank_id'], $total_price);
        } catch(Isucon8\Isubank\CreditInsufficientException $e) {
            throw new CreditInsufficientException();
        } catch(\Throwable $e) {
            error_log(sprintf('%s: isubank check failed', $e->getMessage()));
            throw $e;
        } finally {
            if ($e !== null) {
                sendLog($tx, "buy.error", [
                    'error' => (string)$e->getMessage(),
                    'user_id' => (int)$user['id'],
                    'amount' => (int)$amount,
                    'price' => (int)$price,
                ]);
             }
        }
        break;
    case ORDER_TYPE_SELL:
        // TODO 椅子の保有チェック
        break;
    default:
        throw new ParameterInvalidException();
    }

    try {
        $tx->execute('INSERT INTO orders (type, user_id, amount, price, created_at) VALUES (?, ?, ?, ?, NOW(6))', $ot, $user['id'], $amount, $price);
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: insert order failed', $throwable->getMessage()));
        throw $throwable;
    }

    $id = $tx->last_insert_id();
    sendLog($tx, $ot.".order", [
        'order_id' => (int)$id,
        'user_id' => (int)$user['id'],
        'amount' => (int)$amount,
        'price' => (int)$price,
    ]);

    return GetOrderByID($tx, $id);
}

function DeleteOrder(PDOWrapperTxn $tx, int $user_id, int $order_id, string $reason): void {
    $user = null;
    try {
        $user = getUserByIDWithLock($tx, $user_id);
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: getUserByIDWithLock failed. id:%d', $throwable->getMessage(), $user_id));
        throw $throwable;
    }

    $order = null;
    try {
        $order = getOrderByIDWithLock($tx, $order_id);
    } catch(\NoRowsException $e) {
        throw new OrderNotFoundException();
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: getOrderByIDWithLock failed. id:%d', $throwable->getMessage(), $order_id));
        throw $throwable;
    }
    if ($order['user_id'] !== $user['id']) {
        throw new OrderNotFoundException();
    }
    if (isset($order['closed_at'])) {
        throw new OrderAlreadyClosedException();
    }

    cancelOrder($tx, $order, $reason);
}

function cancelOrder(PDOWrapper $dbh, JsonableOrder $order, string $reason): void {
    try {
        $dbh->execute('UPDATE orders SET closed_at = NOW(6) WHERE id = ?', $order['id']);
    } catch(\Throwable $throwable) {
        error_log(sprintf('%s: update orders for cancel failed', $throwable->getMessage()));
        throw $throwable;
    }
    sendLog($dbh, $order['type'].'.delete', [
        'order_id' => (int)$order['id'],
        'user_id' => (int)$order['user_id'],
        'reason' => (string)$reason,
    ]);
}
