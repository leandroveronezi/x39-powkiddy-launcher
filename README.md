# Emuladores no Powkiddy X39 sem tocar na memória interna

Estado em 24/10/2026. Um lançador próprio (Go, sem dependências) abre emuladores RetroArch **compilados por nós**
dentro de uma jaula (`x39jail`) que impede o programa de alcançar a NAND. Tudo roda a partir da RAM e de um
cartão SD só nosso. O firmware original do console (CPS, FBA, NES, GB, GBC, GBA, MD, Neo Geo, PS1, SNES) não é
alterado e continua disponível; o nosso trabalho cobre o que ele **não** tem.

## Sistemas cobertos (cores em `pacote/app/cores`, para Cortex-A5, sem rede)

| Sistema | Core | Testado no X39 |
|---|---|---|
| Master System / Game Gear / SG-1000 | picodrive | só como Mega Drive (100% da velocidade); falta ROM de SMS |
| Mega Drive (opcional; o original já tem) | picodrive | sim, 48001 quadros/s = 100%, sem XRUN |
| Atari 2600 / 7800 / Lynx / 5200-800 | stella2014 / prosystem / handy / atari800 | compilados; faltam ROMs |
| PC Engine, WonderSwan, Neo Geo Pocket | mednafen_pce_fast / wswan / ngp | compilados; faltam ROMs |
| MSX, ColecoVision, Intellivision | bluemsx / freeintv | compilados; faltam ROMs e BIOS |
| NES, GB/GBC, GBA, SNES (só se quiser) | quicknes (100%) / fceumm (86-89%), gambatte, gpsp, snes9x2005 | NES medido; os outros só compilados |

BIOS: Atari 5200/800, MSX, ColecoVision e Intellivision precisam das ROMs de sistema do respectivo console
(copie para `x39/data/<sistema>/system/`). PC Engine só precisa de BIOS para jogos em CD.

## Uso no dia a dia

1. Prepare o cartão (FAT32) no PC: `instalacao/monta-cartao.sh /run/media/.../SEU-CARTAO`
   (cria `x39/` com o lançador, o RetroArch, os cores usados e uma pasta de ROMs por sistema; 28 MB).
2. Copie os ROMs para `x39/roms/<sistema>/`. O lançador só mostra os sistemas que tiverem jogos.
3. Por ADB, sem instalar nada: `./jogar.sh <core> <arquivo-do-jogo> [segundos] [--limpar]` roda um jogo direto
   da RAM do console (testes).

## Capas dos jogos

Na lista, o cartão do sistema à esquerda dá lugar à **capa do jogo selecionado** (PNG ou JPEG). A capa é a imagem de
**mesmo nome do ROM**; para jogos em pasta própria, na falta dela, a primeira imagem da pasta (a sua coleção já vem
assim: `roms/<sistema>/<Jogo>/<Jogo>.jpg`). Ela só é pedida **250 ms depois de parar no jogo** (rolar a lista não
carrega nada), é decodificada **em segundo plano** por um único trabalhador, reduzida a no máximo 128×128 (sem ampliar
capas pequenas) e as **8 últimas** ficam em cache (~100 KB cada). Imagem inválida ou grande demais (>6 MB, >2048 px)
é ignorada sem travar nada; sem capa continua o cartão do sistema. A varredura anota a capa de cada jogo na mesma
leitura de pastas que já fazia, sem acessos extras ao cartão.

## USB, 2 jogadores, fone e HDMI (descobertas de 24/10/2026)

- O aparelho tem 3 portas: **USB-C** e **USB-A 1** (os dois cabos do PC, modo dispositivo, controlador
  `b0240000.usb`) e **USB-A 2** (livre, **modo hospedeiro**, controlador `aotg_hcd`). O kernel tem `aotg_hcd`,
  `usbhid`/`hid-generic` e `joydev`: um teclado (Logitech) na USB-A 2 apareceu como `event2`/`event3`.
- **Lançador:** `StartHotplug` (`launcher/usbinput.go`) vigia a USB-A 2 a cada 2 s e lê teclado e controles USB como
  botões do console, com plug a qualquer momento. Teclado: setas, Enter/Espaço = A, Esc/Backspace = B, Y = favorito,
  F1/Tab = Menu, `[` `]` = L1/R1, PgUp/PgDn = L2/R2. Controle: direcional/hat e botões comuns (A/B para menu).
- **2 jogadores:** com um **controle** (dispositivo com `js`) na USB-A 2 **ao abrir o jogo**, o jogador 2 é ligado; sem
  ele (nada, ou só USB-A 1/USB-C) é 1 jogador. O lançador passa `-input /dev/input/jsN=js1` e `eventM=event2` à jaula
  (a jaula confere no host que é dispositivo de entrada, grupo 13, e recria **só esse nó**) e anexa
  `input_max_users = 2` e as linhas `input_player2_*` de `x39/controles/<vendor>-<produto>.cfg` (ou `padrao.cfg`).
  O controle precisa estar plugado **antes** de abrir o jogo (a jaula monta o `/dev` no início). O `padrao.cfg` é
  **provisório**: calibre com `tools/diagnostico/evlog` quando houver um controle (cada modelo numera os botões de um jeito).
- **Controle calibrado:** "USB gamepad" estilo Super Nintendo (vendor `081f`, produto `e401`; entra como `js1`/`event2`):
  direcional = eixos 0/1 (0, 127, 255); botões evdev X=288 A=289 B=290 Y=291 L=292 R=293 Select=296 Start=297 (SDL: X=0 A=1 B=2
  Y=3 L=4 R=5 Select=8 Start=9). Perfil em `cartao-base/controles/081f-e401.cfg`; as linhas `lancador_<tecla>` dão os
  botões do **menu** (sem elas vale o mapa genérico). O `padrao.cfg` provisório coincidiu com este controle.
- **Jaula nova instalada em 24/10/2026:** passou no `test-nand.sh` (todas as verificações `[PASS]`, sem mounts restantes);
  hash `8cbcaa86…`.
- A jaula nova (`jail/x39jail-novo`, com `-input` e o comando `features`) **precisa passar no `test-nand.sh` antes de
  ser usada**: `X39JAIL=jail/x39jail-novo ./test-nand.sh` e só então `make jail-instalar`. Enquanto a jaula antiga
  estiver em uso, o lançador pergunta `x39jail features` e não usa o controle (o jogo abre normalmente com 1 jogador).
- **Fone:** há um detector (`/sys/class/switch/h2w`, hoje `4` sem fone plugado) e o mixer tem volumes separados de
  fone e alto-falante. O lançador hoje liga o alto-falante (`speaker on off switch`, controle 35) em todo jogo: com o
  fone plugado é possível que o som saia pelos dois. **Não tratado ainda**: primeiro ler `h2w` com o fone plugado.
- **HDMI:** existe `fb1` ("hdmi") e o detector `/sys/class/switch/hdmi`; o jogo desenha em `fb0`. Falta testar com TV
  se o hardware já copia a tela. O GPU tem a biblioteca de blit 2D `libpvr2d.so` (ideia futura para ampliar a imagem
  sem gastar CPU; hoje ~70% da CPU de um jogo leve é o vídeo do RetroArch).

## Cores novos (24/10/2026): carregam no X39, ainda sem jogo

`gw` (Game & Watch, `.mgw`), `o2em` (Odyssey 2, `.bin`; **precisa de `o2rom.bin`** em `data/odyssey2/system/`),
`freechaf` (Channel F; sem BIOS ele usa uma simulada) e `pokemini` (Pokémon Mini, `.min`; sem BIOS). Os quatro
compilam em menos de 30 s, são ARMv7 sem `sdiv/udiv` e sem importação de rede, e `corebench <core> -` os carrega e
identifica dentro da jaula. Falta rodar jogos de verdade (e calibrar com `corewrap`/`corebench`) quando houver ROMs.

### Mais três (24/10/2026)

`fuse` (ZX Spectrum; `.tzx .tap .z80 .sna .szx .trd .scl .dsk`), `cap32` (Amstrad CPC; `.dsk .sna .tap .cdt .voc .cpr`) e
`prboom` (Doom; `.wad`, o jogo é o IWAD e o core precisa de `prboom.wad` na pasta `data/doom/system/`, gerado em
`build/src/prboom`). Compilam em 36 s, ARMv7 sem `sdiv/udiv` e sem rede, e `corebench <core> -` os identifica no
aparelho. **`cap32` precisa das ROMs do Amstrad** (`cpc464.rom`, `cpc664.rom`, `cpc6128.rom`...) em
`data/amstrad/system/`; sem elas o core aborta ao iniciar (o repositório do core não as traz). **O Doom não foi
rodado**: os WADs do Freedoom (28 MB) não cabem na RAM do console para o teste e o WAD shareware não pôde ser
baixado; no uso real o WAD fica no cartão.

## Sistemas e cores vêm do cartão (nada fixo no programa)

O lançador não tem nenhum sistema embutido: ele lê `x39/sistemas/<id>/sistema.json` na inicialização. Para
acrescentar um sistema basta uma pasta, sem recompilar:

```
x39/sistemas/master-system/sistema.json   (+ console.png | console.jpg | console.jpeg, opcional)
x39/cores/genesis_plus_gx_libretro.so     (o core; roda dentro da jaula)
x39/roms/master-system/<jogos>            (o id da pasta é o mesmo em sistemas/ e roms/)
```
```json
{ "nome": "Master System", "sigla": "SMS", "core": "genesis_plus_gx_libretro.so",
  "extensoes": ["sms", "sg", "zip"], "cor1": "#E8344A", "cor2": "#7A1226", "ordem": 10,
  "imagem": "console.png",
  "retroarch": { "video_smooth": true, "audio_max_timing_skew": 0.02 } }
```
- Obrigatórios: `core` (só o nome do `.so`, sem pastas) e `extensoes`. O resto tem padrão (nome = pasta, sigla = nome,
  cores cinza-azuladas, `ordem` 100; sem `imagem` usa `console.png|jpg|jpeg` se existir).
- `retroarch`: opções do RetroArch só deste sistema (texto, número ou verdadeiro/falso), anexadas ao `x39.cfg` do jogo.
  Chaves com `directory`, `path`, `dir`, `file`, `url`, `cmd`, `netplay`, `network`, `driver`, `login`, `password`
  ou `log` são recusadas (quem manda nisso é o lançador), e valores com aspas ou quebra de linha também.
- A imagem do console (PNG de preferência, com transparência) é lida **uma vez** na inicialização, reduzida ao
  tamanho do cartão e fica em cache; até 2048×2048 px e 6 MB.
- Um sistema com erro (JSON quebrado, sem core, pasta com nome inválido) é **ignorado com aviso** em `stderr`; nunca
  derruba o lançador. Só aparecem os sistemas que têm ao menos um jogo.
- `cartao-base/sistemas/` traz os 13 sistemas de partida; `instalacao/monta-cartao.sh` os copia **sem sobrescrever**
  o que já estiver no cartão e leva os cores que eles pedem. Teste sem cartão: `launcher -preview <pasta> -card <x39 simulado>`.
- O RetroArch carrega UM core por vez e o lançador copia só o core usado para a RAM (~43 MB no `/tmp`); por isso o
  cartão pode ter quantos cores quiser (cada um tem de 1 a 6 MB), mas só um ocupa RAM por vez.

## Importar uma coleção de ROMs

`tools/importa-roms.sh [--dry-run] <origem> <sistema> <x39/roms>` copia uma coleção (uma pasta por jogo, com ROM e
capa) limpando nomes como `Jogo.bin?md5=…&expires=…`, extraindo zips e levando a capa ao lado. Não altera a origem.

## Compilar

```
make test      # go vet + testes do lançador
make all       # launcher/launcher-arm, x39jail (ARM), x39jail-host (PC) e tools/diagnostico/evlog/evlog-arm
```

Os binários não são versionados (veja `.gitignore`). O RetroArch e os cores se compilam com `build/build-retroarch.sh`
e `build/build-cores.sh`, que precisam da toolchain Linaro (`X39_TOOLCHAIN` ou `toolchain/`, ver `build/env.sh`).

**Atenção com a jaula:** um `x39jail` recompilado tem hash diferente do binário que foi testado (mesmo com o código
idêntico). **Rode `./test-nand.sh` com o binário novo antes de usá-lo no console**; só vale o resultado do binário
que de fato vai rodar.

## Testar sem instalar nada (feito em 24/10/2026)

`instalacao/teste-sem-instalar.sh` usa só a RAM do console (`/tmp/x39`), por ADB. Nada é gravado na memória interna
e um reinício devolve o menu original.

| Comando | O que faz |
|---|---|
| `enviar` | envia o lançador e o inicia em modo demonstração, desligado do ADB (ignora `SIGHUP`) |
| `suave` | **NÃO USE** (reinicia o aparelho): congelava a interface do menu original com `SIGSTOP`; veja o aviso abaixo |
| `jogo <roms…>` / `entrar` | monta um "cartão" em RAM (`/tmp/x39card`) com os ROMs e os cores e abre o lançador de verdade (`-grab`), **sem enviar sinais ao firmware** |
| `ver` | lê o registro de batimentos, o log do lançador e os processos |
| `limpar` | apaga `/tmp/x39` da RAM |

`SEG=<segundos>` define a duração (padrão 120). Resultados medidos no X39:

- Com `enviar`, o lançador **sobrevive à retirada dos dois cabos USB**, mas o menu original (que segue rodando)
  se redesenha por cima ao tirar o cabo.
**AVISO: não congele nem encerre processos do firmware.** O `manager` supervisiona os filhos (`launcher`,
`msg_server`, `audio_service`) e, se um deles sai de forma anormal ou muda de estado, **reinicia o sistema**
(`... exited abnormally, so reboot system`, nos textos de `/usr/bin/manager`). Congelar o `launcher` com `SIGSTOP`
e descongelá-lo/encerrá-lo gerou três reinícios/desligamentos espontâneos em 24/10/2026 (um deles ficou na tela de
carga). O kernel liga um watchdog de hardware de 60 s em *hard mode* (`owl_wdt`), alimentado pelo `manager`.
O lançador agora assume só o que é dele: o uso exclusivo das teclas (`-grab`) e a sua própria página de vídeo,
reafirmada 4 vezes por segundo; **nenhum sinal é enviado a processos do firmware**.

- Com `suave` (histórico), depois de tirar os cabos a tela continua sendo **a nossa** e os botões respondem; ao fim, a interface
  original volta sozinha (aparece a tela de transferência por ~2 s e depois o menu). O `manager` e o `msg_server`
  continuam vivos (watchdog, USB e ADB não são afetados). `suave.sh` retoma a interface original ao terminar, em
  `INT`/`TERM`/saída e, por uma rede de segurança independente, depois de N+30 s.
- Os **dois cabos USB são necessários** para o ADB neste console (USB-C sozinho não basta).
- Se tudo falhar: segurar o botão de ligar reinicia o console.

`tools/diagnostico/evlog` (`make evlog`) imprime, com instante e nome do dispositivo, os eventos de entrada. Mapa medido:
`atc2603c-adckeypad` (event1) = joystick como eixos (`±128` ao empurrar, `0` ao soltar) **e** a cruz direcional como
teclas `103/105/106/108`; `atc260x_onoff` (event0) = botão de ligar.

## Primeira instalação (a ÚNICA gravação na memória interna)

**Risco real:** com `/var/upgrade/test.sh` presente, o `rcS` **não** inicia o `manager` sozinho. Se esse arquivo falhar
cedo (não executar, erro de sintaxe, travar), o console liga **sem interface** (soft-brick; o hardware, o bootloader e
o sistema não são tocados). Por isso o arquivo da memória interna é **mínimo (24 linhas)** e quase toda a lógica fica
no cartão (`x39/gancho.sh`):

- A primeira ação do arquivo mínimo arma um **temporizador de segurança**: se em 75 s o lançador não criar `/tmp/x39-vivo`
  (o que ele faz ao desenhar a primeira tela), o menu original é iniciado de qualquer jeito.
- Só assume o controle se o cartão tiver `x39/ATIVO` e `x39/gancho.sh`; em qualquer falha (inclusive `gancho.sh`
  com erro de sintaxe, travado ou ausente), inicia o menu original.
- `gancho.sh` (cartão): proteção `INICIANDO` contra reinício em laço, cópia do programa para a RAM, ADB, lançador.

**Instalação em etapas** (cada uma com OK explícito, com você presente):
0. `gancho-boot/ensaio-gancho.sh` (fora do repositório): ensaia a lógica no console, só em RAM (6 cenários), sem instalar nada.
1. `gancho-boot/prova-execucao.sh` (fora do repositório): grava um arquivo inofensivo com OUTRO nome em `/var/upgrade`, executa-o pelo caminho
   completo (como o `rcS`) e o apaga; prova que o firmware executa scripts dessa partição, sem arriscar o boot.
2. `gancho-boot/instala-gancho.sh` (fora do repositório): valida a sintaxe com o interpretador do console, recusa sobrescrever um arquivo
   existente, grava `/var/upgrade/test.sh` e confere o hash. **Primeiro boot com o cartão ORIGINAL** (sem `x39/ATIVO`):
   o menu original deve subir normalmente (prova o fluxo de boot pelo botão de ligar).
3. Boot com o cartão do lançador (`x39/ATIVO`), com você presente.

### Recuperação, da mais simples à mais forte
1. Tirar o cartão e reiniciar (sem `x39/ATIVO` o menu original inicia).
2. Apagar `x39/ATIVO` do cartão.
3. No lançador: Menu > "Menu original do console".
4. `gancho-boot/remove-gancho.sh` (fora do repositório) (ou `adb shell rm /var/upgrade/test.sh`). **Atenção:** o ADB de hoje existe porque
   o firmware está no modo de transferência USB: quem reage ao cabo é o `msg_server` (filho do `manager`), que roda
   `usb.sh ADD_FUNCTIONS mass_adb`. Num boot com o lançador **não há `msg_server`**, então o gancho tenta repetir o
   que o firmware faz (`/etc/usbmond.sh` carrega o gadget USB e inicia o `adbd`), mas isso **não foi confirmado num
   boot real**. Se o ADB não aparecer, use as rotas 1 a 3 (não dependem de ADB): com o firmware original o console
   volta ao modo de transferência e o `remove-gancho.sh` (fora do repositório) funciona.
5. Proteção contra reinício em laço: o gancho cria `x39/INICIANDO` e o lançador o apaga ao desenhar a primeira
   tela; se o boot anterior não chegou lá, o seguinte usa o menu original.

**Como enxergar o que aconteceu num boot sem ADB:** o gancho grava o log no cartão (`x39/boot.log`, limitado a
100 KB). Tire o cartão e leia no PC, ou entre no modo de transferência do firmware ("Menu original do console")
e leia o arquivo por lá.

**Copiar ROMs:** com um leitor de cartão no PC, ou pelo modo de transferência do firmware (Menu > "Menu original
do console"; o cartão aparece como disco no PC; depois reinicie para voltar ao lançador).

**Watchdog:** o `manager` do firmware alimenta o watchdog de hardware. Quando o lançador assume (`-watchdog`),
ele mesmo o alimenta (a cada 5 s, prazo de 30 s) e o desarma de forma limpa antes de devolver o controle.
Isso só foi testado em lógica, **não num boot real**: o primeiro boot com o gancho deve ser acompanhado.

## O lançador (`launcher/`)

Carrossel de sistemas (que dá a volta) e lista de jogos; **Recentes** e **Favoritos** (Y marca ★); L1/R1 pulam
7 jogos, L2/R2 pulam de letra; Opções (Menu): brilho, volume dos jogos (−40 a 0 dB), informações, reiniciar,
desligar, menu original. Bateria na barra superior; a luz da tela apaga após 90 s sem botões; botão de ligar:
toque apaga/acende a tela, segurar 1,5 s desliga. Nomes de ROM sem `(USA)`, `[!]`. Estado em `x39/data/launcher.json`.
Interface pré-renderizada em RGB565 (fundos e sprites em cache): **~14 ms por quadro** no X39 (a primeira versão
levava 250 ms). Pré-visualização no PC: `cd launcher && go run . -preview /tmp/prev`.

## O que garante que a NAND não é tocada

`x39jail` (`jail/main.go`) monta em RAM uma raiz própria com um `/dev` só com os nós necessários (vídeo, entrada, som,
terminal; **nunca** `nand*`, `mmcblk*` nem `mem`), usa chroot, remove todas as capabilities, vira o usuário 1000 e
liga `no_new_privs`. `./test-nand.sh` roda as verificações dentro da jaula, nos dois modos (`test` e `test -dev`):
abrir `/dev/nand0*`, `mmcblk0` e `mem` falha, `mknod` falha, `mount` falha, não há como voltar a root.
Resultado em 23/10/2026 com o binário final (SHA-256 `72176ede…`): **todas passaram** e nenhum mount restou.
Refaça o teste sempre que recompilar o `x39jail`.
Limites honestos: proteção forte contra bug ou código descuidado; não é absoluta contra um ataque deliberado que
explore o kernel (3.10, antigo). Não há rede: o RetroArch e os cores foram compilados sem rede e nenhum importa
símbolos de rede (`gambatte` foi recompilado com `HAVE_NETWORK=0`).

## Descobertas sobre o hardware

- A CPU do ATM7051 é um **Cortex-A5** (CPU part 0xc05, em ordem), não um A9; memória ~200 MB/s. Compilar com
  `-mcpu=cortex-a5 -mfpu=neon-vfpv4`; **nunca** `-march=armv7ve` (o A5 não tem `sdiv/udiv`).
- **Calibração dos cores (24/10/2026, com `corebench` e `corewrap`; 600 quadros depois de 120 de aquecimento, jogo
  em abertura, 900 MHz).** "CPU a 60 Hz" = quanto de um núcleo o core usa para rodar em tempo real; "no RetroArch" =
  `retro_run` por segundo medido dentro do RetroArch (esperado ~60). Os números antigos (fceumm 86%, genesis_plus_gx MD
  65%...) vinham do medidor de áudio quebrado e **não valem**.

  | Core | Jogo | Coerência* | Sem freio | CPU a 60 Hz | No RetroArch |
  |---|---|---|---|---|---|
  | `quicknes` (NES) | Air Contra | 0,9986 | 564 q/s | **11%** | 60 |
  | `fceumm` (NES) | Air Contra | 1,0000 | 103 q/s | 58% | 60 |
  | `picodrive` (Mega Drive) | Asterix | 1,0000 | 346 q/s | **17%** | 60 |
  | `genesis_plus_gx` (Mega Drive) | Asterix | 1,0000 | 88 q/s | 68% | 60 |
  | `picodrive` (Master System) | Mônica | 1,0000 | 399 q/s | **15%** | 60 |
  | `genesis_plus_gx` (Master System) | Mônica | 1,0000 | 124 q/s | 48% | 60 |
  | `stella2014` (Atari 2600) | Enduro | 0,9999 | 146 q/s | 41% | 60 |
  | `gambatte` (GB/GBC) | Tetris DX | 1,0008 | 429 q/s | 14% | 60 |
  | `gpsp` (GBA) | Pokémon Blue Ocean (16 MB) | 1,0000 | 273 q/s | 22% | 60 |
  | `snes9x2005` (SNES) | Donkey Kong Country | 1,0000 | 123 q/s | 49% | 60 |

  \* amostras de áudio por quadro ÷ (taxa ÷ fps) do core; 1,0000 = o core mantém o próprio tempo.
  Todos seguram 60 `retro_run`/s no RetroArch com 0 a 2 quadros descartados em 15 s. Padrão recomendado: `picodrive`
  para Master System/Game Gear/Mega Drive e `quicknes` para NES (os mais leves: menos CPU e bateria);
  `genesis_plus_gx` e `fceumm` ficam como alternativas mais exatas. O Mega Drive no `genesis_plus_gx` tem a menor
  folga (1,5×): cenas pesadas podem derrubá-lo. São medidas de **abertura de jogo**; o jogo em cena pesada custa mais.
- Luz da tela `/sys/class/backlight/owl_backlight` (máx. 900), bateria `/sys/class/power_supply/battery`,
  `/sbin/poweroff` e `/sbin/reboot` (BusyBox). Teclado: ver `launcher/input.go` (códigos evdev medidos).
- O vídeo usa o driver genérico `sdl` (framebuffer por software). O `sdl_powkiddy` do autor exige `/dev/mem`,
  que daria acesso a toda a memória física e derrubaria o isolamento.

## Como foi construído

Backup completo da NAND feito antes de qualquer teste, fora deste repositório (MD5 conferidos). O SuperX CFW (scooper540) foi lido por
inteiro: não é malicioso, mas não usamos os binários prontos; compilamos tudo do código-fonte. Toolchain: Linaro
gcc 7.5.0-2019.12 (tarball SHA-256 `abf877f0…`), runtime glibc 2.25 em `pacote/rt/lib`.
Patches (em `build/patches/`), aplicados só nas cópias de build: `sdl-sem-prints-de-depuracao.diff` (o SDL do autor
imprimia a cada tecla), `retroarch-tinyalsa-mmap-start.diff` (o driver MMAP do fork nunca iniciava o PCM; prepara,
inicia, recupera de XRUN e sincroniza o `appl_ptr`), `retroarch-barra-de-volume.diff` (barra de volume sem fonte,
faixa −40..0 dB). `build/Makefile.x39.retroarch` é uma cópia minimal do `Makefile.powkiddy`.
A fonte do lançador é DejaVu (licença livre), rasterizada em atlas por `launcher/tools/fontatlas` (só no PC).

| Componente | Commit |
|---|---|
| RetroArch 1.14.0 (scooper540/RetroArch-powkiddy) | `e3df6bc` |
| SDL 1.2 (scooper540/SDL-1.2-powkiddy) | `2d50f5d` |
| picodrive / stella2014 / prosystem / handy | `1890c29` / `7d1361e` / `8a88014` / `bc55d46` |
| atari800 / mednafen_pce_fast / wswan / ngp | `4e7fbc7` / `4eee99c` / `4b01295` / `a50d5ac` |
| bluemsx / freeintv | `e3086eb` / `ef3e0fe` |
| quicknes / fceumm / genesis_plus_gx | `26bb785` / `ac0d00a` / `60bee4c` |
| gambatte / snes9x2005 / gpsp | `7809bb6` / `a79dfe9` / `5819380` |

Para conferir um cartão: `cd <cartão>/x39 && sha256sum -c MANIFESTO.sha256` (todas as linhas devem dar OK).

## Armadilhas já descobertas

- **Go + `/dev/input`:** abrir esses dispositivos com `os.OpenFile` (epoll do runtime) derruba a sessão ADB. Use
  `syscall.Open`/`syscall.Read`.
- **Direcional e joystick são fontes diferentes** (teclas 103/105/106/108 e eixos). O tradutor de eixos
  (`emitAxis`) guarda o estado já classificado (-1/0/1); aplicar `sign()` de novo sobre ele apagava o estado e
  o "soltar" nunca saía (carrossel/lista girando sozinhos). Há teste em `launcher/input_test.go`.
- **O áudio do RetroArch era entregue ao aparelho pela METADE (jogos a 2× a velocidade).** O driver `tinyalsa`
  do fork detectava "S16" por `size % 4 != 0`, o que nunca era verdade (S16 estéreo tem 4 bytes por quadro), então
  tratava S16 como S32 e gravava metade dos quadros: o áudio do aparelho (48 kHz) segurava o laço a **120
  `retro_run`/s em vez de 60**, com o som a 2×. Corrigido em `build/patches/retroarch-tinyalsa-mmap-start.diff`
  (sempre converte S16→S32; buffer de conversão de 256 KB). **Como se mede** (não use `--max-frames` nem os
  contadores de vídeo do RetroArch, que enganam): `tools/diagnostico/corewrap` é um core-embrulho que conta os `retro_run` por
  segundo dentro do RetroArch (`X39_REALCORE=/app/cores/<core> run-game.sh corewrap_libretro.so <jogo>`; o esperado é
  o fps do core, ~60) e `tools/diagnostico/corebench` mede a coerência do core sem o RetroArch (amostras de áudio por quadro =
  taxa/fps) e o custo de CPU. Medidas de 24/10/2026: picodrive SMS 15% de um núcleo a 60 Hz, genesis_plus_gx SMS
  48%, stella 41%; coerência 1,0000 nos três.
- **Tela preta depois do jogo (causa achada em 24/10/2026):** o RetroArch/SDL deixa o hardware de vídeo apontando para
  outra página, mas o driver continua com o `yoffset` antigo. Mandar exibir a página X com X igual ao `yoffset` guardado
  é **ignorado** pelo driver (nenhuma escrita no hardware) e a tela fica preta, mesmo com a luz, o painel
  (`/sys/class/owl_panel/lcd`: `enable=1`) e o framebuffer corretos. Prova: com o lançador parado, pintar a página 2 de
  vermelho e exibi-la **mostra vermelho** (o hardware obedece ao `pan`), e voltar à página 1 **revive** a tela.
  Correção: `OpenFramebuffer` escolhe como página própria sempre uma **diferente** da que o `yoffset` indica no
  momento (`launcher/fb.go`); o lançador reabre o framebuffer depois de cada jogo, então o primeiro pedido já é uma
  mudança real. Não adiantam: `FBIOBLANK`, `force_refresh`, `KD_GRAPHICS`, nem exibir de novo a mesma página.
  Ferramentas de diagnóstico: `tools/diagnostico/fbwake` (`paint N RRGGBB` pinta e exibe uma página) e `tools/diagnostico/fbinfo`.
- **O SDL do fork joga entradas no joystick 0 (jogador 1) — por isso o controle USB comandava o jogador 1** (achado em
  25/10/2026 com `tools/diagnostico/sdljoy`, que lista os joysticks do SDL e o roteamento de cada botão/eixo). Dois pontos do código do
  autor (`src/joystick/linux/SDL_sysjoystick.c`): (1) no caminho **evdev** (`EV_HandleEvents`) TODO evento de TODO `event*` vai
  para o joystick 0 ("always fire the events to device 0"); (2) no caminho **joydev** (`JS_HandleEvents`) os BOTÕES ficam no
  próprio joystick, mas os EIXOS e o hat também iam para o joystick 0. Correção: a jaula recebe SÓ o `js` do controle (sem o
  `event*` dele) e o patch `build/patches/sdl-eixos-no-joystick-proprio.diff` (aplicar depois de `sdl-sem-prints-de-depuracao.diff`)
  entrega eixos e hat de um `js` ao próprio joystick. A ordem dos índices do SDL na jaula é `event0, event1, js0, js1`:
  console = joystick 0 (evdev, fundido) e js0; **controle USB = joystick 3** (`padJoypadIndex` em `launcher/launch.go`).
- **Analógico do console no RetroArch (1/11/2026):** o analógico do console chega ao SDL pelo nó `js0` (`atc2603c-adckeypad`), não
  pelo evdev. Depois do patch dos eixos ele caía no joystick 2 e o jogador 1 ficava sem analógico. O patch agora mantém no joystick 0
  os eixos desse `js` (nome `atc2603c-adckeypad`) e entrega os de qualquer outro `js` ao próprio joystick. No `retroarch.cfg` o
  jogador 1 também tem o "+" como botões 14–17 (cima, esquerda, direita, baixo) e `analog_dpad_mode = "1"`. Testado no Combat: console =
  tanque 1 (analógico e "+"), controle USB = tanque 2.
- **O firmware troca de página de vídeo sem atualizar o `yoffset`** do framebuffer: desenhar na página que o
  `FBIOGET_VSCREENINFO` indica pode ir para uma página que não está na tela (a nossa interface ficava invisível,
  com a do firmware por cima). O lançador desenha numa página própria e manda exibi-la (`FBIOPAN_DISPLAY`).
- **O botão de ligar e o teclado são lidos pelo `msg_server` do firmware (`event0`/`event1`), não pelo `launcher`.**
  Congelar o `launcher` (modo `suave`) NÃO impede o `msg_server` de reagir ao botão de ligar: nos testes de 24/10/2026
  o console desligou/reiniciou duas vezes depois de toques no botão de ligar. O lançador deve rodar com `-grab`
  (`EVIOCGRAB` em `event0` e `event1`; solta só o `event1` durante o jogo, que o RetroArch lê por `js0`). Até a
  confirmação num teste, **não aperte o botão de ligar** durante os testes sem `-grab`.
- **Saída do RetroArch:** ele pode deixar a luz da tela apagada (`bl_power=1`) e o lançador não percebe; ao voltar de
  um jogo é preciso religar a luz e redesenhar a tela, e esperar/descartar botões que o kernel ainda entrega.
- **Sem `setsid`/`nohup` no console:** para desligar um processo do ADB use `trap '' HUP` e `&`.
- **`adb shell` não devolve o código de saída remoto:** decida pela saída (`echo SIM/NAO`).
- **Mixer:** sem `tinymix 30=1, 35=1` o som fica mudo; o estado volta ao normal no fim de cada jogo.
- **RAM:** o `/tmp` do console tem ~43 MB; envie só o core que vai usar. Em modo de transferência USB o console
  não monta o cartão.
- **`make` dos cores:** variáveis passadas na linha de comando (`CFLAGS`, `CC`) sobrescrevem as dos makefiles dos cores (perdem os `-D` internos); passe-as pelo ambiente.

## Pendências

- **Antes de instalar qualquer gancho na memória interna:** esclarecer como entrar no recovery/ADFU (ver
  `docs/boot-e-recuperacao.md`). O dono do aparelho pediu cautela: houve vários erros nossos nos testes.
- **Tela original depois do lançador:** o firmware não se redesenha sozinho; hoje só tirar os dois cabos do PC o faz voltar.

- Etapa seguinte à `suave`: lançar jogo de verdade pelo lançador depois de tirar os cabos, e tratar o **cartão SD**
  (com o USB ligado o cartão é exportado ao PC; o firmware só o monta em `/mnt/card` depois de tirar o cabo).
- `bin/tinymix` (binário ARM estático do tinyalsa usado para ligar o mixer) **não está versionado e a origem dele
  não foi documentada**: documentar como compilá-lo do código-fonte (ou adicionar um alvo no `Makefile`).
- `pacote/app/retroarch`, `pacote/app/cores/*.so` e `pacote/rt/lib` também são artefatos de build (veja `build/`).

- Primeiro boot real com o gancho (acompanhado) e teste do watchdog.
- ROMs de Master System e Atari para o teste que importa; BIOS dos sistemas que exigem.
- Atalho para sair de um jogo e botão de ligar durante o jogo (hoje: o do RetroArch; teste o seu).

## Organização dos arquivos
- `launcher/`, `jail/`, `cartao-base/`, `pacote/`, `build/`, `instalacao/`: o produto (código, cartão-base, scripts de envio e de compilação).
- `tools/diagnostico/`: ferramentas só de medição/depuração (`corebench`, `corewrap`, `sdljoy`, `evlog`, `fbwake`, `fbinfo`, `fbcount`).
- `tools/importa-roms.sh`: copia coleções de ROMs para o cartão.
- Os scripts que **gravam o gancho de boot na memória interna** (`test.sh`, `instala-gancho.sh`, `remove-gancho.sh`,
  `ensaio-gancho.sh`, `prova-execucao.sh`) foram tirados do repositório de propósito: não se instala nada na NAND antes de existir
  um caminho de recuperação testado (ver `docs/boot-e-recuperacao.md`). `instalacao/gancho.sh` fica porque só vai para o cartão e
  não faz nada sem o `test.sh` instalado.
