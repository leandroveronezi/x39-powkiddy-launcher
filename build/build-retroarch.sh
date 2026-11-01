#!/bin/sh
# Compila o RetroArch (fork powkiddy) para o X39 com o makefile minimal.
set -e
. "$(dirname "$0")/env.sh"
cd $B/src/RetroArch-powkiddy
export SDL_CONFIG="$PREFIX/bin/sdl-config"
export FREETYPE_CONFIG=true
export INC_DIR="$PREFIX/include"
export LIB_DIR="$PREFIX/lib"
export LD_FLAGS="-L$PREFIX/lib"
make -f Makefile.x39 -j"$(nproc)" "$@"
