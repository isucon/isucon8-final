<?php

use Isucon8\PDOWrapper;
use Isucon8\PDOWrapperTxn;
use Isucon8\Isubank\Isubank;
use Isucon8\Isulogger\Isulogger;

const BANK_ENDPOINT = 'bank_endpoint';
const BANK_APPID    = 'bank_appid';
const LOG_ENDPOINT  = 'log_endpoint';
const LOG_APPID     = 'log_appid';

function SetSetting(PDOWrapperTxn $txn, string $k, string $v): void {
    $txn->execute('INSERT INTO setting (name, val) VALUES (?, ?) ON DUPLICATE KEY UPDATE val = VALUES(val)', $k, $v);
}

function GetSetting(PDOWrapper $dbh, string $k): string {
    return $dbh->select_one('SELECT val FROM setting WHERE name = ?', $k);
}

function Isubank(PDOWrapper $dbh): Isubank {
    $ep = GetSetting($dbh, BANK_ENDPOINT);
    $id = GetSetting($dbh, BANK_APPID);
    return new Isubank($ep, $id);
}

function Logger(PDOWrapper $dbh): Isulogger {
    $ep = GetSetting($dbh, LOG_ENDPOINT);
    $id = GetSetting($dbh, LOG_APPID);
    return new Isulogger($ep, $id);
}

function sendLog(PDOWrapper $dbh, string $tag, array $v): void {
    $logger = null;
    try {
        $logger = Logger($dbh);
    } catch(\Throwable $throwable) {
        error_log(sprintf('[WARN] new logger failed. tag: %s, v: %s, err:%s', $tag, serialize($v), $throwable));
    }
    if ($logger === null) {
        return;
    }
    try {
        $logger->send($tag, $v);
    } catch(\Throwable $throwable) {
        error_log(sprintf('[WARN] logger send failed. tag: %s, v: %s, err:%s', $tag, serialize($v), $throwable));
    }
}
