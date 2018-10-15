package Isulogger;

use strict;
use warnings;
use utf8;

use Furl;
use JSON::XS qw/encode_json/;

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

sub send {
    my ($self, $tag, $data) = @_;
}

sub request {
    my ($self, $p, $v) = @_;

    my $body = encode_json $v;
    my $res = $self->client->post(
        $self->endpoint . "/" . $p,
        [
            "Content-Type"  => "application/json",
            "Authorization" => "Bearer " . $self->app_id,
        ],
        $body,
    );
    return if $res->is_success;

    Isulogger::Exception->throw(
        code => $res->code, body => $res->body,
    );
}

package Isulogger::Exception {
    use parent "Exception::Tiny";
    use Class::Accessor::Lite (
        ro => [qw/code body/],
    );

    sub message {
        my $self = shift;

        return sprintf(
            "logger status is not ok. code: %d, body: %s",
            $self->code, $self->body,
        );
    }
}

1;
