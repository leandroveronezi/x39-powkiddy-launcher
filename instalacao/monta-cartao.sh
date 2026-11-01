#!/bin/bash
# Monta o cartão SD do lançador a partir do PC.
# Uso: ./monta-cartao.sh <pasta-do-cartão-montado>      ex.: ./monta-cartao.sh /run/media/SEU-USUARIO/SEU-CARTAO
#
# Cria <cartão>/x39/ com o lançador, o RetroArch, os sistemas (sistemas/), os cores que eles pedem e as pastas de ROMs.
# NÃO apaga nada do cartão e não toca nos seus ROMs. O cartão deve ser FAT32.
set -euo pipefail

DIR="$(cd "$(dirname "$0")/.." && pwd)"
CARTAO="${1:?uso: $0 <pasta-do-cartão-montado>}"
[ -d "$CARTAO" ] || { echo "pasta não encontrada: $CARTAO"; exit 1; }
FS="$(findmnt -n -o FSTYPE -T "$CARTAO" 2>/dev/null || true)"
case "$FS" in vfat|msdos|exfat|"") ;; *) echo "aviso: o sistema de arquivos é '$FS'; o console lê FAT32 (vfat)";; esac

X="$CARTAO/x39"
mkdir -p "$X/app" "$X/rt/lib" "$X/cores" "$X/data"
install -m 755 "$DIR/launcher/launcher-arm" "$X/x39launcher"
install -m 755 "$DIR/x39jail" "$X/x39jail"
install -m 755 "$DIR/bin/tinymix" "$X/tinymix"
install -m 755 "$DIR/instalacao/gancho.sh" "$X/gancho.sh"
install -m 755 "$DIR/pacote/app/retroarch" "$X/app/retroarch"
install -m 644 "$DIR/pacote/app/retroarch.cfg" "$X/app/retroarch.cfg"
for f in "$DIR"/pacote/rt/lib/*; do install -m 755 "$f" "$X/rt/lib/$(basename "$f")"; done

# sistemas: o lançador não tem nenhum sistema fixo, ele lê x39/sistemas/<id>/sistema.json. Copia os do
# cartão-base SEM sobrescrever o que já existe (seus ajustes e imagens ficam).
mkdir -p "$X/sistemas"
cp -rn "$DIR/cartao-base/sistemas/." "$X/sistemas/"

# mapeamentos de controles USB (jogador 2), sem sobrescrever os seus
mkdir -p "$X/controles"
cp -rn "$DIR/cartao-base/controles/." "$X/controles/"

# cores: leva os que os sistemas do cartão pedem ("core" em cada sistema.json) e que existem no pacote
for j in "$X"/sistemas/*/sistema.json; do
    core=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("core",""))' "$j" 2>/dev/null || true)
    [ -n "$core" ] || continue
    if [ -f "$X/cores/$core" ]; then continue; fi
    if [ -f "$DIR/pacote/app/cores/$core" ]; then
        install -m 755 "$DIR/pacote/app/cores/$core" "$X/cores/$core"
    else
        echo "aviso: o core $core (de $(basename "$(dirname "$j")")) não está em pacote/app/cores; coloque-o em $X/cores/"
    fi
done

# pastas de ROMs, uma por sistema (o lançador só mostra os sistemas que tiverem jogos)
for d in "$X"/sistemas/*/; do mkdir -p "$X/roms/$(basename "$d")"; done

cp "$DIR/instalacao/LEIA-ME.txt" "$X/LEIA-ME.txt"
( cd "$X" && find . -type f \( -path './app/*' -o -path './rt/*' -o -path './cores/*' -o -path './sistemas/*' -o -name x39launcher -o -name x39jail -o -name tinymix -o -name gancho.sh \) -print0 | sort -z | xargs -0 sha256sum > MANIFESTO.sha256 )

touch "$X/ATIVO"
sync
echo "pronto: $X"
du -sh "$X" | cut -f1 | sed 's/^/tamanho: /'
echo "Copie os ROMs para as pastas de $X/roms/ (uma por sistema)."
echo "O arquivo x39/ATIVO liga o lançador no boot (se o gancho estiver instalado); apague-o para desligar."
