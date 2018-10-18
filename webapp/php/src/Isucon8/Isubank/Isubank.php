<?php

namespace Isucon8\Isubank;

use Exception;
use GuzzleHttp\Client;

// いすこん銀行にアカウントが存在しない
class NoUserException extends Exception {
    public function __construct($message = 'no bank user', $code = 0, Exception $previous = null) {
        parent::__construct($message, $code, $previous);
    }
}

// 仮決済時または残高チェック時に残高が不足している
class CreditInsufficientException extends Exception {
    public function __construct($message = 'credit is insufficient', $code = 0, Exception $previous = null) {
        parent::__construct($message, $code, $previous);
    }
}


// Isubank はISUBANK APIクライアントです
// new Isubankによって初期化してください
class Isubank
{
    protected $endpoint;
    protected $app_id;

    // endpoint: ISUBANK APIを利用するためのエンドポイントURI
    // appID:    ISUBANK APIを利用するためのアプリケーションID
    function __construct(string $endpoint, string $app_id) {
        $this->endpoint = $endpoint;
        $this->app_id = $app_id;
    }

    // check は残高確認です
    // reserve による予約済み残高は含まれません
    public function check(string $bank_id, int $price): void {
        $res = $this->request('/check', ['bank_id' => $bank_id, 'price' => $price]);

        if ($res['success']) {
            return;
        }
        if ($res['error'] === 'bank_id not found') {
            throw new NoUserException();
        }
        if ($res['error'] === 'credit is insufficient') {
            throw new CreditInsufficientException();
        }
        throw new Exception(sprintf('check failed. err: %s', $res['error']));
    }

    // reserve は仮決済(残高の確保)を行います
    public function reserve(string $bank_id, int $price): int {
        $res = $this->request('/reserve', ['bank_id' => $bank_id, 'price' => $price]);

        if (!$res['success']) {
            if ($res['error'] === 'credit is insufficient') {
                throw new CreditInsufficientException();
            }
            throw new Exception(sprintf('reserve failed. err: %s', $res['error']));
        }

        return $res['reserve_id'];
    }

    // commit は決済の確定を行います
    // 正常に仮決済処理を行っていればここでエラーになることはありません
    public function commit(array $reserve_ids): void {
        $res = $this->request('/commit', ['reserve_ids' => $reserve_ids]);

        if (!$res['success']) {
            if ($res['error'] === 'credit is insufficient') {
                throw new CreditInsufficientException();
            }
            throw new Exception(sprintf('commit failed. err: %s', $res['error']));
        }
    }

    // cancel は決済の取り消しを行います
    public function cancel(array $reserve_ids): void {
        $res = $this->request('/cancel', ['reserve_ids' => $reserve_ids]);

        if (!$res['success']) {
            throw new Exception(sprintf('cancel failed. err: %s', $res['error']));
        }
    }

    protected function request(string $p, array $v): array {
        $client = new Client(['base_uri' => $this->endpoint]);
        $response = $client->request('POST', $p, [
            'headers' => ['Authorization' => 'Bearer '.$this->app_id],
            'http_errors' => false,
            'json' => $v,
        ]);
        $data = json_decode($response->getBody(), true);
        $data['success'] = $response->getStatusCode() === 200;
        return $data;
    }
}
