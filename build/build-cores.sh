#!/bin/bash
# Compila cores libretro para o X39 (Cortex-A5). Cada core: git clone --depth 1 + make.
# Uso: build-cores.sh <nome> [<nome> ...]   (nomes: ver o case abaixo)
# Saída: $B/cores-out/<arquivo>.so (sem strip) e $B/logs/<nome>.log
. "$(dirname "$0")/env.sh"
mkdir -p $B/src $B/logs $B/cores-out
export CFLAGS="-O2 -pipe $ARCHF -fPIC -I$PREFIX/include"
export CXXFLAGS="$CFLAGS"
unset LDFLAGS

build() {
    nome=$1; repo=$2; mk=$3; so=$4; shift 4
    dir=$B/src/$nome
    log=$B/logs/$nome.log
    : > $log
    if [ ! -d $dir ]; then
        timeout 400 git clone -q --depth 1 --recursive https://github.com/libretro/$repo.git $dir >> $log 2>&1 \
            || { echo "$nome: FALHOU o clone"; return; }
    fi
    commit=$(git -C $dir log -1 --format='%h %ad' --date=short)
    ( cd $dir && make -f $mk "$@" -j"$(nproc)" >> $log 2>&1 )
    rc=$?
    f=$(find $dir -maxdepth 2 -name "$so" | head -1)
    if [ $rc -eq 0 ] && [ -n "$f" ]; then
        cp "$f" $B/cores-out/$so
        echo "$nome ($commit): OK  $(du -h $B/cores-out/$so | cut -f1)"
    else
        echo "$nome ($commit): FALHOU (rc=$rc) — veja $log"
    fi
}

for c in "$@"; do
    case $c in
        stella2014) build stella2014 stella2014-libretro Makefile stella2014_libretro.so platform=unix ;;
        prosystem)  build prosystem  prosystem-libretro  Makefile prosystem_libretro.so platform=unix ;;
        handy)      build handy      libretro-handy      Makefile handy_libretro.so platform=unix ;;
        gambatte)   build gambatte   gambatte-libretro   Makefile gambatte_libretro.so platform=unix ;;
        snes9x2005) build snes9x2005 snes9x2005          Makefile snes9x2005_libretro.so platform=unix ;;
        pce_fast)   build beetle-pce-fast beetle-pce-fast-libretro Makefile mednafen_pce_fast_libretro.so platform=unix ;;
        gpsp)       build gpsp       gpsp                Makefile gpsp_libretro.so platform=unix ;;
        wswan)      build beetle-wswan beetle-wswan-libretro Makefile mednafen_wswan_libretro.so platform=unix ;;
        ngp)        build beetle-ngp  beetle-ngp-libretro  Makefile mednafen_ngp_libretro.so platform=unix ;;
        bluemsx)    build blueMSX     blueMSX-libretro     Makefile.libretro bluemsx_libretro.so platform=unix ;;
        freeintv)   build FreeIntv    FreeIntv             Makefile  freeintv_libretro.so platform=unix ;;
        vecx)       build libretro-vecx libretro-vecx      Makefile.libretro vecx_libretro.so platform=unix ;;
        atari800)   build libretro-atari800 libretro-atari800 Makefile atari800_libretro.so platform=unix ;;
        gw)         build gw-libretro gw-libretro Makefile gw_libretro.so platform=unix ;;
        o2em)       build libretro-o2em libretro-o2em Makefile o2em_libretro.so platform=unix ;;
        freechaf)   build freechaf freechaf Makefile freechaf_libretro.so platform=unix ;;
        pokemini)   build PokeMini PokeMini Makefile.libretro pokemini_libretro.so platform=unix ;;
        fuse)       build fuse fuse-libretro Makefile.libretro fuse_libretro.so platform=unix ;;
        cap32)      build cap32 libretro-cap32 Makefile cap32_libretro.so platform=unix ;;
        prboom)     build prboom libretro-prboom Makefile prboom_libretro.so platform=unix ;;
        *) echo "core desconhecido: $c" ;;
    esac
done
