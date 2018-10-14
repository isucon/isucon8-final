package Isucoin::Model;

use strict;
use warnings;
use utf8;

use Mouse;

has dbh => (
    isa      => "DBIx::Sunny",
    is       => "ro",
    required => 1,
);

no Mouse;

sub init_benchmark {
}

sub set_setting {
}

sub endpoint_names {
    return [qw/
        bank_endpoint
        bank_appid
        log_endpoint
        log_appid
    /];
}

sub user_signup {
}

sub user_login {
}

sub get_user_by_id {
}

sub get_trade_by_id {
}

sub get_latest_trade {
}

sub get_order_by_user_id_and_last_trade_id {
}

sub fetch_order_relation {
}

sub get_candletick_data {
}

sub get_lowest_sell_order {
}

sub get_highest_buy_order {
}

sub add_order {
}

sub has_trade_chance_by_order {
}

sub run_trade {
}

sub get_orders_by_user_id {
}

sub delete_order {
}

1;
