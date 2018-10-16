<?php

use Isucon8\PDOWrapperTxn;

class BankUserNotFoundException extends Exception {
    public function __construct($message = 'bank user not found', $code = 0, Exception $previous = null) {
        parent::__construct($message, $code, $previous);
    }
}

class BankUserConflictException extends Exception {
    public function __construct($message = 'bank user conflict', $code = 0, Exception $previous = null) {
        parent::__construct($message, $code, $previous);
    }
}

class UserNotFoundException extends Exception {
    public function __construct($message = 'user not found', $code = 0, Exception $previous = null) {
        parent::__construct($message, $code, $previous);
    }
}

class OrderNotFoundException extends Exception {
    public function __construct($message = 'order not found', $code = 0, Exception $previous = null) {
        parent::__construct($message, $code, $previous);
    }
}

class OrderAlreadyClosedException extends Exception {
    public function __construct($message = 'order is already closed', $code = 0, Exception $previous = null) {
        parent::__construct($message, $code, $previous);
    }
}

class CreditInsufficientException extends Exception {
    public function __construct($message = '銀行の残高が足りません', $code = 0, Exception $previous = null) {
        parent::__construct($message, $code, $previous);
    }
}

class ParameterInvalidException extends Exception {
    public function __construct($message = 'parameter invalid', $code = 0, Exception $previous = null) {
        parent::__construct($message, $code, $previous);
    }
}

class NoOrderForTradeException extends Exception {
    public function __construct($message = 'no order for trade', $code = 0, Exception $previous = null) {
        parent::__construct($message, $code, $previous);
    }
}

function InitBenchmark(PDOWrapperTxn $txn) {
    // 前回の10:00:00+0900までのデータを消す
    // 本戦当日は2018-10-20T10:00:00+0900 固定だが、他の時間帯にデータ量を揃える必要がある
    $stop = date('Y-m-d 10:00:00', strtotime('-10 hours'));
    foreach (
        [
            'DELETE FROM orders WHERE created_at >= ?',
            'DELETE FROM trade  WHERE created_at >= ?',
            'DELETE FROM user   WHERE created_at >= ?',
        ] as $query
    ) {
        $txn->execute($query, $stop);
    }
}
