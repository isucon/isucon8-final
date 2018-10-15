package Isubank;

use strict;
use warnings;
use utf8;

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
    isa      => "UInt",
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

sub check {
    my ($self, %args) = @_;

    my ($bank_id, $price) = @args{qw/bank_id price/};

    my $res = $self->request("/check" => {
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

sub reserve {
    my ($self, %args) = @_;

    my ($bank_id, $price) = @args{qw/bank_id price/};

    my $res = $self->request("/reserve" => {
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

sub commit {
    my ($self, @reserve_ids) = @_;

    my $res = $self->request("/commit" => {
        reserve_ids => \@reserve_ids,
    });

    if ($res->{status} != 200) {
        if ($res->{error} eq "credit is insufficient") {
            Isubank::Exception::CreditInsufficient->throw;
        }
        Isubank::Exception::CommitFailed->throw(error => $res->{error});
    }
}

sub cancel {
    my ($self, @reserve_ids) = @_;

    my $res = $self->request("/cancel" => {
        reserve_ids => \@reserve_ids,
    });

    if ($res->{status} != 200) {
        Isubank::Exception::CommitFailed->throw(error => $res->{error});
    }
}

sub request {
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

package Isubank::Exception::CreditInsufficient {
    use parent "Exception::Tiny";
}


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
