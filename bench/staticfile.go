package bench

type StaticFile struct {
	Path string
	Size int64
	Hash string
}

var StaticFiles = []*StaticFile{
	&StaticFile{"/", 886, "3a571469b58349f869846a51327e7cff"},
	&StaticFile{"/css/app.033eaee3.css", 11992, "7bf63f337dc2e96d62aefa4c7c482732"},
	&StaticFile{"/favicon.ico", 894, "74ba79ca3f41bb01fe12454f4f13bd96"},
	&StaticFile{"/img/isucoin_logo.png", 8988, "549012f31fcf8a328bedf6b8cab2b1af"},
	&StaticFile{"/js/Chart.Financial.js", 14403, "765490c323c5073857bf15309133edee"},
	&StaticFile{"/js/Chart.min.js", 159638, "f6c8efa65711e0cbbc99ba72997ecd0e"},
	&StaticFile{"/js/app.2be81752.js", 19425, "ece197c60a70f36b87d2a390428095b9"},
	&StaticFile{"/js/chunk-vendors.3f054da5.js", 139427, "d004b96351883062178f479d06dd376a"},
	&StaticFile{"/js/moment.min.js", 51679, "8999b8b5d07e9c6077ac5ac6bc942968"},
}
