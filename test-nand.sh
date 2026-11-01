#!/bin/sh
# (X39JAIL=jail/x39jail-novo ./test-nand.sh testa a jaula nova antes de instalá-la)
# Envia o x39jail para a RAM (/tmp) do X39 por ADB, roda o teste de isolamento
# e remove o binário. Não grava nada na memória interna.
#
# Uso: ./test-nand.sh [serial-adb]
set -eu

DIR="$(cd "$(dirname "$0")" && pwd)"
ADB="${ADB:-adb}"
SERIAL="${1:-0123456789ABCDEF}"
A="$ADB -s $SERIAL"

$A push "${X39JAIL:-$DIR/x39jail}" /tmp/x39jail-bin >/dev/null
$A shell 'chmod 755 /tmp/x39jail-bin'
echo "md5 local : $(md5sum "${X39JAIL:-$DIR/x39jail}" | cut -d' ' -f1)"
echo "md5 X39   : $($A shell 'md5sum /tmp/x39jail-bin' | tr -d '\r' | cut -d' ' -f1)"

rc=0
for modo in "" "-dev"; do
    echo "=== x39jail test $modo"
    $A shell "/tmp/x39jail-bin test $modo; echo \"exit=\$?\"" | tr -d '\r' | tee /dev/stderr | grep -q '^exit=0$' || rc=1
done

$A shell 'rm -f /tmp/x39jail-bin'
$A shell 'mount | grep x39jail || echo "nenhum mount x39jail restante"' | tr -d '\r'
exit $rc
