#!/bin/sh
# Etapa 1 do "jogar sem instalar": nada é gravado na memória interna e o menu original NÃO é parado.
# Envia o lançador para a RAM (/tmp/x39), inicia em modo demonstração, desligado do ADB, e deixa
# um registro de batimentos para vermos, depois de reconectar, se ele sobreviveu à retirada do USB.
#
# Uso (no PC, console com ADB):
#   teste-sem-instalar.sh enviar   # envia e inicia (demo, 120 s)
#   (desconectar os dois USB, olhar a tela e os botões; reconectar depois de ~2 min)
#   teste-sem-instalar.sh ver      # mostra o registro
#   teste-sem-instalar.sh limpar   # apaga /tmp/x39 da RAM
set -eu
DIR="$(cd "$(dirname "$0")/.." && pwd)"
ADB="${ADB:-adb}"
A="$ADB -s ${SERIAL:-0123456789ABCDEF}"
SEG="${SEG:-120}"

# NUNCA mexer no lançador/arquivos com um jogo aberto: o RetroArch ficaria órfão e o menu dele repete teclas para sempre
case "${1:-}" in
enviar|suave|jogo|entrar)
    if [ "$($A shell 'ps | grep "/app/retroarch" | grep -v grep | wc -l' 2>/dev/null | tr -d '\r')" != 0 ] && [ -z "${FORCA:-}" ]; then
        echo "HÁ UM JOGO ABERTO no console: feche-o antes (FORCA=1 ignora esta trava)"; exit 1
    fi ;;
esac
case "${1:-}" in
enviar)
    $A get-state >/dev/null 2>&1 || { echo "X39 não encontrado pelo ADB"; exit 1; }
    $A shell 'mkdir -p /tmp/x39' >/dev/null
    $A push "$DIR/launcher/launcher-arm" /tmp/x39/x39launcher >/dev/null
    $A shell 'chmod 755 /tmp/x39/x39launcher'
    # embrulho em segundo plano: ignora HUP, sem entrada, registra um batimento a cada 5 s
    $A shell "cat > /tmp/x39/demo.sh <<'EOF'
#!/bin/sh
trap '' HUP
X39_DEBUG=1 /tmp/x39/x39launcher -demo -t $SEG -root /tmp/x39 >/tmp/x39/demo.log 2>&1 </dev/null &
LP=\$!
echo \"inicio \$(date)\" > /tmp/x39/batida
while kill -0 \$LP 2>/dev/null; do echo \"vivo \$(date)\" >> /tmp/x39/batida; sleep 5; done
echo \"fim \$(date)\" >> /tmp/x39/batida
EOF
chmod 755 /tmp/x39/demo.sh
/tmp/x39/demo.sh </dev/null >/dev/null 2>&1 &
sleep 2; cat /tmp/x39/batida"
    echo "iniciado por ${SEG}s. Pode desconectar os dois USB e olhar o console."
    ;;
jogo)
    # Jogar de verdade, só em RAM: monta um "cartão" em /tmp/x39card com os ROMs dados e os cores certos,
    # congela a interface original (como em "suave") e roda o lançador (sem -demo).
    # Uso: teste-sem-instalar.sh jogo <rom.sms|.gg|.a26|.bin|.zip> ...   (SEG=duração, padrão 600)
    shift
    [ $# -gt 0 ] || { echo "informe os ROMs"; exit 2; }
    $A get-state >/dev/null 2>&1 || { echo "X39 não encontrado pelo ADB"; exit 1; }
    TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
    CORES=""
    for f in "$@"; do
        case "${f##*.}" in
            zip) unzip -qq -o -d "$TMP/x" "$f" ;;
            *)   mkdir -p "$TMP/x"; cp "$f" "$TMP/x/" ;;
        esac
    done
    find "$TMP/x" -type f | while read -r f; do
        case "$(echo "${f##*.}" | tr A-Z a-z)" in
            sms|sg) sys=sms ;; gg) sys=gamegear ;; a26|bin) sys=atari2600 ;; a78) sys=atari7800 ;;
            *) echo "ignorado (extensão não prevista): $f"; continue ;;
        esac
        mkdir -p "$TMP/card/roms/$sys"; cp "$f" "$TMP/card/roms/$sys/"
    done
    [ -d "$TMP/card/roms" ] || { echo "nenhum ROM reconhecido"; exit 1; }
    for s in $(ls "$TMP/card/roms"); do
        case $s in sms|gamegear) CORES="$CORES picodrive_libretro.so" ;; atari2600) CORES="$CORES stella2014_libretro.so" ;; atari7800) CORES="$CORES prosystem_libretro.so" ;; esac
    done
    $A shell 'rm -rf /tmp/x39card; mkdir -p /tmp/x39/app/cores /tmp/x39card/cores /tmp/x39card/data' >/dev/null
    $A push "$TMP/card/roms" /tmp/x39card/ >/dev/null
    # o lançador não tem sistemas fixos: leva o sistema.json (e a imagem do console) de cada sistema usado
    $A shell 'mkdir -p /tmp/x39card/sistemas' >/dev/null
    for s in $(ls "$TMP/card/roms"); do $A push "$DIR/cartao-base/sistemas/$s" /tmp/x39card/sistemas/ >/dev/null; done
    $A push "$DIR/cartao-base/controles" /tmp/x39card/ >/dev/null  # mapeamentos de controles USB (jogador 2 e menu)
    for c in $(echo $CORES | tr ' ' '\n' | sort -u); do $A push "$DIR/pacote/app/cores/$c" /tmp/x39card/cores/ >/dev/null; done
    $A push "$DIR/launcher/launcher-arm" /tmp/x39/x39launcher >/dev/null
    $A push "$DIR/pacote/app/retroarch" /tmp/x39/app/retroarch >/dev/null
    $A push "$DIR/pacote/app/retroarch.cfg" /tmp/x39/app/retroarch.cfg >/dev/null
    $A push "$DIR/pacote/rt" /tmp/x39/ >/dev/null
    $A push "$DIR/pacote/x39jail" /tmp/x39/x39jail >/dev/null
    $A push "$DIR/bin/tinymix" /tmp/x39/tinymix >/dev/null
    $A push "$DIR/instalacao/suave.sh" /tmp/x39/suave.sh >/dev/null
    $A shell 'chmod 755 /tmp/x39/x39launcher /tmp/x39/x39jail /tmp/x39/tinymix /tmp/x39/suave.sh /tmp/x39/app/retroarch /tmp/x39/rt/lib/*; /tmp/x39/x39jail clean >/dev/null 2>&1; df /tmp | tail -1; ls -R /tmp/x39card | head -20' | tr -d '\r'
    echo "preparado. Para entrar: ${0##*/} entrar"
    ;;
entrar)
    # Congela a interface original e abre o lançador de verdade (ROMs de /tmp/x39card). Pare com SEG=segundos.
    $A shell "/tmp/x39/suave.sh $SEG '-card /tmp/x39card -grab' </dev/null >/dev/null 2>&1 &
sleep 3; cat /tmp/x39/batida; tail -n 5 /tmp/x39/demo.log" | tr -d '\r'
    echo "no ar por ${SEG}s com a interface original congelada."
    ;;
suave)
    # Etapa 2: congela só a interface do menu original (SIGSTOP) durante o teste; ver suave.sh
    $A get-state >/dev/null 2>&1 || { echo "X39 não encontrado pelo ADB"; exit 1; }
    $A shell 'mkdir -p /tmp/x39' >/dev/null
    $A push "$DIR/launcher/launcher-arm" /tmp/x39/x39launcher >/dev/null
    $A push "$DIR/instalacao/suave.sh" /tmp/x39/suave.sh >/dev/null
    $A shell 'chmod 755 /tmp/x39/x39launcher /tmp/x39/suave.sh'
    $A shell "/tmp/x39/suave.sh $SEG </dev/null >/dev/null 2>&1 &
sleep 2; cat /tmp/x39/batida"
    echo "modo suave por ${SEG}s: a interface original ficou congelada; ao fim ela volta sozinha."
    ;;
ver)
    $A shell 'echo "--- batida"; tail -n 4 /tmp/x39/batida; echo "--- log do lançador"; tail -n 15 /tmp/x39/demo.log; echo "--- processos"; ps | grep -E "x39launcher|manager|launcher" | grep -v grep' | tr -d '\r'
    ;;
limpar)
    $A shell 'rm -rf /tmp/x39' && echo "RAM limpa."
    ;;
*)
    echo "uso: $0 enviar|suave|jogo|entrar|ver|limpar"; exit 2 ;;
esac
