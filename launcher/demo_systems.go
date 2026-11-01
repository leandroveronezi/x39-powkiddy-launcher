package main

// demoSystems é a tabela de sistemas usada SÓ pelos modos -demo e -preview (nunca no uso real: no uso real
// todos os sistemas vêm do cartão, em x39/sistemas/<id>/sistema.json).
func demo(id, name, short, core string, exts []string, c1, c2 uint32) System {
	return System{ID: id, Name: name, Short: short, Core: core, Exts: exts, C1: c1, C2: c2, Order: 100}
}

func demoSystems() []System {
	return []System{
		demo("sms", "Master System", "SMS", "picodrive_libretro.so", []string{"sms", "sg", "zip"}, 0xE8344A, 0x7A1226),
		demo("gamegear", "Game Gear", "GG", "picodrive_libretro.so", []string{"gg", "zip"}, 0x3B82F6, 0x1E3A8A),
		demo("atari2600", "Atari 2600", "2600", "stella2014_libretro.so", []string{"a26", "bin", "zip"}, 0xF59E0B, 0x9A4A06),
		demo("atari7800", "Atari 7800", "7800", "prosystem_libretro.so", []string{"a78", "bin", "zip"}, 0x94A3B8, 0x3D4B60),
		demo("lynx", "Atari Lynx", "LYNX", "handy_libretro.so", []string{"lnx", "zip"}, 0x10B981, 0x064E3B),
		demo("atari800", "Atari 5200 / 800", "5200", "atari800_libretro.so", []string{"a52", "atr", "xex", "bin", "zip"}, 0x6B8AAE, 0x2A3C57),
		demo("pce", "PC Engine", "PCE", "mednafen_pce_fast_libretro.so", []string{"pce", "zip"}, 0xFB7185, 0x8E1236),
		demo("wonderswan", "WonderSwan", "WS", "mednafen_wswan_libretro.so", []string{"ws", "wsc", "zip"}, 0x38BDF8, 0x0A4468),
		demo("ngp", "Neo Geo Pocket", "NGP", "mednafen_ngp_libretro.so", []string{"ngp", "ngc", "zip"}, 0xA78BFA, 0x45208A),
		demo("msx", "MSX", "MSX", "bluemsx_libretro.so", []string{"rom", "mx1", "mx2", "dsk", "cas", "zip"}, 0x22D3EE, 0x0E5870),
		demo("coleco", "ColecoVision", "CV", "bluemsx_libretro.so", []string{"col", "rom", "zip"}, 0xD946EF, 0x6A1A78),
		demo("intellivision", "Intellivision", "INTV", "freeintv_libretro.so", []string{"int", "rom", "bin", "zip"}, 0xFACC15, 0x946006),
		demo("md", "Mega Drive", "MD", "picodrive_libretro.so", []string{"md", "gen", "smd", "bin", "zip"}, 0x818CF8, 0x2E2F7A),
	}
}
