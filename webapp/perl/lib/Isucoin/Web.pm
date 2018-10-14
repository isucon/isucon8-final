package Isucoin::Web;

use 5.28.0;
use warnings;
use utf8;

use DBIx::Sunny;
use Kossy;
use Plack::Session;
use Time::Moment;
use JSON::Types;

use Isucoin::Model;

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

        $model->user_signup(
            name     => $name,
            bank_id  => $bank_id,
            password => $password,
        );

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
    my $user = $model->user_login(bank_id => $bank_id, password => $password);

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

    my $last_trade_id;
    my $lt = Time::Moment->now_epoch(0);
    my %res;
    my $cursor = $c->req->parameters->{cursor};
    if ($last_trade_id = $cursor) {
        my $trade = $model->get_trade_by_id($last_trade_id);
        if ($trade) {
            $lt = Time::Moment->from_string("$trade->{created_at}Z", lenient => 1);
        }
    }

    my $latest_trade = $model->get_latest_trade;
    $res{cursor} = $latest_trade->{id};

    my $user = $self->user_by_request($c);
    if ($user) {
        my $orders = $model->get_order_by_user_id_and_last_trade_id(
            $user->{id}, $last_trade_id,
        );
        for my $order (@$orders) {
            $model->fetch_order_relation($order);
        }
        $res{traded_orders} = $orders;
    }

    my $by_sec_time = Time::Moment->now_utc->minus_seconds(300);
    if ($lt->is_after($by_sec_time)) {
        $by_sec_time = $lt->with_precision(0);
    }
    $res{chart_by_sec} = $model->get_candletick_data(
        mt => $by_sec_time,
        tf => "%Y-%m-%d %H:%i:%s",
    );

    my $by_min_time = Time::Moment->now_utc->minus_minutes(300);
    if ($lt->is_after($by_min_time)) {
        $by_min_time = $lt->with_precision(-1);
    }
    $res{chart_by_min} = $model->get_candletick_data(
        mt => $by_min_time,
        tf => "%Y-%m-%d %H:%i:00",
    );

    my $by_hour_time = Time::Moment->now_utc->minus_hours(48);
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

        $order = $model->add_order(
            type    => $type,
            user_id => $user->{id},
            amount  => $amount,
            price   => $price,
        );

        $txn->commit;
    };

    my $trade_chance = $model->has_trade_chance_by_order($order->{id});
    if ($trade_chance) {
        $model->run_trade;
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

router "DELETE" => [qw/login_required/] => "/order/{id}" => sub {
    my ( $self, $c )  = @_;

    my $user = $c->stash->{user};
    my $order_id = $c->args->{id};

    my $dbh = $self->dbh;
    my $model = Isucoin::Model->new(dbh => $dbh);
    {
        my $txn = $dbh->txn_scope;

        $model->delete_order(
            user_id  => $user->{id},
            order_id => $order_id,
            reason   => "canceled",
        );

        $txn->commit;
    };

    return $c->render_json({ id => $order_id });
};

sub dbh {
    my $self = shift;
    $self->{_dbh} ||= do {
        my $dsn = "dbi:mysql:database=$ENV{ISU_DB_NAME};host=$ENV{ISU_DB_HOST};port=$ENV{ISU_DB_PORT}";
        DBIx::Sunny->connect($dsn, $ENV{ISU_DB_USER}, $ENV{ISU_DB_PASS}, {
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

