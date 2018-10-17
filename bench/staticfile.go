package bench

type StaticFile struct {
	Path string
	Size int64
	Hash string
}

var StaticFiles = []*StaticFile{
	&StaticFile{"/", 886, "ccc54f6c44e19c04143e6873010a16ff"},
	&StaticFile{"/css/app.afc1317c.css", 11951, "c3e39c6809659a44c3adb07b55265b0b"},
	&StaticFile{"/favicon.ico", 894, "74ba79ca3f41bb01fe12454f4f13bd96"},
	&StaticFile{"/img/isucoin_logo.png", 8988, "549012f31fcf8a328bedf6b8cab2b1af"},
	&StaticFile{"/js/Chart.Financial.js", 14403, "765490c323c5073857bf15309133edee"},
	&StaticFile{"/js/Chart.min.js", 159638, "f6c8efa65711e0cbbc99ba72997ecd0e"},
	&StaticFile{"/js/app.a6721fee.js", 19420, "ed8ef9a996459fc8d9c1d8dce4f6f571"},
	&StaticFile{"/js/chunk-vendors.3f054da5.js", 139427, "d004b96351883062178f479d06dd376a"},
	&StaticFile{"/js/moment.min.js", 51679, "8999b8b5d07e9c6077ac5ac6bc942968"},
}
