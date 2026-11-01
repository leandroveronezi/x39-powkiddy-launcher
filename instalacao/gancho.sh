#!/bin/sh
# Lógica do lançador do X39. FICA NO CARTÃO (x39/gancho.sh).
#
# Chamado em segundo plano pelo arquivo mínimo da memória interna, DEPOIS de o menu original já ter
# sido iniciado. O cartão já está montado em $X39_CARD (padrão /mnt/card). Este script:
#   1) copia o lançador para a RAM e o inicia POR CIMA do menu original;
#   2) só quando o lançador confirma que desenhou a primeira tela, encerra o menu original;
#   3) se o lançador falhar antes disso, o menu original continua sem ter sido tocado;
#   4) quando o lançador termina, restaura o menu original.
#
# Variáveis de ensaio (nunca definidas no boot real):
#   X39_CARD  X39_RAIZ  X39_ARGS  X39_MANAGER  X39_WD  X39_CONFIRMA  X39_ENSAIO=1 (não mata nem reinicia nada do firmware)

CARD="${X39_CARD:-/mnt/card}"
R="${X39_RAIZ:-/tmp/x39}"
LOG=/tmp/x39-boot.log
LOGCARD="$CARD/x39/boot.log"
MGR="${X39_MANAGER:-manager}"

log() { echo "$1" >> $LOG; echo "$1" >> "$LOGCARD" 2>/dev/null; }

# PIDs de processos pelo nome (comm); o nosso lançador se chama x39launcher, sem confusão
pids_de() {
    for p in /proc/[0-9]*; do
        c=$(cat $p/comm 2>/dev/null)
        for n in "$@"; do [ "$c" = "$n" ] && echo "${p#/proc/}"; done
    done
}

if [ -f "$LOGCARD" ] && [ "$(wc -c < "$LOGCARD" 2>/dev/null)" -gt 100000 ]; then : > "$LOGCARD"; fi
log "x39: gancho.sh iniciado $(date 2>/dev/null)"

# Proteção contra falhas repetidas: se a tentativa anterior não chegou à primeira tela, pula esta uma vez.
if [ -f "$CARD/x39/INICIANDO" ]; then
    rm -f "$CARD/x39/INICIANDO"
    log "x39: a tentativa anterior não terminou; o menu original segue (apague x39/ATIVO para desligar o lançador)"
    exit 0
fi
touch "$CARD/x39/INICIANDO"

# 1) copia o programa para a RAM (o cartão é FAT: sem garantia de permissão de execução)
mkdir -p "$R/app/cores" "$R/rt/lib"
for f in x39launcher x39jail tinymix; do
    cp "$CARD/x39/$f" "$R/$f" || { log "x39: falha ao copiar $f; o menu original segue"; rm -f "$CARD/x39/INICIANDO"; exit 0; }
done
for f in retroarch retroarch.cfg; do
    cp "$CARD/x39/app/$f" "$R/app/$f" || { log "x39: falha ao copiar app/$f; o menu original segue"; rm -f "$CARD/x39/INICIANDO"; exit 0; }
done
cp "$CARD"/x39/rt/lib/* "$R/rt/lib/" || { log "x39: falha ao copiar rt/lib; o menu original segue"; rm -f "$CARD/x39/INICIANDO"; exit 0; }
chmod 755 "$R/x39launcher" "$R/x39jail" "$R/tinymix" "$R/app/retroarch" "$R"/rt/lib/*

# O módulo de teclas pode estar em modo reduzido (tiny_mode=1; parâmetro somente leitura).
if [ "$(cat /sys/module/owl_gpio_matrix_adcjoystick/parameters/tiny_mode 2>/dev/null)" = 1 ] && [ -z "$X39_ENSAIO" ]; then
    log "x39: recarregando owl_gpio_matrix_adcjoystick com tiny_mode=0"
    rmmod owl_gpio_matrix_adcjoystick >> $LOG 2>&1 &&
        insmod /lib/modules/3.10.0/owl_gpio_matrix_adcjoystick.ko tiny_mode=0 >> $LOG 2>&1
fi

# 2) guarda os PIDs do menu original ANTES de iniciar o nosso lançador
STOCK_MGR=$(pids_de manager)
STOCK_UI=$(pids_de launcher)
log "x39: menu original: manager=[$STOCK_MGR] launcher=[$STOCK_UI]"

rm -f /tmp/x39-vivo
LLOG=/tmp/x39-launcher.log
: > $LLOG
"$R/x39launcher" ${X39_WD--watchdog} -okfile "$CARD/x39/INICIANDO" -vivo /tmp/x39-vivo -card "$CARD/x39" -root "$R" $X39_ARGS >> $LLOG 2>&1 < /dev/null &
LPID=$!

# 3) espera o lançador confirmar a primeira tela (até 25 s)
i=0
while [ $i -lt "${X39_CONFIRMA:-25}" ] && ! [ -e /tmp/x39-vivo ] && kill -0 $LPID 2>/dev/null; do i=$((i + 1)); sleep 1; done

if ! [ -e /tmp/x39-vivo ]; then
    log "x39: o lançador não confirmou a primeira tela; o menu original segue sem ter sido tocado"
    kill $LPID 2>/dev/null
    tail -n 20 $LLOG >> "$LOGCARD" 2>/dev/null
    exit 0   # INICIANDO fica: o próximo boot pula a tentativa (apague x39/INICIANDO para tentar de novo)
fi

# 4) o lançador está no ar: agora sim, encerra o menu original (SIGKILL: sem rotinas de saída)
if [ -n "$X39_ENSAIO" ]; then
    log "x39: (ensaio) encerraria manager=[$STOCK_MGR] launcher=[$STOCK_UI]"
else
    [ -n "$STOCK_MGR" ] && kill -9 $STOCK_MGR 2>/dev/null
    [ -n "$STOCK_UI" ] && kill -9 $STOCK_UI 2>/dev/null
    log "x39: lançador no ar; menu original encerrado"
fi

# 5) espera o lançador terminar (por exemplo, "Menu original do console") e restaura o menu original
wait $LPID
rc=$?
cat $LLOG >> $LOG
tail -n 30 $LLOG >> "$LOGCARD" 2>/dev/null
log "x39: o lançador terminou ($rc)"
rm -f "$CARD/x39/INICIANDO"
sync
if [ -n "$X39_ENSAIO" ]; then
    log "x39: (ensaio) reiniciaria o menu original"
else
    # evita serviços duplicados: encerra o que sobrou do menu original antes de iniciar um novo
    kill -9 $(pids_de msg_server audio_service launcher manager) 2>/dev/null
    log "x39: reiniciando o menu original"
    $MGR &
fi
exit 0
