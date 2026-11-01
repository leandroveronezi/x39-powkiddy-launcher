#!/bin/sh
# Roda DENTRO DO X39, só leitura: amostra a cada 3 s o estado do RetroArch e do áudio.
# Uso: monitor.sh <amostras>
N=${1:-20}
CPU=/sys/devices/system/cpu

i=0
while [ $i -lt $N ]; do
    i=$((i + 1))
    line=$(ps | grep "/app/retroarch" | grep -v grep | head -1)
    set -- $line
    pid=$1
    if [ -z "$pid" ]; then
        echo "t=$((i * 3))s retroarch não está rodando"
        sleep 3
        continue
    fi
    rest=$(sed 's/^[^)]*) //' /proc/$pid/stat 2>/dev/null)
    set -- $rest
    state=$1; ut=${12}; st=${13}
    thr=""
    for t in /proc/$pid/task/*; do
        s=$(sed 's/^[^)]*) //' $t/stat 2>/dev/null | cut -d' ' -f1)
        w=$(cat $t/wchan 2>/dev/null)
        thr="$thr [$s:$w]"
    done
    snd=$(grep -E "^(state|hw_ptr|appl_ptr)" /proc/asound/card0/pcm0p/sub0/status 2>/dev/null | tr '\n' ' ')
    echo "t=$((i * 3))s pid=$pid estado=$state cpu(utime=$ut stime=$st) load=$(cut -d' ' -f1 /proc/loadavg) cpu0=$(cat $CPU/cpu0/cpufreq/scaling_cur_freq 2>/dev/null) cpu1on=$(cat $CPU/cpu1/online 2>/dev/null)"
    echo "     threads:$thr"
    echo "     som: $snd"
    sleep 3
done
