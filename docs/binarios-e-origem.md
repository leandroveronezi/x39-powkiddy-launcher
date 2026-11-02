# Binários que não estão no repositório: de onde vêm e como refazer

O Git guarda só o código. Estes arquivos aparecem na pasta de trabalho, mas são ignorados (`.gitignore`) e **podem ser recriados**.
Ordem para refazer tudo a partir de um clone novo (precisa de Go, git, `patch` e da toolchain Linaro, ver `build/env.sh`):

```
build/prepara-fontes.sh        # baixa RetroArch e SDL do fork nos commits abaixo, aplica os patches, compila o SDL
build/build-retroarch.sh       # compila o RetroArch
build/build-cores.sh stella2014 prosystem handy ...   # compila os cores (nomes no case do script)
build/monta-pacote.sh          # rt/lib, tinymix, retroarch e cores para pacote/ e bin/
make all                       # launcher-arm, x39jail (ARM e PC) e ferramentas
make jail && ./test-nand.sh && make jail-instalar     # jaula: confira com o teste antes de instalar
```

| Arquivo (ignorado pelo Git) | Origem | Como refazer |
|---|---|---|
| `launcher/launcher-arm` | código deste repositório | `make launcher` |
| `x39jail`, `pacote/x39jail`, `x39jail-host`, `jail/*-novo` | código deste repositório (`jail/`) | `make jail`, depois `make jail-instalar` |
| `tools/diagnostico/*/*-arm`, `corewrap_libretro.so` | código deste repositório | `make evlog` e, para os demais, `go build`/`gcc` de cada pasta |
| `pacote/rt/lib/*` | **bibliotecas do próprio toolchain Linaro** (glibc 2.25, libgcc, libstdc++), só com `strip` | `build/monta-pacote.sh` (conferido: sai idêntico byte a byte) |
| `bin/tinymix` | **tinyalsa v1.0.0** (`github.com/tinyalsa/tinyalsa`), `utils/tinymix.c` + `src/mixer.c`, estático | `build/monta-pacote.sh` (sai equivalente; menor porque o binário antigo tinha símbolos de depuração) |
| `pacote/app/retroarch` | **scooper540/RetroArch-powkiddy**, commit `e3df6bc`, com 2 patches nossos | `build/prepara-fontes.sh` + `build/build-retroarch.sh` + `build/monta-pacote.sh` |
| `pacote/app/cores/*.so` | cores libretro, repositórios e commits abaixo | `build/build-cores.sh <nome>` + `build/monta-pacote.sh` |

## Fontes e patches
- **SDL 1.2:** `github.com/scooper540/SDL-1.2-powkiddy`, commit `2d50f5d`, com `sdl-sem-prints-de-depuracao.diff` e depois
  `sdl-eixos-no-joystick-proprio.diff` (nessa ordem). Compilado estático, só vídeo fbcon e joystick.
- **RetroArch:** `github.com/scooper540/RetroArch-powkiddy`, commit `e3df6bc` (1.14.0), com `retroarch-tinyalsa-mmap-start.diff` e
  `retroarch-barra-de-volume.diff`, e o `build/Makefile.x39.retroarch` copiado como `Makefile.x39`.
- **Toolchain:** Linaro gcc 7.5.0-2019.12, `arm-linux-gnueabihf`, x86_64 (o SHA-256 do tarball está no README).

## Cores (repositório em `github.com/libretro/…` e commit usado)
| Core | Repositório | Commit |
|---|---|---|
| stella2014 | stella2014-libretro | `7d1361e` |
| prosystem | prosystem-libretro | `8a88014` |
| handy | libretro-handy | `bc55d46` |
| atari800 | libretro-atari800 | `4e7fbc7` |
| picodrive | picodrive | `1890c29` |
| genesis_plus_gx | Genesis-Plus-GX | `60bee4c` |
| fceumm | libretro-fceumm | `ac0d00a` |
| quicknes | QuickNES_Core | `26bb785` |
| gambatte | gambatte-libretro | `7809bb6` |
| snes9x2005 | snes9x2005 | `a79dfe9` |
| gpsp | gpsp | `5819380` |
| mednafen_pce_fast / wswan / ngp | beetle-pce-fast-libretro / beetle-wswan-libretro / beetle-ngp-libretro | `4eee99c` / `4b01295` / `a50d5ac` |
| bluemsx / freeintv | blueMSX-libretro / FreeIntv | `e3086eb` / `ef3e0fe` |
| gw / o2em / freechaf / pokemini | gw-libretro / libretro-o2em / freechaf / PokeMini | `91d599b` / `679d6fe` / `76c7a84` / `132111b` |
| fuse / cap32 / prboom | fuse-libretro / libretro-cap32 / libretro-prboom | `e997e2b` / `8e61dd6` / `4849268` |

`build-cores.sh` clona o `HEAD` atual de cada repositório; para reproduzir exatamente, faça `git -C build/src/<pasta> checkout <commit>`
antes de compilar. Os binários compilados **não são idênticos byte a byte** entre máquinas; confira por teste (`corebench`).

## Firmware oficial
Espelho na release do GitHub (não no código). Origem, link e hashes em `docs/firmware-oficial.md`.
