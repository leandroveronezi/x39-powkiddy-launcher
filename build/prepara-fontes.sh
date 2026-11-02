#!/bin/bash
# Baixa e prepara as fontes do RetroArch e do SDL do fork Powkiddy (nos commits que usamos), aplica nossos patches
# e compila o SDL como biblioteca estática. Depois rode build-retroarch.sh e build-cores.sh.
# Uso: build/prepara-fontes.sh        (precisa de git, patch e da toolchain; veja env.sh)
set -e
. "$(dirname "$0")/env.sh"
mkdir -p "$B/src"
cd "$B/src"

clona() {   # clona <url> <pasta> <commit>
    [ -d "$2/.git" ] || git clone -q "$1" "$2"
    git -C "$2" checkout -q "$3"
}

clona https://github.com/scooper540/SDL-1.2-powkiddy.git SDL-1.2-powkiddy 2d50f5d
clona https://github.com/scooper540/RetroArch-powkiddy.git RetroArch-powkiddy e3df6bc

# patches: só se ainda não foram aplicados (a ordem do SDL importa: o segundo parte do resultado do primeiro)
aplica() {   # aplica <pasta> <patch>
    if git -C "$1" apply --check "$B/patches/$2.diff" 2>/dev/null; then
        git -C "$1" apply "$B/patches/$2.diff"; echo "aplicado: $2"
    else
        echo "já aplicado (ou não aplica): $2"
    fi
}
aplica SDL-1.2-powkiddy sdl-sem-prints-de-depuracao
aplica SDL-1.2-powkiddy sdl-eixos-no-joystick-proprio
aplica RetroArch-powkiddy retroarch-tinyalsa-mmap-start
aplica RetroArch-powkiddy retroarch-barra-de-volume
cp "$B/Makefile.x39.retroarch" RetroArch-powkiddy/Makefile.x39

# SDL 1.2: só vídeo fbcon e joystick, estático, com NEON
cd SDL-1.2-powkiddy
./configure --host=arm-linux-gnueabihf --prefix="$PREFIX" --disable-shared --enable-static \
    --disable-video-x11 --disable-video-opengl --disable-video-directfb --disable-video-svga \
    --disable-video-ps2gs --disable-video-ps3 --enable-video-fbcon --enable-joystick \
    --disable-audio --disable-cdrom --disable-alsa --disable-oss --disable-esd --disable-pulseaudio \
    --disable-arts --disable-nas --disable-diskaudio --disable-dummyaudio --disable-input-tslib \
    --disable-rpath --enable-arm-simd --enable-arm-neon
make -j"$(nproc)"
make install
echo "SDL pronto em $PREFIX. Agora: build/build-retroarch.sh e build/build-cores.sh <nomes>"
