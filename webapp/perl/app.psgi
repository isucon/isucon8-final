use FindBin;
use lib "$FindBin::Bin/extlib/lib/perl5";
use lib "$FindBin::Bin/lib";
use File::Basename;
use Plack::Builder;
use Isucoin::Web;

my $root_dir = File::Basename::dirname(__FILE__);

my $app = Isucoin::Web->psgi($root_dir);
builder {
    enable 'ReverseProxy';
    enable 'Session::Cookie',
        session_key => 'isucoin_session',
        expires     => 3600,
        secret      => 'tonymoris';
    enable 'Static',
        path => qr!^/(?:(?:css|js|img)/|favicon\.ico$)!,
        root => $root_dir . $ENV{ISU_PUBLIC_DIR};
    $app;
};

