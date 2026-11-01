#!/bin/sh
# Roda DENTRO DO X39 (como root, via ADB).
# Uso: run-game.sh <core.so em /app/cores> "<jogo>/<arquivo>" [segundos]
# Estrutura em /tmp/x39 (RAM): app/ (retroarch, cfg, cores), rt/lib (glibc), data/ (saves em RAM),
# roms/ (ROMs copiados para este teste), x39jail e tinymix.
# EXTRA_ARGS="--max-frames=600" repassa argumentos ao RetroArch (ex.: contar quadros para medir a velocidade).
# APPEND=/app/arquivo.cfg anexa uma configuração extra (ex.: vídeo null para medir só o emulador).
X=/tmp/x39
ROMS=${ROMS:-$X/roms}
CORE=$1
ROM=$2
SECS=${3:-120}
MIX=$X/tinymix

# CPU: o console fica em "interactive" com máximo de 600 MHz; o SuperX usa 900 MHz em performance.
# Guardo os valores atuais e devolvo ao sair. É estado volátil (volta ao normal no reinício).
CPU=/sys/devices/system/cpu
G0=$CPU/cpu0/cpufreq
G1=$CPU/cpu1/cpufreq

guarda_cpu() {
    echo "$(cat $G0/scaling_governor) $(cat $G0/scaling_max_freq)" > $X/cpu.bak
}
ajusta_cpu() {
    echo 900000 > $G0/scaling_max_freq
    echo performance > $G0/scaling_governor
    if [ -d $G1 ]; then
        echo 900000 > $G1/scaling_max_freq
        echo performance > $G1/scaling_governor
    fi
}
restaura_cpu() {
    [ -f $X/cpu.bak ] || return
    read gov max < $X/cpu.bak
    echo "$gov" > $G0/scaling_governor; echo "$max" > $G0/scaling_max_freq
    if [ -d $G1 ]; then
        echo "$gov" > $G1/scaling_governor; echo "$max" > $G1/scaling_max_freq
    fi
}

restaura() {
    restaura_cpu
    $MIX 30 0; $MIX 35 0; $MIX 15 40
    # devolve o conteúdo da tela que havia antes do teste
    [ -f $X/fb.bak ] && dd if=$X/fb.bak of=/dev/fb0 bs=65536 2>/dev/null
}
trap restaura EXIT INT TERM

mkdir -p $X/data/system $X/data/saves $X/data/states $X/data/screens
dd if=/dev/fb0 of=$X/fb.bak bs=65536 2>/dev/null
$MIX 30 1; $MIX 35 1; $MIX 15 40
guarda_cpu
ajusta_cpu

$X/x39jail run -dev -t "$SECS" \
    ${X39_REALCORE:+-env X39_REALCORE=$X39_REALCORE} $X39_JAIL_EXTRA \
    -env SDL_VIDEODRIVER=fbcon -env SDL_NOMOUSE=1 -env SDL_MOUSEDEV=/dev/null \
    -ro $X/app=/app \
    -ro $X/rt/lib=/lib \
    -rw $X/data=/data \
    -ro "$ROMS"=/roms \
    -- /app/retroarch -v -c /app/retroarch.cfg ${APPEND:+--appendconfig $APPEND} $EXTRA_ARGS -L "/app/cores/$CORE" "/roms/$ROM" \
    > $X/run.log 2>&1
echo "retroarch saiu com $? (log em $X/run.log)"
