#!/bin/sh
# Roda NO CONSOLE (enviado para /tmp/x39 pelo teste-sem-instalar.sh suave|jogo).
# Uso: suave.sh <segundos> [argumentos do lançador]
# Em vez de matar o menu original, CONGELA só a interface dele (SIGSTOP): congelada ela não redesenha
# a tela nem lê botões. O manager e o msg_server seguem vivos (watchdog, USB, ADB).
# Tudo volta com SIGCONT: ao fim do teste, em Ctrl-C/SIGTERM, ou pela rede de segurança abaixo.
SEG="${1:-120}"
CONGELA="${CONGELA:-0}"   # 1 = congela a interface original (SIGSTOP). NÃO USE: o manager reinicia o sistema (ver README)
ARGS="${2:--demo}"   # -demo (padrão) ou, para jogar de verdade, "-card /tmp/x39card"
trap '' HUP

pids_de() {
    for p in /proc/[0-9]*; do
        [ "$(cat $p/comm 2>/dev/null)" = "$1" ] && echo "${p#/proc/}"
    done
}

UI=$(pids_de launcher)
retoma() {
    for p in $UI; do kill -CONT $p 2>/dev/null; done
    echo "retomado ui=[$UI] $(date)" >> /tmp/x39/batida
}

echo "inicio suave ui=[$UI] $(date)" > /tmp/x39/batida
[ "$CONGELA" != 1 ] || [ -n "$UI" ] || { echo "sem interface original para congelar; abortei" >> /tmp/x39/batida; exit 1; }

# rede de segurança independente: retoma a interface original mesmo se este script morrer
[ "$CONGELA" = 1 ] && ( sleep $((SEG + 30)); for p in $UI; do kill -CONT $p 2>/dev/null; done ) </dev/null >/dev/null 2>&1 &

[ "$CONGELA" = 1 ] && trap retoma EXIT INT TERM
[ "$CONGELA" = 1 ] && for p in $UI; do kill -STOP $p; done

X39_DEBUG=1 /tmp/x39/x39launcher $ARGS -t "$SEG" -root /tmp/x39 > /tmp/x39/demo.log 2>&1 </dev/null &
LP=$!
while kill -0 $LP 2>/dev/null; do
    echo "vivo $(date)" >> /tmp/x39/batida
    sleep 5
done
