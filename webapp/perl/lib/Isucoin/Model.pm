package Isucoin::Model;

use strict;
use warnings;
use utf8;

use Mouse;
use Time::Moment;
use Digest;
use Data::Entropy::Algorithms qw/rand_bits/;
use Try::Tiny;
use Guard; 
use JSON::Types;

use Isucoin::Exception;
use Isubank;
use Isulogger;

use constant {
    SETTING_BANK_ENDPOINT => "bank_endpoint",
    SETTING_BANK_APPID    => "bank_appid",
    SETTING_LOG_ENDPOINT  => "log_endpoint",
    SETTING_LOG_APPID     => "log_appid",

    PASSWORD_DEFAULT_COST => 10,

    ORDER_TYPE_SELL => "sell",
    ORDER_TYPE_BUY  => "buy",
};

has dbh => (
    isa      => "DBIx::Sunny::db",
    is       => "ro",
    required => 1,
);

no Mouse;

my $datetime_to_str = sub {
    Isucoin::Exception::Unknown->throw unless $_[0];

    Time::Moment->from_string(
        "$_[0]Z", lenient => 1,
    )->strftime("%FT%TZ");
};

sub _digest {
    Digest->new("Bcrypt", cost => PASSWORD_DEFAULT_COST, salt => rand_bits 16 * 8);
}

sub init_benchmark {
    my $self = shift;

    $self->dbh->query(qq{DELETE FROM orders WHERE created_at >= '2018-10-16 10:00:00'});
    $self->dbh->query(qq{DELETE FROM trade WHERE created_at >= '2018-10-16 10:00:00'});
    $self->dbh->query(qq{DELETE FROM user WHERE created_at >= '2018-10-16 10:00:00'});
}

sub set_setting {
    my ($self, $k, $v) = @_;

    $self->dbh->query(qq{
        INSERT INTO setting (name, val) VALUES (?, ?)
            ON DUPLICATE KEY UPDATE val = VALUES(val)
    }, $k, $v);
}

sub get_setting {
    my ($self, $k) = @_;

    return $self->dbh->select_one(qq{
        SELECT val FROM setting WHERE name = ?
    }, $k);
}

sub endpoint_names {
    return [
        SETTING_BANK_ENDPOINT,
        SETTING_BANK_APPID,
        SETTING_LOG_ENDPOINT,
        SETTING_LOG_APPID,
    ];
}

sub isubank {
    my $self = shift;

    my $ep = $self->get_setting(SETTING_BANK_ENDPOINT);
    my $id = $self->get_setting(SETTING_BANK_APPID);

    return Isubank->new(endpoint => $ep, app_id => $id);
}

sub logger {
    my $self = shift;

    my $ep = $self->get_setting(SETTING_LOG_ENDPOINT);
    my $id = $self->get_setting(SETTING_LOG_APPID);

    return Isulogger->new(endpoint => $ep, app_id => $id);
}

sub send_log {
    my ($self, $tag, $v) = @_;

    my $logger = $self->logger;

    $logger->send($tag => $v);
}

sub user_signup {
    my ($self, %args) = @_;

    my $bank = $self->isubank;
    my ($name, $bank_id, $password) = @args{qw/name bank_id password/};

    # bank_idの検証
    $bank->check(bank_id => $bank_id, price => 0);

    my $digest = $self->_digest->add($password);
    my $pass = $digest->settings . $digest->bcrypt_b64digest;

    try {
        $self->dbh->query(qq{
            INSERT INTO user (bank_id, name, password, created_at)
                VALUES (?, ?, ?, NOW(6))
        }, $bank_id, $name, $pass);
    } catch {
        my $err = $_;
        if ($err =~ /^DBD::mysql::st execute failed: Duplicate entry/) {
            Isucoin::Exception::BankUserConflict->throw;
        }
        die $err;
    };

    my $user_id = $self->dbh->last_insert_id;
    $self->send_log(signup => {
        bank_id => $bank_id,
        user_id => number $user_id,
        name    => $name,
    });
}

sub _to_user_hash {
    my ($class, $user) = @_;

    return unless $user;

    return {
        id         => $user->{id},
        name       => $user->{name},
        bank_id    => $user->{bank_id},
        created_at => $datetime_to_str->($user->{created_at}),
    };
}

sub user_login {
    my ($self, %args) = @_;

    my ($bank_id, $password) = @args{qw/bank_id password/};

    my $user = $self->dbh->select_row(qq{
        SELECT * FROM user WHERE bank_id = ?
    }, $bank_id);
    if (!$user) {
        Isucoin::Exception::UserNotFound->throw;
    }

    my $digest = $self->_digest->settings($user->{password});
    $digest->add($password);
    my $pass_hash = $digest->settings . $digest->bcrypt_b64digest;

    if ($user->{password} ne $pass_hash) {
        Isucoin::Exception::UserNotFound->throw;
    }

    $self->send_log(signin => {
        user_id => number $user->{id},
    });

    return $self->_to_user_hash($user);
}

sub get_user_by_id {
    my ($self, $id) = @_;

    my $user = $self->dbh->select_row(qq{
        SELECT * FROM user WHERE id = ?
    }, $id);

    return $self->_to_user_hash($user);
}

sub get_user_by_id_with_lock {
    my ($self, $id) = @_;

    my $user = $self->dbh->select_row(qq{
        SELECT * FROM user WHERE id = ? FOR UPDATE
    }, $id);

    return $self->_to_user_hash($user);
}

sub _to_order_hash {
    my ($class, $order) = @_;

    return unless $order;

    return {
        %$order,
        closed_at  => $order->{closed_at} ? $datetime_to_str->($order->{closed_at}) : undef,
        created_at => $datetime_to_str->($order->{created_at}),
    };
}

sub get_orders_by_user_id {
    my ($self, $user_id) = @_;

    my $rows = $self->dbh->select_all(qq{
        SELECT * FROM orders WHERE user_id = ? AND (closed_at IS NULL OR trade_id IS NOT NULL) ORDER BY created_at ASC
    }, $user_id);

    return [map { $self->_to_order_hash($_) } @$rows];
}

sub get_orders_by_user_id_and_last_trade_id {
    my ($self, $user_id, $trade_id) = @_;

    my $rows = $self->dbh->select_all(qq{
        SELECT * FROM orders WHERE user_id = ? AND trade_id IS NOT NULL AND trade_id > ? ORDER BY created_at ASC
    }, $user_id, $trade_id);


    return [map { $self->_to_order_hash($_) } @$rows];
}

sub get_open_order_by_id {
    my ($self, $id) = @_;

    my $order = $self->get_order_by_id_with_lock($id);
    if (defined $order->{closed_at}) {
        Isucoin::Exception::OrderAlreadyClosed->throw;
    }
    $order->{user} = $self->get_user_by_id_with_lock($order->{user_id});

    return $order;
}

sub get_order_by_id {
    my ($self, $id) = @_;

    my $order = $self->dbh->select_row(qq{
        SELECT * FROM orders WHERE id = ?
    }, $id);

    return $self->_to_order_hash($order);
}

sub get_order_by_id_with_lock {
    my ($self, $id) = @_;

    my $order = $self->dbh->select_row(qq{
        SELECT * FROM orders WHERE id = ? FOR UPDATE
    }, $id);

    return $self->_to_order_hash($order);
}

sub get_lowest_sell_order {
    my $self = shift;

    my $order = $self->dbh->select_row(qq{
        SELECT * FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price ASC, created_at ASC LIMIT 1
    }, ORDER_TYPE_SELL);

    return $self->_to_order_hash($order);
}

sub get_highest_buy_order {
    my $self = shift;

    my $order = $self->dbh->select_row(qq{
        SELECT * FROM orders WHERE type = ? AND closed_at IS NULL ORDER BY price DESC, created_at ASC LIMIT 1
    }, ORDER_TYPE_BUY);

    return $self->_to_order_hash($order);
}

sub fetch_order_relation {
    my ($self, $order) = @_;

    $order->{user} = $self->get_user_by_id($order->{user_id});

    if ($order->{trade_id}) {
        $order->{trade} = $self->get_trade_by_id($order->{trade_id});
    }

    return $order;
}

sub add_order {
    my ($self, %args) = @_;

    my ($ot, $user_id, $amount, $price) = @args{qw/type user_id amount price/};

    if ($amount <= 0 || $price <= 0) {
        Isucoin::Exception::ParameterInvalid->throw;
    }

    my $user = $self->get_user_by_id_with_lock($user_id);
    my $bank = $self->isubank;

    if ($ot eq ORDER_TYPE_BUY) {
        my $total_price = $price * $amount;
        try {
            $bank->check(bank_id => $user->{bank_id}, price => $total_price);
        } catch {
            my $err = $_;
            $self->send_log("buy.error", {
                error   => $err->message,
                user_id => number $user->{id},
                amount  => number $amount,
                price   => number $price,
            });
            if (Isubank::Exception::CreditInsufficient->caught($err)) {
                Isucoin::Exception::CreditInsufficiant->throw;
            }
            die $err;
        };
    }
    elsif ($ot eq ORDER_TYPE_SELL) {
        # TODO 椅子の保有チェック
    }
    else {
        Isucoin::Exception::ParameterInvalid->throw;
    }

    $self->dbh->query(qq{
        INSERT INTO orders (type, user_id, amount, price, created_at)
            VALUES (?, ?, ?, ?, NOW(6))
    }, $ot, $user->{id}, $amount, $price);
    my $id = $self->dbh->last_insert_id;
    $self->send_log($ot . ".order" => {
        order_id => number $id,
        user_id  => number $user->{id},
        amount   => number $amount,
        price    => number $price,
    });

    return $self->get_order_by_id($id);
}

sub delete_order {
    my ($self, %args) = @_;

    my ($user_id, $order_id, $reason) = @args{qw/user_id order_id reason/};

    my $user = $self->get_user_by_id_with_lock($user_id);
    my $order = $self->get_order_by_id_with_lock($order_id);
    if (!$order) {
        Isucoin::Exception::OrderNotFound->throw;
    }

    if ($order->{user_id} != $user->{id}) {
        Isucoin::Exception::OrderNotFound->throw;
    }
    if (defined $order->{closed_at}) {
        Isucoin::Exception::OrderAlreadyClosed->throw;
    }

    return $self->cancel_order(order => $order, reason => $reason);
}

sub cancel_order {
    my ($self, %args) = @_;

    my ($order, $reason) = @args{qw/order reason/};

    $self->dbh->query(qq{
        UPDATE orders SET closed_at = NOW(6) WHERE id = ?
    }, $order->{id});

    $self->send_log($order->{type} . ".delete" => {
        order_id => number $order->{id},
        user_id  => number $order->{user_id},
        reason   => $reason,
    });
}

sub _to_trade_hash {
    my ($class, $trade) = @_;

    return unless $trade;

    return {
        %$trade,
        created_at => $datetime_to_str->($trade->{created_at}),
    };
}

sub get_trade_by_id {
    my ($self, $id) = @_;

    my $trade = $self->dbh->select_row(qq{
        SELECT * FROM trade WHERE id = ?
    }, $id);

    return $self->_to_trade_hash($trade);
}

sub get_latest_trade {
    my $self = shift;

    my $trade = $self->dbh->select_row(qq{
        SELECT * FROM trade ORDER BY id DESC
    });

    return $self->_to_trade_hash($trade);
}

sub get_candletick_data {
    my ($self, %args) = @_;

    my ($mt, $tf) = @args{qw/mt tf/};

    my $query = sprintf(qq{
        SELECT m.t as time, a.price as open, b.price as close, m.h as high, m.l as low
        FROM (
            SELECT
                STR_TO_DATE(DATE_FORMAT(created_at, '%s'), '%s') AS t,
                MIN(id) AS min_id,
                MAX(id) AS max_id,
                MAX(price) AS h,
                MIN(price) AS l
            FROM trade
            WHERE created_at >= ?
            GROUP BY t
        ) m
        JOIN trade a ON a.id = m.min_id
        JOIN trade b ON b.id = m.max_id
        ORDER BY m.t
    }, $tf, "%Y-%m-%d %H:%i:%s");
    my $rows = $self->dbh->select_all($query, $mt);

    my $results = [
        map {
            { %$_, time => $datetime_to_str->($_->{time}), }
        } @$rows
    ];

    return $results;
}

sub has_trade_chance_by_order {
    my ($self, $order_id) = @_;

    my $order = $self->get_order_by_id($order_id);

    my $lowest = $self->get_lowest_sell_order;
    return unless $lowest;

    my $highest = $self->get_highest_buy_order;
    return unless $highest;

    if ($order->{type} eq ORDER_TYPE_BUY) {
        if ($lowest->{price} <= $order->{price}) {
            return 1;
        }
    }
    elsif ($order->{type} eq ORDER_TYPE_SELL) {
        if ($order->{price} <= $highest->{price}) {
            return 1;
        }
    }
    else {
        Isucoin::Exception::OtherOrderType->throw(type => $order->{type});
    }

    return;
}

sub reserve_order {
    my ($self, %args) = @_;

    my ($order, $price) = @args{qw/order price/};

    my $bank = $self->isubank;

    my $p = $order->{amount} * $price;
    if ($order->{type} eq ORDER_TYPE_BUY) {
        $p *= -1;
    }

    my $id;
    try {
        $id = $bank->reserve(bank_id => $order->{user}{bank_id}, price => $p);
    }
    catch {
        my $err = $_;
        if (Isubank::Exception::CreditInsufficient->caught($err)) {
            $self->cancel_order(order => $order, reason => "reserve_failed");
            $self->send_log($order->{type} . ".error" => {
                error   => $err->message,
                user_id => number $order->{user_id},
                amount  => number $order->{amount},
                price   => number $price,
            });
            $err->rethrow;
        }
        die $err;
    };

    return $id;
}

sub commit_reserved_order {
    my ($self, %args) = @_;

    my ($order, $targets, $reserves) = @args{qw/order targets reserves/};

    $self->dbh->query(qq{
        INSERT INTO trade (amount, price, created_at) VALUES (?, ?, NOW(6))
    }, $order->{amount}, $order->{price});
    my $trade_id = $self->dbh->last_insert_id;
    $self->send_log(trade => {
        trade_id => $trade_id,
        price    => number $order->{price},
        amount   => number $order->{amount},
    });

    for my $o (@$targets, $order) {
        $self->dbh->query(qq{
            UPDATE orders SET trade_id = ?, closed_at = NOW(6) WHERE id = ?
        }, $trade_id, $o->{id});
        $self->send_log($o->{type} . ".trade", {
            order_id => number $o->{id},
            price    => number $order->{price},
            amount   => number $o->{amount},
            user_id  => number $o->{user_id},
            trade_id => number $trade_id,
        });
    }
    my $bank = $self->isubank;
    $bank->commit(@$reserves);
}

sub try_trade {
    my ($self, $order_id) = @_;

    my $order = $self->get_open_order_by_id($order_id);

    my $rest_amount = $order->{amount};
    my $unit_price = $order->{price};
    my (@reserves, @targets);
    $reserves[0] = $self->reserve_order(order => $order, price => $unit_price);

    scope_guard {
        if (scalar(@reserves) > 0) {
            my $bank = $self->isubank;
            $bank->cancel(@reserves);
        }
    };

    my $target_orders;
    if ($order->{type} eq ORDER_TYPE_BUY) {
        $target_orders = $self->dbh->select_all(qq{
            SELECT * FROM orders WHERE type = ? AND closed_at IS NULL AND price <= ? ORDER BY price ASC, created_at ASC, id ASC
        }, ORDER_TYPE_SELL, $order->{price});
    }
    elsif ($order->{type} eq ORDER_TYPE_SELL) {
        $target_orders = $self->dbh->select_all(qq{
            SELECT * FROM orders WHERE type = ? AND closed_at IS NULL AND price >= ? ORDER BY price DESC, created_at ASC, id ASC
        }, ORDER_TYPE_BUY, $order->{price});
    }

    if (scalar(@$target_orders) == 0) {
        Isucoin::Exception::NoOrderForTrade->throw;
    }

    for my $to (@$target_orders) {
        eval {  $to = $self->get_open_order_by_id($to->{id}); };
        if (my $err = $@) {
            if (Isucoin::Exception::OrderAlreadyClosed->caught($err)) {
                next;
            }
            die $err;
        };
        next if $to->{amount} > $rest_amount;

        my $rid = eval { $self->reserve_order(order => $to, price => $unit_price) };
        if (my $err = $@) {
            if (Isubank::Exception::CreditInsufficient->caught($err)) {
                next;
            }
            die $err;
        };
        push @reserves, $rid;
        push @targets, $to;
        $rest_amount -= $to->{amount};
        if ($rest_amount == 0) {
            last;
        }
    }
    if ($rest_amount > 0) {
        Isucoin::Exception::NoOrderForTrade->throw;
    }

    $self->commit_reserved_order(
        order => $order, targets => \@targets, reserves => \@reserves,
    );

    @reserves = ();
}

sub run_trade {
    my $self = shift;

    my $lowest_sell_order = $self->get_lowest_sell_order;
    # 売り注文がないため成立しない
    return unless $lowest_sell_order;

    my $highest_buy_order = $self->get_highest_buy_order;
    # 買い注文が無いため成立しない
    return unless $highest_buy_order;

    if ($lowest_sell_order->{price} > $highest_buy_order->{price}) {
        # 最安の売値が最高の買値よりも高いため成立しない
        return;
    }

    my @candidates;
    if ($lowest_sell_order->{amount} > $highest_buy_order->{amount}) {
        push @candidates, $lowest_sell_order->{id}, $highest_buy_order->{id};
    }
    else {
        push @candidates, $highest_buy_order->{id}, $lowest_sell_order->{id};
    }

    for my $order_id (@candidates) {
        eval {
            my $txn = $self->dbh->txn_scope;

            eval { $self->try_trade($order_id) };
            if (my $err = $@) {
                if (
                    Isucoin::Exception::NoOrderForTrade->caught($err) ||
                    Isucoin::Exception::OrderAlreadyClosed->caught($err) ||
                    Isubank::Exception::CreditInsufficient->caught($err)
                ) {
                    $txn->commit;
                    $err->rethrow;
                }
                $txn->rollback;
                die $err;
            };
            $txn->commit;
        };
        if (my $err = $@) {
            if (
                Isucoin::Exception::NoOrderForTrade->caught($err) ||
                Isucoin::Exception::OrderAlreadyClosed->caught($err)
            ) {
                # 注文個数の多い方で成立しなかったので少ないほうで試す
                next;
            }
            $err->rethrow if $err->can("rethrow");
            die $err;
        }
        # トレード成立したため次の取引を行う
        return $self->run_trade;
    }
}

1;
