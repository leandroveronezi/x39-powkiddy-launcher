#!/bin/bash
# Faz o backup da memória interna (NAND) do X39 pelo ADB, SÓ LENDO: nada é gravado no aparelho.
# Confere o MD5 de cada imagem no PC contra o MD5 calculado no próprio console.
# Uso: backup-nand.sh <pasta-de-destino>     (fora do repositório; ~245 MB; precisa dos dois cabos USB)
# Variáveis: ADB (caminho do adb), SERIAL (padrão 0123456789ABCDEF)
set -u
DEST="${1:-}"
ADB="${ADB:-adb}"
A="$ADB -s ${SERIAL:-0123456789ABCDEF}"
PARTS="nand0 nand0p1 nand0p2 nand0p3 nand0p4 nand0p5 nand0p6 nand0p7"

[ -n "$DEST" ] || { echo "uso: $0 <pasta-de-destino>"; exit 2; }
DIR_REPO="$(cd "$(dirname "$0")/.." && pwd)"
case "$(realpath -m "$DEST")/" in "$DIR_REPO"/*) echo "ERRO: o backup é um dump do SEU aparelho: não guarde dentro do repositório"; exit 2 ;; esac
$A get-state >/dev/null 2>&1 || { echo "X39 não encontrado pelo ADB (dois cabos ligados?)"; exit 1; }
if ls "$DEST"/nand0*.img >/dev/null 2>&1; then echo "ERRO: já existem imagens em $DEST; não sobrescrevo. Use outra pasta."; exit 2; fi
mkdir -p "$DEST" && cd "$DEST" || exit 1
echo "livre em $DEST: $(df -h . | tail -1 | awk '{print $4}')"

for n in $PARTS; do
    echo "lendo /dev/$n ..."
    timeout 600 $A exec-out "dd if=/dev/$n bs=1M 2>/dev/null" > "$n.img" || { echo "FALHOU $n"; exit 1; }
done
md5sum nand0*.img > MD5SUMS-pc.txt
for n in $PARTS; do timeout 300 $A shell "md5sum /dev/$n" 2>&1 | tr -d '\r'; done > MD5SUMS-console.txt

ok=1
for n in $PARTS; do
    pc=$(grep " $n.img$" MD5SUMS-pc.txt | cut -d' ' -f1); co=$(grep "/dev/$n\$" MD5SUMS-console.txt | cut -d' ' -f1)
    if [ -n "$pc" ] && [ "$pc" = "$co" ]; then echo "OK    $n"; else echo "DIFERE $n (pc=$pc console=$co)"; ok=0; fi
done
[ $ok = 1 ] && echo "Backup completo e conferido em $DEST" || { echo "ATENÇÃO: há diferenças; repita antes de gravar qualquer coisa no aparelho"; exit 1; }
