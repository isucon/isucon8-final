package Isubank;

use strict;
use warnings;
use utf8;

use Mouse;

has endpoint => (
    isa      => "Str",
    is       => "ro",
    required => 1,
);

has id => (
    isa      => "UInt",
    is       => "ro",
    required => 1,
);

no Mouse;

sub check {
    my ($self, %args) = @_;

    my ($bank_id, $price) = @args{qw/bank_id price/};
}

package Isubank::Exception::CreditInsufficient {
    use parent "Exception::Tiny";
}

1;
