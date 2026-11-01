#!/bin/bash
# Importa uma coleção de ROMs (uma pasta por jogo, com ROM + capa) para a pasta de ROMs do cartão do X39,
# limpando os nomes. NUNCA altera a pasta de origem: só lê e copia.
#
# Uso: importa-roms.sh [--dry-run] <pasta-origem> <id-do-sistema> <pasta-roms-destino>
#   id do sistema: sms | gamegear | atari2600 | atari7800 | lynx | pce | ... (ver launcher/systems.go)
#   destino:       <cartão>/x39/roms   (cria <destino>/<sistema>/<Jogo>/<Jogo>.<ext> e <Jogo>.jpg|png)
#
# O que ele resolve:
#   - nomes com sobra de URL do download:  "Jogo.bin?md5=...&expires=..."  ->  "Jogo.bin"
#   - zips com uma ROM dentro: extrai (o cartão fica sem depender de descompactar na RAM do console)
#   - capa (.jpg/.jpeg/.png) copiada ao lado da ROM, com o mesmo nome
set -u
DRY=0
[ "${1:-}" = "--dry-run" ] && { DRY=1; shift; }
SRC="${1:?faltou a pasta de origem}"; SYS="${2:?faltou o id do sistema}"; DST="${3:?faltou a pasta de destino}"
[ -d "$SRC" ] || { echo "origem não existe: $SRC"; exit 1; }

case "$SYS" in
    sms)        EXTS="sms sg" ;;
    gamegear)   EXTS="gg" ;;
    atari2600)  EXTS="a26 bin" ;;
    atari7800)  EXTS="a78 bin" ;;
    lynx)       EXTS="lnx" ;;
    atari800)   EXTS="a52 atr xex bin" ;;
    pce)        EXTS="pce" ;;
    wonderswan) EXTS="ws wsc" ;;
    ngp)        EXTS="ngp ngc" ;;
    msx)        EXTS="rom mx1 mx2 dsk cas" ;;
    coleco)     EXTS="col rom" ;;
    intellivision) EXTS="int rom bin" ;;
    md)         EXTS="md gen smd bin" ;;
    *) echo "sistema desconhecido: $SYS"; exit 1 ;;
esac
IMG="jpg jpeg png"

lower() { echo "$1" | tr 'A-Z' 'a-z'; }
tem() { local e v; e=$(lower "$1"); shift; for v in "$@"; do [ "$v" = "$e" ] && return 0; done; return 1; }
# nome sem a sobra "?md5=...": corta no primeiro "?"
limpo() { echo "${1%%\?*}"; }

ok=0; sem_rom=0; capas=0; zips=0; multi=0
shopt -s nullglob
for d in "$SRC"/*/; do
    nome=$(basename "$d")
    [ "$nome" = "images" ] && continue
    rom=""; romnome=""; cap=""
    for f in "$d"*; do
        [ -f "$f" ] || continue
        b=$(basename "$f"); l=$(limpo "$b"); e="${l##*.}"
        if tem "$e" zip; then rom="$f"; romnome="$l"; zips=$((zips+1))
        elif tem "$e" $EXTS; then rom="$f"; romnome="$l"
        elif tem "$e" $IMG; then cap="$f"; capaext=$(lower "$e")
        fi
    done
    if [ -z "$rom" ]; then sem_rom=$((sem_rom+1)); echo "sem ROM reconhecida: $nome"; continue; fi
    base=$(limpo "$nome")
    out="$DST/$SYS/$base"
    if [ $DRY = 1 ]; then ok=$((ok+1)); [ -n "$cap" ] && capas=$((capas+1)); continue; fi
    mkdir -p "$out"
    if tem "${romnome##*.}" zip; then
        # extrai só arquivos com extensão de ROM do sistema
        t=$(mktemp -d)
        unzip -qq -o -d "$t" "$rom" 2>/dev/null
        n=0
        while IFS= read -r -d '' x; do
            e="${x##*.}"
            if tem "$e" $EXTS; then cp "$x" "$out/$(basename "$x")"; n=$((n+1)); fi
        done < <(find "$t" -type f -print0)
        rm -rf "$t"
        if [ $n -eq 0 ]; then rmdir "$out" 2>/dev/null; sem_rom=$((sem_rom+1)); echo "zip sem ROM do sistema: $nome"; continue; fi
        [ $n -gt 1 ] && multi=$((multi+1))
        romfinal=$(ls "$out" | head -1)
        stem="${romfinal%.*}"
    else
        cp "$rom" "$out/$romnome"
        stem="${romnome%.*}"
    fi
    [ -n "$cap" ] && { cp "$cap" "$out/$stem.$capaext"; capas=$((capas+1)); }
    ok=$((ok+1))
done
echo "jogos importados: $ok  (com capa: $capas, vindos de zip: $zips, zips com mais de uma ROM: $multi, ignorados: $sem_rom)"
[ $DRY = 1 ] && echo "(simulação: nada foi copiado)"
exit 0
