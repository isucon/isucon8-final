package Isucoin::Exception;

package Isucoin::Exception::UserNotFound {
    use parent "Exception::Tiny";
}

package Isucoin::Exception::OrderNotFound {
    use parent "Exception::Tiny";
}

package Isucoin::Exception::OtherOrderType {
    use parent "Exception::Tiny";
    use Class::Accessor::Lite (
        ro => [qw/type/],
    );

    sub message {
        my $self = shift;

        return sprintf("other type [%s]", $self->type);
    }
}

package Isucoin::Exception::OrderAlreadyClosed {
    use parent "Exception::Tiny";
}

package Isucoin::Exception::ParameterInvalid {
    use parent "Exception::Tiny";
}

package Isucoin::Exception::CreditInsufficiant {
    use parent "Exception::Tiny";
}

package Isucoin::Exception::NoOrderForTrade {
    use parent "Exception::Tiny";
}

package Isucoin::Exception::BankUserConflict {
    use parent "Exception::Tiny";
}

package Isucoin::Exception::Unknown {
    use parent "Exception::Tiny";
}

1;
