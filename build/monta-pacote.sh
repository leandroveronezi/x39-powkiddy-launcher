#!/bin/bash
# Recria os binários do pacote que NÃO ficam no repositório (veja docs/binarios-e-origem.md):
#   pacote/rt/lib/*   bibliotecas glibc 2.25 e libgcc/libstdc++ do próprio toolchain Linaro (só strip)
#   bin/tinymix       tinyalsa v1.0.0, estático, para ligar o mixer de áudio
#   pacote/app/retroarch  o RetroArch compilado por build-retroarch.sh, sem símbolos
#   pacote/app/cores/*.so os cores compilados por build-cores.sh
# Pré-requisitos: toolchain (env.sh), build/prepara-fontes.sh e build/build-retroarch.sh já executados.
# Uso: build/monta-pacote.sh [pasta-de-saída]   (padrão: a raiz do repositório)
set -e
. "$(dirname "$0")/env.sh"
OUT="${1:-$B/..}"
mkdir -p "$OUT/pacote/rt/lib" "$OUT/pacote/app/cores" "$OUT/bin"

echo "== rt/lib (do toolchain)"
SYS="$TC/arm-linux-gnueabihf/libc"
for f in ld-linux-armhf.so.3 libc.so.6 libm.so.6 libdl.so.2 librt.so.1 libpthread.so.0; do
    cp -L "$SYS/lib/$f" "$OUT/pacote/rt/lib/$f"
done
for f in libgcc_s.so.1 libstdc++.so.6; do
    src=$(find "$TC/arm-linux-gnueabihf" -name "$f" -not -path '*debug*' | head -1)
    [ -n "$src" ] && cp -L "$src" "$OUT/pacote/rt/lib/$f"
done
for f in "$OUT"/pacote/rt/lib/*; do $STRIP --strip-unneeded "$f"; done
chmod 755 "$OUT"/pacote/rt/lib/*

echo "== tinymix (tinyalsa v1.0.0, estático)"
if [ ! -d "$B/src/tinyalsa/.git" ]; then
    git clone -q -b v1.0.0 --depth 1 https://github.com/tinyalsa/tinyalsa.git "$B/src/tinyalsa"
fi
$CC -O2 -static $ARCHF -I"$B/src/tinyalsa/include" -o "$OUT/bin/tinymix" \
    "$B/src/tinyalsa/utils/tinymix.c" "$B/src/tinyalsa/src/mixer.c"
$STRIP "$OUT/bin/tinymix"

echo "== RetroArch"
if [ -f "$B/src/RetroArch-powkiddy/retroarch" ]; then
    $STRIP -o "$OUT/pacote/app/retroarch" "$B/src/RetroArch-powkiddy/retroarch"
else
    echo "  falta compilar: rode build/build-retroarch.sh"
fi

echo "== cores (de cores-out)"
if ls "$B"/cores-out/*.so >/dev/null 2>&1; then
    for c in "$B"/cores-out/*.so; do $STRIP -o "$OUT/pacote/app/cores/$(basename "$c")" "$c"; done
else
    echo "  falta compilar: rode build/build-cores.sh <nome> ... (nomes no case do script)"
fi

echo "== jaula (Go)"
echo "  rode: make jail && ./test-nand.sh && make jail-instalar   (ver README)"
ls -l "$OUT/pacote/rt/lib" "$OUT/bin" 2>/dev/null | tail -15
