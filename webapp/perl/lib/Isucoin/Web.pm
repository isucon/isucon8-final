package Isucoin::Web;

use 5.28.0;
use warnings;
use utf8;

use DBIx::Sunny;
use Kossy;
use Plack::Session;
use Time::Moment;
use JSON::Types;
use Path::Tiny;
use Try::Tiny;
use Log::Minimal;

use Isucoin::Model;
use Isucoin::Exception;
use Isubank;

# ISUCON用初期データの基準時間です
# この時間以降のデータはinitializeで削除されます
my $base_time = Time::Moment->from_string("2018-10-16T10:00:00Z");

get "/" => sub {
    my ( $self, $c )  = @_;

    my $content = path($self->root_dir . $ENV{ISU_PUBLIC_DIR} . "/index.html");

    return [
        200,
        ["Content-Type" => "text/html"],
        [$content->slurp],
    ];
};

post "/initialize" => sub {
    my ( $self, $c )  = @_;

    my $dbh = $self->dbh;
    my $model = Isucoin::Model->new(dbh => $dbh);
    {
        my $txn = $dbh->txn_scope;

        $model->init_benchmark;
        my %args = map {
            $_ => ($c->req->parameters->{$_} // "")
        } @{$model->endpoint_names};
        for my $k (keys %args) {
            $model->set_setting($k => $args{$k});
        }

        $txn->commit;
    };

    return $c->render_json({});
};

post "/signup" => sub {
    my ( $self, $c )  = @_;

    my ($name, $bank_id, $password) =
        $c->req->parameters->@{qw/name bank_id password/};
    if (!$name || !$bank_id || !$password) {
        return $c->halt(400, "all parameters are required");
    }

    my $dbh = $self->dbh;
    my $model = Isucoin::Model->new(dbh => $dbh);
    {
        my $txn = $dbh->txn_scope;

        try {
            $model->user_signup(
                name     => $name,
                bank_id  => $bank_id,
                password => $password,
            );
        } catch {
            my $err = $_;
            # TODO: 失敗が多いときに403を返すBanの仕様に対応
            if (Isubank::Exception::NoUser->caught($err)) {
                $txn->rollback;
                return $c->halt(404, "bank user not found");
            }
            if (Isucoin::Exception::BankUserConflict->caught($err)) {
                $txn->rollback;
                return $c->halt(409, "bank user conflict");
            }
            return $c->halt(500, $err);
        };

        $txn->commit;
    };

    return $c->render_json({});
};

post "/signin" => sub {
    my ( $self, $c )  = @_;

    my ($bank_id, $password) =
        $c->req->parameters->@{qw/bank_id password/};
    if (!$bank_id || !$password) {
        return $c->halt(400, "all parameters are required");
    }

    my $model = Isucoin::Model->new(dbh => $self->dbh);
    my $user;
    try {
        $user = $model->user_login(bank_id => $bank_id, password => $password);
    } catch {
        my $err = $_;
        # TODO: 失敗が多いときに403を返すBanの仕様に対応
        if (Isucoin::Exception::UserNotFound->caught($err)) {
            return $c->halt(404, "user not found");
        }
        return $c->halt(500, $err);
    };


    my $session = Plack::Session->new($c->env);
    $session->set(user_id => $user->{id});

    return $c->render_json($user);
};

filter login_required => sub {
    my $app = shift;

    return sub {
        my ($self, $c) = @_;

        my $user = $self->user_by_request($c);
        return $self->halt(401, "login_required") unless $user;
        $c->stash->{user} = $user;

        return $app->($self, $c);
    };
};

post "/signout" => sub {
    my ( $self, $c )  = @_;

    my $session = Plack::Session->new($c->env);
    $session->remove("user_id");

    return $c->render_json({});
};

get "/info" => sub {
    my ( $self, $c )  = @_;

    my $model = Isucoin::Model->new(dbh => $self->dbh);

    my $last_trade_id = 0;
    my $lt = Time::Moment->from_epoch(0);
    my %res;
    my $cursor = $c->req->parameters->{cursor};
    if ($last_trade_id = $cursor) {
        my $trade = $model->get_trade_by_id($last_trade_id);
        if ($trade) {
            $lt = Time::Moment->from_string($trade->{created_at}, lenient => 1);
        }
    }

    my $latest_trade = $model->get_latest_trade;
    $res{cursor} = $latest_trade->{id};

    my $user = $self->user_by_request($c);
    if ($user) {
        my $orders = $model->get_orders_by_user_id_and_last_trade_id(
            $user->{id}, $last_trade_id,
        );
        for my $order (@$orders) {
            $model->fetch_order_relation($order);
        }
        $res{traded_orders} = $orders;
    }

    my $by_sec_time = $base_time->minus_seconds(300);
    if ($lt->is_after($by_sec_time)) {
        $by_sec_time = $lt->with_precision(0);
    }
    $res{chart_by_sec} = $model->get_candletick_data(
        mt => $by_sec_time,
        tf => "%Y-%m-%d %H:%i:%s",
    );

    my $by_min_time = $base_time->minus_minutes(300);
    if ($lt->is_after($by_min_time)) {
        $by_min_time = $lt->with_precision(-1);
    }
    $res{chart_by_min} = $model->get_candletick_data(
        mt => $by_min_time,
        tf => "%Y-%m-%d %H:%i:00",
    );

    my $by_hour_time = $base_time->minus_hours(48);
    if ($lt->is_after($by_hour_time)) {
        $by_hour_time = $lt->with_precision(-2);
    }
    $res{chart_by_hour} = $model->get_candletick_data(
        mt => $by_hour_time,
        tf => "%Y-%m-%d %H:00:00",
    );

    my $lowest_sell_order = $model->get_lowest_sell_order;
    $res{lowest_sell_price} = $lowest_sell_order->{price};

    my $highest_buy_order = $model->get_highest_buy_order;
    $res{highest_buy_price} = $highest_buy_order->{price};

    # TODO: trueにするとシェアボタンが有効になるが、アクセスが増えてヤバイので一旦falseにしておく
    $res{enable_share} = bool 0;

    return $c->render_json(\%res);
};

post "/orders" => [qw/login_required/] => sub {
    my ( $self, $c )  = @_;

    my $user = $c->stash->{user};
    my ($amount, $price, $type) = $c->req->parameters->@{qw/amount price type/};

    my $dbh = $self->dbh;
    my $model = Isucoin::Model->new(dbh => $dbh);
    my $order;
    {
        my $txn = $dbh->txn_scope;

        try {
            $order = $model->add_order(
                type    => $type,
                user_id => $user->{id},
                amount  => $amount,
                price   => $price,
            );
        } catch {
            my $err = $_;
            if (
                Isucoin::Exception::ParameterInvalid->caught($err) ||
                Isucoin::Exception::CreditInsufficiant->caught($err)
            ) {
                $txn->rollback;
                return $c->halt(400);
            }
            die $err;
        };

        $txn->commit;
    };

    my $trade_chance = $model->has_trade_chance_by_order($order->{id});
    if ($trade_chance) {
        try {
            $model->run_trade;
        } catch {
            # トレードに失敗してもエラーにはしない
            warnf "run_trade err: %s", $_;
        };
    }

    return $c->render_json({ id => $order->{id} });
};

get "/orders" => [qw/login_required/] => sub {
    my ( $self, $c )  = @_;

    my $user = $c->stash->{user};

    my $model = Isucoin::Model->new(dbh => $self->dbh);
    my $orders = $model->get_orders_by_user_id($user->{id});

    for my $order (@$orders) {
        $model->fetch_order_relation($order);
    }

    return $c->render_json($orders);
};

router ["DELETE"] => "/order/{id}" => [qw/login_required/] => sub {
    my ( $self, $c )  = @_;

    my $user = $c->stash->{user};
    my $order_id = $c->args->{id};

    my $dbh = $self->dbh;
    my $model = Isucoin::Model->new(dbh => $dbh);
    {
        my $txn = $dbh->txn_scope;

        try {
            $model->delete_order(
                user_id  => $user->{id},
                order_id => $order_id,
                reason   => "canceled",
            );
        } catch {
            my $err = $_;
            if (
                Isucoin::Exception::OrderNotFound->caught($err) ||
                Isucoin::Exception::OrderAlreadyClosed->caught($err)
            ) {
                $txn->rollback;
                return $c->halt(404);
            }

        };

        $txn->commit;
    };

    return $c->render_json({ id => number $order_id });
};

sub dbh {
    my $self = shift;
    $self->{_dbh} ||= do {
        my $dsn = "dbi:mysql:database=$ENV{ISU_DB_NAME};host=$ENV{ISU_DB_HOST};port=$ENV{ISU_DB_PORT}";
        DBIx::Sunny->connect($dsn, $ENV{ISU_DB_USER}, $ENV{ISU_DB_PASSWORD}, {
            mysql_enable_utf8mb4 => 1,
            mysql_auto_reconnect => 1,
            # TODO: replace mysqld's sql_mode setting and remove following codes
            Callbacks => {
                connected => sub {
                    my $dbh = shift;
                    $dbh->do('SET SESSION sql_mode="STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION"');
                    return;
                },
            },
        });
    };
}

sub user_by_request {
    my ($self, $c) = @_;

    my $session = Plack::Session->new($c->env);
    my $user_id = $session->get("user_id");
    return unless $user_id;

    my $model = Isucoin::Model->new(dbh => $self->dbh);

    return $model->get_user_by_id($user_id);
}

1;

