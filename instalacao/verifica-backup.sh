#!/bin/bash
# Confere o backup da NAND ANTES de qualquer gravação no aparelho. Sai com erro (1) se algo faltar ou não bater.
# Uso: verifica-backup.sh <pasta-do-backup> [--console]
#   sem --console: confere se as 8 imagens existem e batem com MD5SUMS-pc.txt
#   --console:     também lê o MD5 de cada partição NO APARELHO e compara com o backup
#                  (igual = o aparelho continua como estava no backup; diferente = foi alterado depois)
# Variáveis: ADB, SERIAL (como em backup-nand.sh)
set -u
DIR="${1:-}"; MODO="${2:-}"
ADB="${ADB:-adb}"; A="$ADB -s ${SERIAL:-0123456789ABCDEF}"
PARTS="nand0 nand0p1 nand0p2 nand0p3 nand0p4 nand0p5 nand0p6 nand0p7"
[ -d "$DIR" ] || { echo "SEM BACKUP: pasta '$DIR' não existe. Rode backup-nand.sh antes de gravar qualquer coisa."; exit 1; }
cd "$DIR" || exit 1
[ -f MD5SUMS-pc.txt ] || { echo "SEM BACKUP: falta MD5SUMS-pc.txt"; exit 1; }
for n in $PARTS; do [ -s "$n.img" ] || { echo "SEM BACKUP: falta $n.img"; exit 1; }; done
md5sum -c --quiet MD5SUMS-pc.txt || { echo "BACKUP CORROMPIDO: algum MD5 não bate"; exit 1; }
echo "Backup no PC: OK (8 imagens conferem)"
if [ "$MODO" = "--console" ]; then
    $A get-state >/dev/null 2>&1 || { echo "X39 não encontrado pelo ADB"; exit 1; }
    ok=1
    for n in $PARTS; do
        pc=$(grep " $n.img$" MD5SUMS-pc.txt | cut -d' ' -f1)
        co=$(timeout 300 $A shell "md5sum /dev/$n" 2>&1 | tr -d '\r' | cut -d' ' -f1)
        if [ "$pc" = "$co" ]; then echo "  $n: igual ao aparelho"; else echo "  $n: DIFERENTE do aparelho (já foi alterado?)"; ok=0; fi
    done
    [ $ok = 1 ] && echo "Aparelho: igual ao backup" || { echo "AVISO: o aparelho mudou desde o backup; este backup NÃO é do estado atual"; exit 1; }
fi
