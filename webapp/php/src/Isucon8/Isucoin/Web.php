<?php

use Slim\Http\Request;
use Slim\Http\Response;
use Isucon8\PDOWrapper;

date_default_timezone_set('Asia/Tokyo');

$app = new \Slim\App();

$container = $app->getContainer();

$container['dbh'] = function (): PDOWrapper {
    $database = getenv('ISU_DB_NAME');
    $host = getenv('ISU_DB_HOST');
    $port = getenv('ISU_DB_PORT');
    $user = getenv('ISU_DB_USER');
    $password = getenv('ISU_DB_PASSWORD');
    $dsn = "mysql:host={$host};port={$port};dbname={$database};charset=utf8mb4;";
    return new PDOWrapper(new PDO(
        $dsn,
        $user,
        $password,
        [
            PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
            PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
        ]
    ));
};

# ISUCON用初期データの基準時間です
# この時間以降のデータはinitializeで削除されます
$container['base_time'] = new DateTime('2018-10-16 10:00:00');

$app->post('/initialize', function (Request $request, Response $response): Response {
    $tx = $this->dbh->beginTxn();
    try {
        InitBenchmark($tx);
        $postParams = $request->getParsedBody();
        foreach (
            [
                BANK_ENDPOINT,
                BANK_APPID,
                LOG_ENDPOINT,
                LOG_APPID,
            ] as $k
        ) {
            SetSetting($tx, $k, $postParams[$k]);
        }
        $tx->commit();
    } catch(\Throwable $throwable) {
        $tx->rollback();
        return resError($response, $throwable->getMessage(), 500);
    };
    return resSuccess($response, []);
});

$app->post('/signup', function (Request $request, Response $response): Response {
    $name = $request->getParsedBodyParam('name');
    $bank_id = $request->getParsedBodyParam('bank_id');
    $password = $request->getParsedBodyParam('password');
    if (empty($name) || empty($bank_id) || empty($password)) {
        return resError($response, 'all parameters are required', 400);
    }
    $tx = $this->dbh->beginTxn();
    try {
        UserSignup($tx, $name, $bank_id, $password);
        $tx->commit();
    } catch(BankUserNotFoundException $e) {
        // TODO: 失敗が多いときに403を返すBanの仕様に対応
        return resError($response, $e->getMessage(), 404);
    } catch(BankUserConflictException $e) {
        return resError($response, $e->getMessage(), 409);
    } catch(\Throwable $throwable) {
        $tx->rollback();
        return resError($response, $throwable->getMessage(), 500);
    };
    return resSuccess($response, []);
});

$app->post('/signin', function (Request $request, Response $response): Response {
    $bank_id = $request->getParsedBodyParam('bank_id');
    $password = $request->getParsedBodyParam('password');
    if (empty($bank_id) || empty($password)) {
        return resError($response, 'all parameters are required', 400);
    }
    $user = null;
    try {
        $user = UserLogin($this->dbh, $bank_id, $password);
    } catch(UserNotFoundException $e) {
        // TODO: 失敗が多いときに403を返すBanの仕様に対応
        return resError($response, $e->getMessage(), 404);
    } catch(\Throwable $throwable) {
        return resError($response, $throwable->getMessage(), 500);
    };
    $session = $this->session;
    $session->set('user_id', (int)$user['id']);
    return resSuccess($response, [
        'id' => (int)$user['id'],
        'name' => (string)$user['name'],
    ]);
});

$app->post('/signout', function (Request $request, Response $response): Response {
    $session = $this->session;
    $session->delete('user_id');
    return resSuccess($response, []);
});

$app->get('/info', function (Request $request, Response $response): Response {
    $last_trade_id = 0;
    $lt = new DateTime('@0');
    $res = [];

    $cursor = $request->getQueryParam('cursor');
    if (!empty($cursor)) {
        $last_trade_id = (int)$cursor;
        if (0 < $last_trade_id) {
            $trade = null;
            try {
                $trade = GetTradeByID($this->dbh, $last_trade_id);
            } catch(Isucon8\NoRowsException $e) {
            } catch(\Throwable $throwable) {
                return resError($response, $throwable->getMessage().': getTradeByID failed', 500);
            }
            if ($trade !== null) {
                $lt = new DateTime($trade['created_at']);
            }
        }
    }

    $latest_trade = null;
    try {
        $latest_trade = GetLatestTrade($this->dbh);
    } catch(\Throwable $throwable) {
        return resError($response, $throwable->getMessage().': GetLatestTrade failed', 500);
    }
    $res['cursor'] = (int)$latest_trade['id'];

    $user = $request->getAttribute('user');
    if ($user !== null) {
        $orders = [];
        try {
            $orders = GetOrdersByUserIDAndLastTradeId($this->dbh, $user['id'], $last_trade_id);
        } catch(\Throwable $throwable) {
            return resError($response, $throwable->getMessage(), 500);
        }

        foreach ($orders as $order) {
            try {
                FetchOrderRelation($this->dbh, $order);
            } catch(\Throwable $throwable) {
                return resError($response, $throwable->getMessage(), 500);
            }
        }

        $res['traded_orders'] = $orders;
    }

    $by_sec_time = (clone $this->base_time)->modify('-300 seconds');
    if ($by_sec_time < $lt) {
        $by_sec_time = clone $lt;
    }
    try {
        $res['chart_by_sec'] = GetCandlestickData($this->dbh, $by_sec_time, '%Y-%m-%d %H:%i:%s');
    } catch(\Throwable $throwable) {
        return resError($response, $throwable->getMessage().': GetCandlestickData by sec', 500);
    }

    $by_min_time = (clone $this->base_time)->modify('-300 minutes');
    if ($by_min_time < $lt) {
        $by_min_time = new DateTime($lt->format('Y-m-d H:i:00'));
    }
    try {
        $res['chart_by_min'] = GetCandlestickData($this->dbh, $by_min_time, '%Y-%m-%d %H:%i:00');
    } catch(\Throwable $throwable) {
        return resError($response, $throwable->getMessage().': GetCandlestickData by min', 500);
    }

    $by_hour_time = (clone $this->base_time)->modify('-48 hours');
    if ($by_hour_time < $lt) {
        $by_hour_time = new DateTime($lt->format('Y-m-d H:00:00'));
    }
    try {
        $res['chart_by_hour'] = GetCandlestickData($this->dbh, $by_hour_time, '%Y-%m-%d %H:00:00');
    } catch(\Throwable $throwable) {
        return resError($response, $throwable->getMessage().': GetCandlestickData by hour', 500);
    }

    $lowest_sell_order = null;
    try {
        $lowest_sell_order = GetLowestSellOrder($this->dbh);
    } catch(Isucon8\NoRowsException $e) {
    } catch(\Throwable $throwable) {
        return resError($response, $throwable->getMessage().': GetLowestSellOrder', 500);
    }
    if ($lowest_sell_order !== null) {
        $res['lowest_sell_price'] = (int)$lowest_sell_order['price'];
    }

    $highest_sell_order = null;
    try {
        $highest_sell_order = GetHighestBuyOrder($this->dbh);
    } catch(Isucon8\NoRowsException $e) {
    } catch(\Throwable $throwable) {
        return resError($response, $throwable->getMessage().': GetHighestBuyOrder', 500);
    }
    if ($highest_sell_order !== null) {
        $res['highest_buy_price'] = (int)$highest_sell_order['price'];
    }

    // TODO: trueにするとシェアボタンが有効になるが、アクセスが増えてヤバイので一旦falseにしておく
    $res["enable_share"] = false;

    return resSuccess($response, $res);
});

$app->post('/orders', function (Request $request, Response $response): Response {
    $user = $request->getAttribute('user');
    if ($user === null) {
        return resError($response, 'Not authenticated', 401);
    }
    $amount = $request->getParsedBodyParam('amount');
    $price = $request->getParsedBodyParam('price');

    $tx = $this->dbh->beginTxn();
    $order = null;
    try {
        $order = AddOrder($tx, $request->getParsedBodyParam('type'), $user['id'], $amount, $price);
        $tx->commit();
    } catch(ParameterInvalidException | CreditInsufficientException $e) {
        $tx->rollback();
        return resError($response, $e->getMessage(), 400);
    } catch(\Throwable $throwable) {
        $tx->rollback();
        return resError($response, $throwable->getMessage(), 500);
    }

    $trade_chance = false;
    try {
        $trade_chance = HasTradeChanceByOrder($this->dbh, $order['id']);
    } catch(\Throwable $throwable) {
        return resError($response, $throwable->getMessage(), 500);
    }

    if ($trade_chance) {
        try {
            RunTrade($this->dbh);
        } catch(\Throwable $throwable) {
            error_log(sprintf('runTrade err:%s', $throwable->getMessage()));
            // トレードに失敗してもエラーにはしない
            //throw $throwable;
        }
    }

    return resSuccess($response, ['id' => (int)$order['id']]);
});

$app->get('/orders', function (Request $request, Response $response): Response {
    $user = $request->getAttribute('user');
    if ($user === null) {
        return resError($response, 'Not authenticated', 401);
    }

    $orders = null;
    try {
        $orders = GetOrdersByUserID($this->dbh, $user['id']);
    } catch(\Throwable $throwable) {
        return resError($response, $throwable->getMessage(), 500);
    }

    foreach ($orders as $order) {
        try {
            FetchOrderRelation($this->dbh, $order);
        } catch(\Throwable $throwable) {
            return resError($response, $throwable->getMessage(), 500);
        }
    }

    return resSuccess($response, $orders);
});

$app->delete('/order/{id}', function (Request $request, Response $response, array $args): Response {
    $user = $request->getAttribute('user');
    if ($user === null) {
        return resError($response, 'Not authenticated', 401);
    }
    $id = (int)$args['id'];
    $tx = $this->dbh->beginTxn();
    try {
        DeleteOrder($tx, $user['id'], $id, 'canceled');
        $tx->commit();
    } catch(OrderNotFoundException | OrderAlreadyClosedException $e) {
        $tx->rollback();
        return resError($response, $e->getMessage(), 404);
    } catch(\Throwable $throwable) {
        $tx->rollback();
        return resError($response, $throwable->getMessage(), 500);
    }
    return resSuccess($response, ['id' => $id]);
});

$app->add(function (Request $request, Response $response, callable $next): Response {
    if (!$request->isGet()) {
        return $next($request, $response);
    }

    $path = $request->getUri()->getPath();
    if ($path === '/') {
        $path = '/index.html';
    }
    $filePath = getenv('ISU_PUBLIC_DIR').$path;

    $ext_to_mime = [
        'js' => 'application/javascript',
        'css' => 'text/css',
        'html' => 'text/html; character=utf-8',
        'ico' => 'image/vnd.microsoft.icon',
    ];

    if (file_exists($filePath)) {
        $content = file_get_contents($filePath);
        $body = $response->getBody();
        $body->write($content);
        $ext = pathinfo($filePath, PATHINFO_EXTENSION);
        $mime_type = $ext_to_mime[$ext] ?? 'plain/text';
        return $response->withBody($body)->withHeader('Content-Type', $mime_type);
    }

    return $next($request, $response);
});

$app->add(function (Request $request, Response $response, callable $next): Response {
    $session = $this->session;
    if ($session->exists('user_id')) {
        $user = null;
        try {
            $user = GetUserByID($this->dbh, (int)$session->get('user_id'));
        } catch(Isucon8\NoRowsException $e) {
            $session->delete('user_id');
            return resError($response, 'セッションが切断されました', 404);
        } catch (\Throwable $throwable) {
            return resError($response, $throwable, 500);
        }
        $request = $request->withAttribute('user', $user);
    }
    return $next($request, $response);
});

$app->add(new \Adbar\SessionMiddleware([
    'name' => 'isucon_session',
    'autorefresh' => true,
    'lifetime' => '1 hour',
    'encryption_key' => 'tonymoris',
    'namespace' => 'isucon',
]));

$container['session'] = function (): \SlimSession\Helper {
    return new \SlimSession\Helper();
};

$app->run();

function resSuccess(Response $response, array $data = []): Response {
    return $response
        ->withStatus(200)
        ->withHeader('Content-Type', 'application/json; charset=utf-8')
        ->withJson($data);
}

function resError(Response $response, string $error = 'unknown', int $status = 500): Response {
    error_log(sprintf('[WARN] err:%s', $error));
    return $response
        ->withStatus($status)
        ->withHeader('Content-Type', 'application/json; charset=utf-8')
        ->withHeader('X-Content-Type-Options', 'nosniff')
        ->withJson(['code' => $status, 'err' => $error]);
}
