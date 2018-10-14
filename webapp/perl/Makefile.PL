use ExtUtils::MakeMaker;

WriteMakefile(
    NAME          => 'Isucoin',
    VERSION_FROM  => 'lib/Isucoin.pm',
    PREREQ_PM     => {
        'Kossy' =>  '0.40',
    },
    MIN_PERL_VERSION => '5.008001'
);

