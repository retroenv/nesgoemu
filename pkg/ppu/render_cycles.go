package ppu

var lineReadCycles = [341]bool{
	257: true, 259: true, 265: true, 267: true, 273: true, 275: true, 281: true, 283: true,
	289: true, 291: true, 297: true, 299: true, 305: true, 307: true, 313: true, 315: true,
	337: true, 339: true,
}

var lineUpdateCycles = [341]bool{
	8: true, 16: true, 24: true, 32: true, 40: true, 48: true, 56: true, 64: true,
	72: true, 80: true, 88: true, 96: true, 104: true, 112: true, 120: true, 128: true,
	136: true, 144: true, 152: true, 160: true, 168: true, 176: true, 184: true, 192: true,
	200: true, 208: true, 216: true, 224: true, 232: true, 240: true, 248: true, 256: true,
	257: true, 259: true, 265: true, 267: true, 273: true, 275: true, 281: true, 283: true,
	289: true, 291: true, 297: true, 299: true, 305: true, 307: true, 313: true, 315: true,
	328: true, 336: true, 337: true, 339: true,
}
