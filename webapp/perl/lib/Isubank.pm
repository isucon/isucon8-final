package Isubank;

use strict;
use warnings;
use utf8;

=pod

=head1 Isubank

Isubank はISUBANK APIクライアントです

=head1 SYNOPSIS

    use Isubank;

    # endpoint: ISUBANK APIを利用するためのエンドポイントURI
    # app_id:   ISUBANK APIを利用するためのアプリケーションID
    my $bank = Isubank->new(endpoint => $endpoint, app_id => $app_id);

=end

=cut

use Furl;
use Try::Tiny;
use JSON::XS qw/decode_json encode_json/;

use Mouse;

has endpoint => (
    isa      => "Str",
    is       => "ro",
    required => 1,
);

has app_id => (
    isa      => "Str",
    is       => "ro",
    required => 1,
);

has client => (
    isa => "Furl",
    is => "ro",
    default => sub {
        Furl->new;
    },
);

no Mouse;

=pod

=head1 DESCRIPTION

=head1 METHODS

=head2 check

check は残高確認です
Reserve による予約済み残高は含まれません

=cut
sub check {
    my ($self, %args) = @_;

    my ($bank_id, $price) = @args{qw/bank_id price/};

    my $res = $self->_request("/check" => {
        bank_id => $bank_id,
        price   => $price,
    });

    if ($res->{status} == 200) {
        return;
    }
    if ($res->{error} eq "bank_id not found") {
        Isubank::Exception::NoUser->throw;
    }
    if ($res->{error} eq "credit is insufficient") {
        Isubank::Exception::CreditInsufficient->throw;
    }

    Isubank::Exception::CheckFailed->throw(error => $res->{error});
}

=pod

=head2 reserve

check は仮決済(残高の確保)を行います

=cut
sub reserve {
    my ($self, %args) = @_;

    my ($bank_id, $price) = @args{qw/bank_id price/};

    my $res = $self->_request("/reserve" => {
        bank_id => $bank_id,
        price   => $price,
    });

    if ($res->{status} != 200) {
        if ($res->{error} eq "credit is insufficient") {
            Isubank::Exception::CreditInsufficient->throw;
        }
        Isubank::Exception::ReserveFailed->throw(error => $res->{error});
    }

    return $res->{reserve_id};
}

=pod

=head2 commit

commit は決済の確定を行います
正常に仮決済処理を行っていればここでエラーになることはありません

=cut
sub commit {
    my ($self, @reserve_ids) = @_;

    my $res = $self->_request("/commit" => {
        reserve_ids => \@reserve_ids,
    });

    if ($res->{status} != 200) {
        if ($res->{error} eq "credit is insufficient") {
            Isubank::Exception::CreditInsufficient->throw;
        }
        Isubank::Exception::CommitFailed->throw(error => $res->{error});
    }
}

=pod

=head2 cancel

commit は決済の取り消しを行います

=cut
sub cancel {
    my ($self, @reserve_ids) = @_;

    my $res = $self->_request("/cancel" => {
        reserve_ids => \@reserve_ids,
    });

    if ($res->{status} != 200) {
        Isubank::Exception::CommitFailed->throw(error => $res->{error});
    }
}

sub _request {
    my ($self, $p, $v) = @_;

    my $body = encode_json $v;
    my $res;
    try {
        $res = $self->client->post(
            $self->endpoint . $p,
            [
                "Content-Type"  => "application/json",
                "Authorization" => "Bearer " . $self->app_id,
            ],
            $body,
        );
    } catch {
        my $err = $_;
        Isubank::Exception::FailRequest->throw(error => $err);
    };

    my $json = decode_json $res->body;
    $json->{status} = $res->status;
    return $json;
}

package Isubank::Exception::FailRequest {
    use parent "Exception::Tiny";
    use Class::Accessor::Lite (
        ro => [qw/error/],
    );

    sub message {
        my $self = shift;

        return sprintf("isubank failed request: %s", $self->error);
    }
}

# 仮決済時または残高チェック時に残高が不足している
package Isubank::Exception::CreditInsufficient {
    use parent "Exception::Tiny";
}


# いすこん銀行にアカウントが存在しない
package Isubank::Exception::NoUser {
    use parent "Exception::Tiny";
}

package Isubank::Exception::CheckFailed {
    use parent "Exception::Tiny";
    use Class::Accessor::Lite (
        ro => [qw/error/],
    );

    sub message {
        my $self = shift;

        return sprintf("check failed. err: %s", $self->error);
    }
}

package Isubank::Exception::ReserveFailed {
    use parent "Exception::Tiny";
    use Class::Accessor::Lite (
        ro => [qw/error/],
    );

    sub message {
        my $self = shift;

        return sprintf("reserve failed. err: %s", $self->error);
    }
}

package Isubank::Exception::CommitFailed {
    use parent "Exception::Tiny";
    use Class::Accessor::Lite (
        ro => [qw/error/],
    );

    sub message {
        my $self = shift;

        return sprintf("commit failed. err: %s", $self->error);
    }
}

package Isubank::Exception::CancelFailed {
    use parent "Exception::Tiny";
    use Class::Accessor::Lite (
        ro => [qw/error/],
    );

    sub message {
        my $self = shift;

        return sprintf("cancel failed. err: %s", $self->error);
    }
}

1;
