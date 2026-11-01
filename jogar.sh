#!/bin/sh
# Roda um jogo no X39 pela jaula, a partir do PC. Nada é gravado na memória interna:
# tudo vai para a RAM do console (/tmp/x39) e é restaurado/removido no fim.
#
# Uso:  ./jogar.sh <core> <arquivo-do-jogo-no-PC> [segundos] [--limpar]
#   core:      fceumm_libretro.so (NES)  |  genesis_plus_gx_libretro.so (Mega Drive / Master System / Game Gear)
#   segundos:  duração máxima (padrão 600; o jogo se encerra sozinho ao fim)
#   --limpar:  remove /tmp/x39 da RAM do console depois de jogar
#
# Exemplo:
#   ./jogar.sh fceumm_libretro.so "/caminho/do/jogo.nes" 300
#
# Requer o console conectado por USB com o ADB ativo (tela de transferência).
set -eu

DIR="$(cd "$(dirname "$0")" && pwd)"
PAC="$DIR/pacote"
ADB="${ADB:-adb}"
SERIAL="${SERIAL:-0123456789ABCDEF}"
A="$ADB -s $SERIAL"

CORE="${1:?faltou o core (ex.: fceumm_libretro.so)}"
ROMFILE="${2:?faltou o arquivo do jogo}"
SECS="${3:-600}"
LIMPAR=0
for x in "$@"; do [ "$x" = "--limpar" ] && LIMPAR=1; done

[ -f "$PAC/app/cores/$CORE" ] || { echo "core não encontrado: $PAC/app/cores/$CORE"; exit 1; }
[ -f "$ROMFILE" ] || { echo "arquivo do jogo não encontrado: $ROMFILE"; exit 1; }

# Confere que o X39 é o único aparelho esperado e que não sobrou jaula de uma execução anterior.
$A get-state >/dev/null 2>&1 || { echo "X39 ($SERIAL) não encontrado pelo ADB"; exit 1; }

NOME="$(basename "$ROMFILE")"
PASTA="${NOME%.*}"

echo "enviando pacote para a RAM do X39 (só o core pedido, a RAM do console é pequena)..."
$A shell 'mkdir -p /tmp/x39/roms /tmp/x39/app/cores' >/dev/null
# remove da RAM os cores que não vão ser usados e os ROMs de execuções anteriores
$A shell "cd /tmp/x39/app/cores 2>/dev/null && for f in *.so; do [ \"\$f\" = \"$CORE\" ] || rm -f \"\$f\"; done; rm -rf /tmp/x39/roms/*" >/dev/null 2>&1 || true
$A push "$PAC/app/retroarch" /tmp/x39/app/retroarch >/dev/null
$A push "$PAC/app/retroarch.cfg" /tmp/x39/app/retroarch.cfg >/dev/null
$A push "$PAC/app/cores/$CORE" "/tmp/x39/app/cores/$CORE" >/dev/null
$A push "$PAC/rt" /tmp/x39/ >/dev/null
$A push "$PAC/x39jail" /tmp/x39/x39jail >/dev/null
$A push "$PAC/run-game.sh" /tmp/x39/run-game.sh >/dev/null
$A push "$PAC/monitor.sh" /tmp/x39/monitor.sh >/dev/null
$A push "$DIR/bin/tinymix" /tmp/x39/tinymix >/dev/null
$A shell "mkdir -p \"/tmp/x39/roms/$PASTA\"" >/dev/null
$A push "$ROMFILE" "/tmp/x39/roms/$PASTA/$NOME" >/dev/null
$A shell 'chmod 755 /tmp/x39/x39jail /tmp/x39/tinymix /tmp/x39/run-game.sh /tmp/x39/monitor.sh /tmp/x39/app/retroarch /tmp/x39/rt/lib/*'

# Se uma execução anterior foi interrompida, desmonta o que sobrou.
$A shell '/tmp/x39/x39jail clean' >/dev/null 2>&1 || true

echo "rodando por até ${SECS}s: $NOME"
$A shell "/tmp/x39/run-game.sh \"$CORE\" \"$PASTA/$NOME\" $SECS" || true

echo "--- resumo do áudio"
$A shell 'grep "Stats" /tmp/x39/run.log | tail -1' | tr -d '\r' || true
$A shell 'mount | grep x39jail || echo "nenhum mount x39jail restante"' | tr -d '\r'

if [ "$LIMPAR" = 1 ]; then
    $A shell 'rm -rf /tmp/x39'
    echo "RAM do console limpa."
fi
