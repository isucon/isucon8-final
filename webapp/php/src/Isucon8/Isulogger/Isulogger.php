<?php

namespace Isucon8\Isulogger;

use Exception;
use DateTime;
use GuzzleHttp\Client;

class Isulogger
{
    protected $endpoint;
    protected $app_id;

    function __construct(string $endpoint, string $app_id) {
        $this->endpoint = $endpoint;
        $this->app_id = $app_id;
    }

    public function send(string $tag, array $data): void {
        $this->request('/send', [
            'tag' => $tag,
            'time' => (new DateTime())->format('Y-m-d\TH:i:sP'),
            'data' => $data,
        ]);
    }

    protected function request(string $p, array $v): void {
        $client = new Client(['base_uri' => $this->endpoint]);
        $response = $client->request('POST', $p, [
            'headers' => ['Authorization' => 'Bearer '.$this->app_id],
            'json' => $v,
        ]);
        if ($response->getStatusCode() === 200) {
            return;
        }
        throw new Exception(sprintf('logger status is not ok. code: %d, body: %s', $response->getStatusCode(), $response->getBody()->getContents()));
    }
}
