<?php

use Isucon8\PDOWrapper;
use Isucon8\PDOWrapperTxn;
use Isucon8\Isubank\Isubank;

class JsonableUser extends ArrayObject implements JsonSerializable {
    public function jsonSerialize() {
        return [
            'id'   => (int)$this['id'],
            'name' => (string)$this['name'],
        ];
    }
}

function reformUser($user): JsonableUser {
    return new JsonableUser($user);
}

function GetUserByID(PDOWrapper $dbh, int $id): JsonableUser {
    return reformUser($dbh->select_row('SELECT * FROM user WHERE id = ?', $id));
}

function getUserByIDWithLock(PDOWrapperTxn $tx, int $id): JsonableUser {
    return reformUser($tx->select_row('SELECT * FROM user WHERE id = ? FOR UPDATE', $id));
}

function UserSignup(PDOWrapperTxn $tx, string $name, string $bank_id, string $password): void {
    $bank = Isubank($tx);

    // bankIDの検証
    try {
        $bank->check($bank_id, 0);
    } catch(\Throwable $throwable) {
        throw new BankUserNotFoundException();
    }

    $pass = password_hash($password, PASSWORD_BCRYPT);

    try {
        $tx->execute('INSERT INTO user (bank_id, name, password, created_at) VALUES (?, ?, ?, NOW(6))', $bank_id, $name, $pass);
    } catch(PDOException $e) {
        if ($e->errorInfo[1] === 1062) {
            throw new BankUserConflictException();
        }
        throw $e;
    }

    $user_id = $tx->last_insert_id();

    sendLog($tx, 'signup', [
        'bank_id' => (string)$bank_id,
        'user_id' => (int)$user_id,
        'name' => (string)$name,
    ]);
}

function UserLogin(PDOWrapper $dbh, string $bank_id, string $password): JsonableUser {
    $user = null;
    try {
        $user = reformUser($dbh->select_row('SELECT * FROM user WHERE bank_id = ?', $bank_id));
    } catch(Isucon8\NoRowsException $e) {
        throw new UserNotFoundException();
    } catch(\Throwable $throwable) {
        throw $throwable;
    }

    if (!password_verify($password, $user['password'])) {
        throw new UserNotFoundException();
    }

    sendLog($dbh, 'signin', ['user_id' => (int)$user['id']]);

    return $user;
}
