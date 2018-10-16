<?php

namespace Isucon8;

use PDO;
use Exception;

class NoRowsException extends Exception {
    public function __construct($message = 'sql: no rows', $code = 0, Exception $previous = null) {
        parent::__construct($message, $code, $previous);
    }
};

class PDOWrapperTxn extends PDOWrapper {
    public function __construct(PDO $pdo) {
        $this->pdo = $pdo;
        $this->pdo->beginTransaction();
    }
}

class PDOWrapper
{
    protected $pdo;

    public function __call($name, $arguments) {
        return call_user_func_array([$this->pdo, $name], $arguments);
    }

    public function __construct(PDO $pdo) {
        $this->pdo = $pdo;
        $this->pdo->query('SET SESSION sql_mode="STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION"');
    }

    public function select_one(string $query, ...$params) {
        $stmt = $this->pdo->prepare($query);
        $stmt->execute($params);
        $row = $stmt->fetch(PDO::FETCH_NUM);
        $stmt->closeCursor();
        return $row[0];
    }

    public function select_all(string $query, ...$params): array {
        $stmt = $this->pdo->prepare($query);
        $stmt->execute($params);
        return $stmt->fetchAll(PDO::FETCH_ASSOC);
    }

    public function select_row(string $query, ...$params): array {
        $stmt = $this->pdo->prepare($query);
        $stmt->execute($params);
        $row = $stmt->fetch(PDO::FETCH_ASSOC);
        $stmt->closeCursor();
        if ($row === false) {
            throw new NoRowsException();
        }
        return $row;
    }

    public function execute($query, ...$params): bool {
        $stmt = $this->pdo->prepare($query);
        return $stmt->execute($params);
    }

    public function last_insert_id() {
        return $this->pdo->lastInsertId();
    }

    public function beginTxn(): PDOWrapperTxn {
        return new PDOWrapperTxn($this->pdo);
    }
}
