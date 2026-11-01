# Como o X39 inicia, como se atualiza e como se recupera (análise de 25/10/2026)

Fonte: o **backup** da memória interna (`nand0p1…p7.img`, MD5 conferido) aberto no PC (nada foi tocado no aparelho),
o repositório `scooper540/powkiddy-X39-X45-X51-X70-cfw` (SuperX) e `FoxExe/PowKiddy_fw` (ferramentas de OTA).

## Partições (X39, ATM7051, kernel 3.10)

| # | Nome | Tipo | Conteúdo |
|---|---|---|---|
| (fora das partições) | U-Boot | RAW | bootloader. **Não está no nosso backup** e nunca foi tocado |
| p1 | MISC (boot normal) | FAT16 | `uImage`, `kernel.dtb`, `ramdisk.img`, `uenv.txt` (`root=/dev/nand0p4 rootfstype=squashfs init=/bin/init`) |
| p2 | RECOVERY | FAT16 | **outro** `uImage` + `kernel.dtb` + `ota_initrafms.img` (sistema mínimo que faz a atualização) |
| p3 | CONFIG | RAW | `config.bin` |
| p4 | SYSTEM | squashfs (só leitura) | o sistema: `manager`, `launcher`, `msg_server`, `upgtool`, `usb.sh`… |
| p5 | UPGRADE | FAT32 | montada em `/var/upgrade` (aqui fica o gancho `test.sh`) |
| p6 | RES | squashfs | recursos |
| p7 | UDISK | FAT32 | `/mnt/udisk`, o "disco interno" que o PC enxerga |

## Boot normal (`/etc/init.d/rcS`, em `system`)
Monta `/var/upgrade` (p5), `/usr/local`, `/mnt/udisk` (p7) — **com `dosfsck -yf` e, se o mount falhar, `mkfs.vfat`**
(formata a partição!) — carrega a GPU e roda `upgtool -o`. Depois:

```
se /tmp/dumplog_dbg_mode  -> manager + klogd + syslogd -O /mnt/udisk/messages.log   (modo de diagnóstico, ligado por `upgtool -O`)
senão se /var/upgrade/test.sh -> executa /var/upgrade/test.sh   (em primeiro plano, NO LUGAR do manager)
senão se /mnt/udisk/test.sh   -> executa /mnt/udisk/test.sh
senão                        -> manager &
```
`/var/upgrade` entra no `PATH` e no `LD_LIBRARY_PATH` (um arquivo `manager` ali substituiria o de fábrica).
**Não existe autorun por cartão SD nem por pen-drive no boot.** O cartão só é tratado depois, pelo frontend.

## Atualização oficial (OTA) e recuperação
- O frontend detecta `update.zip` na raiz do cartão e oferece atualizar; ele chama `upgtool -l <arquivo>`, que grava um
  sinalizador (`miscinfo`) e reinicia **no recovery (p2)**.
- O recovery (`ota_initrafms.img`) monta a `UPGRADE` em `/upgrade` e o **cartão (`/dev/mmcblk0p1`) como `/udisk`**;
  se houver `/udisk/update.zip` roda `upg /udisk` (sem ele tenta o servidor OTA da Actions pelo WiFi). Confere o CRC de
  cada item, **grava as partições**, limpa `/proc/ota_flag` e reinicia. **O recovery NÃO executa `test.sh` nem o `manager`.**
- Formato do `update.zip`: cabeçalho `PATO…` de 2048 bytes (CRC no fim) + itens de 64 bytes (nome, flag RAW/IMG, nº da
  partição, tamanho e offset em blocos de 512, CRC32) + imagens: `uboot.bin` (RAW), `misc.img` (p1), `recovery.img` (p2),
  `system.img` (p4), `res.img` (p6). Ferramentas: `FoxExe/PowKiddy_fw` (`OTA_Unpack.py`, `OTA_Create.py`).

## Como o SuperX (scooper540) instala
Pelo mesmo caminho oficial: um `update.zip` **personalizado** (reescreve o `system.img`) que grava um gancho permanente;
depois, o gancho executa o `run.sh` da raiz do cartão a cada boot ("desinstalar" = tirar o `run.sh`). O sistema dele fica
numa imagem `rootfs.ext3` de 50 MB dentro do cartão, montada por loop e com `chroot`. Ou seja: **o desenho é o mesmo do
nosso `test.sh`, só que instalado regravando o `system`**, que é mais invasivo do que um arquivo FAT em `/var/upgrade`.

## O que o kernel diz sobre reinício, recovery e ADFU (lido no `uImage` do backup, 25/10/2026)
O `uImage` é um zImage com payload XZ (descompacta com `lzma.LZMADecompressor`; a primeira "marca XZ" é falso positivo).
- Há **registradores persistentes no PMIC (`ATC260X`)**, lidos pelo bootloader: `REBOOT_ADFU`, `REBOOT_RECOVERY`, `FW_S2`,
  `BOOT_FROM_S2`, `SHDWN_NOT_DEEP`… Ou seja, existem pedidos de reinício em **ADFU** e em **recovery** por software.
- O comando de reinício reconhece `adfu`, `upgrade_reboot`, `upgrade_halt`, `bootloader`, `fastboot`; a interface é
  `/sys/power/wakeups/reboot` (o recovery faz `echo fastboot > /sys/power/wakeups/reboot; reboot -f` ao terminar).
- Modos de boot do kernel: `normal`, `charger`, `recovery`, `upgrade` (`/proc/bootmode`, hoje `normal`).
- `/proc/ota_flag` (hoje `0`; o recovery grava `0` ao terminar). A área `miscinfo` deste aparelho **não tem cabeçalho**
  (`read MiscInfoHeader failed`, `read OTA_FLAG failed`): não há sinalizadores gravados.
- **NÃO testado** (mudam o boot): `echo adfu > /sys/power/wakeups/reboot` e `echo 1 > /proc/ota_flag`. Entrar no recovery
  sem `update.zip` faz o `upg` tentar a rede (OTA da Actions) e, se falhar, o script **não limpa** o `ota_flag` e
  reinicia: possível laço no recovery. Só testar com um pacote válido e um plano de saída.
- O modo ADFU em si fica na ROM do chip (sempre presente), mas gravar exige a ferramenta de produção da Actions (Windows)
  **e uma imagem de firmware compatível** (que inclui o U-Boot). **Não temos essa imagem**: o backup só tem p1…p7.
  Não achamos firmware oficial do X39 online; o SuperX tem um `update.zip` "genérico" (Discord do projeto).

## A rede de segurança que existe de fato (tópico do Reddit, 25/10/2026) — a CONFIRMAR antes de depender dela
Fonte: r/SBCGaming "Fix black screen on powkiddy x39 pro…" (autor Fihnix; comentários do próprio `scooper540`, autor do
SuperX). É conteúdo da web: serve de roteiro, **cada passo precisa ser verificado no nosso aparelho**.
- **ADFU (gravação de emergência por USB), receita do `scooper540`:** desligar; **USB-C no carregador** e a **USB1 (a
  porta em modo dispositivo, a nossa USB-A 1)** no PC; **segurar o botão "para baixo" e ligar**; a tela fica preta; o PC
  detecta um dispositivo Actions; na **Actions PAD Product Tool** grava-se o arquivo `.FW`. Bate com o que medimos
  (`REBOOT_ADFU` no kernel; A1 = controlador `b0240000.usb` em modo dispositivo). Dica deles: ligar o USB-C no carregador
  antes de gravar (bateria fraca faz a gravação falhar).
- **Arquivos** (pasta pública do Google Drive citada no tópico, id `1RSTNfCleuaTiV6rl9MqCgw4F6b-DxiEP`): subpastas
  **"Actions Pad Product Tool"** (`Actions_Pad_Product_Tool_v1.09.02.zip`, `Actions_Product_Tool_Driver_v2.0.0.3.zip`; Windows),
  **"X39 软件"** (firmware do X39 comum, o nosso) e **"X39 Pro 软件"** (outro modelo: NÃO usar no X39 comum).
  Pedir também ao suporte oficial (powkiddy.com, chat) é como o autor obteve os arquivos.
- **Atualização por cartão** (guia do espelho `eladshu/powkiddy-cfw-firmware…`, `update.zip` oficial da Powkiddy): copiar
  `update.zip` para a RAIZ do cartão, inserir, ligar e esperar; **ligar o USB-C no carregador** antes. Depois de uma
  atualização ruim: ligar, esperar ~3 min e responder à pergunta na tela (direita + A = "sim").
- **Avisos reais do tópico:** quem gravou o firmware de OUTRO modelo (X51/X39 Pro) ficou com tela preta/laço; alguns
  voltaram com a PAD Tool + o `.fw` certo, outros não; `update.zip` pode falhar no meio ("40%, partição recovery");
  conferir o CRC do `update.zip` copiado.
- **Ainda não temos o `.fw` do X39 comum nem uma máquina Windows com a ferramenta.** FoxExe: gerar `.fw` a partir de
  `update.zip` não está resolvido.

## O `update.zip` que o dono baixou (baixado em 25/10/2026) — **NÃO é do nosso modelo**
Pacote OTA válido (cabeçalho `PATO`, versão `2.0.00.230822.1449`, 5 itens, todos os CRCs conferem): `uboot.bin` (RAW, 657 920 B),
`misc.img`, `recovery.img`, `system.img` (26,9 MB), `res.img`. **O logo de boot é 480×854 (o do nosso X39 é 480×272)**:
é firmware do **X39 Pro**. Aplicá-lo no X39 comum causa tela preta/laço (relatos no Reddit). **Nunca copiar para a raiz do
cartão do aparelho** (o recovery o aplica sozinho). Serve só de referência (o kernel é mais novo: `3.10.52 #1 … Aug 22 2023`;
mesmo `uenv.txt`; `kernel.dtb`, `ramdisk.img`, `uImage` e logo diferentes dos nossos).
**U-Boot** (`U-Boot 2015.07 for ls360f_ats360`): cadeias `check_key: keycode=%x` e **`touch v+ key, enter adfu`** (a tecla
**V+ = volume para cima** na partida leva ao ADFU; o Reddit dizia "botão para baixo" — conferir os dois), `reset_to_adfu`,
`PMU ota flag founded!`, `bootloader flag founded!`, `update reboot, not check update package`, `ota load recovery err`,
`power is low, shutdown the machine!`, `androidboot.mode=charger` (com VBUS e sem tecla, o U-Boot sobe o modo de carga:
por isso plugar os cabos com o aparelho desligado mostra a tela de carregamento).

## ADFU CONFIRMADO no nosso aparelho (25/10/2026 06:34)
Receita que funcionou (sem gravar nada): aparelho **totalmente desligado**, **segurar a tecla "para baixo" do direcional** e, com
ela pressionada, **plugar os dois cabos** (USB-A 1 no PC e USB-C no carregador); a tela fica **preta**. O PC enxerga
**`10d6:10d6` (Actions Semiconductor)**, classe específica do fabricante com 2 endpoints bulk, sem strings de produto; o ADB
some. Saída: tirar os cabos, segurar o botão de ligar ~10 s e ligar de novo normalmente (voltou `10d6:0c02` com ADB em 28 s).
Sem a tecla, plugar os cabos com o aparelho desligado só sobe o modo de carga (animação de carregamento).
O "V+" do U-Boot é o nome interno dessa tecla (a de "para baixo"). Falta para usar de verdade: o `.fw` do X39 COMUM e uma
máquina Windows (ou VM com repasse de USB) com a Actions PAD Product Tool + driver.

## FIRMWARE OFICIAL DO NOSSO MODELO ENCONTRADO (25/10/2026)
Pasta do Drive "X39 软件" (`1FeyJGh4qYEVf9tOtFtw5pYXhIC5d4Y_O`) → subpasta `atm7051_game_hdmi_ddr128_480272_024_x39_024V3_LCD0r_V2.0_220905a--红蓝UI 同步`
com **`…_V2.0_220905.fw`** (76,4 MB, contêiner Actions `ACTSCOMPOUNDFW`, 5 itens), **`update.zip`** (61,8 MB, PATO
`2.0.00.220905.2021`; CRC de todos os itens confere; logo 480×272) e `log.txt` (corrige o volume baixo; atualiza a biblioteca
de NAND para `K9HBG08U1M`). O `.fw` embute as mesmas imagens `misc/recovery/system/res` do `update.zip`.
Guardado em uma pasta própria fora do repositório (`firmware-oficial-x39-V2.0-220905/`) (com `SHA256SUMS.txt` e `LEIA-ME.txt`).
**Diferenças para o nosso backup (firmware de jan/2022):** kernel de set/2022; `dtb` com corrente de carga 400→800 mA,
backlight 1000→900 (padrão 750→450), tabela da bateria, um 2º modo de tela 480×272@60 e o `pixel_clock` do 1º modo
alterado, `joystick_type`; `uboot.bin` de 647 680 B (nosso U-Boot atual não tem backup). **O `devicetree` em execução no
aparelho é idêntico ao do backup** (conferido em `/proc/device-tree`). Gravar este `.fw` **atualiza** o aparelho; não devolve o
backup byte a byte.

## O que isso muda para o medo de perder o aparelho
1. O nosso gancho (um arquivo `test.sh` na FAT `p5`) é **a menor alteração possível**; o OTA regrava partições inteiras.
2. Existe um **recovery independente** (p2) que regrava as partições a partir do cartão e **ignora o `test.sh`**.
3. O backup cobre p1…p7, **menos o U-Boot** (que não tocamos).
4. **Pontos em aberto, a esclarecer ANTES de instalar o gancho**:
   - como entrar no recovery à mão (sinalizador em `miscinfo` via `upgtool -s`, e/ou combinação de teclas no U-Boot);
   - se existe o modo de emergência por USB (**ADFU**, ferramenta de produção da Actions; o repositório do FoxExe traz
     as DLLs dela) e qual a combinação de teclas;
   - listar (só leitura) os itens de `miscinfo` com `upgtool -L`;
   - gerar um `update.zip` de restauração a partir do backup (faltaria o `uboot.bin`) e decidir se testá-lo.
5. Sem manager não há ADB (`msg_server` sobe o ADB), então um `test.sh` que falhe deixa o aparelho sem interface e sem ADB:
   por isso o `test.sh` nosso inicia o `manager` **primeiro** e só depois o resto.

## Ponto a revisar: a tela original depois do lançador
Ao sair do lançador (ou de um teste de diagnóstico que pinta páginas do framebuffer) o firmware **não se redesenha
sozinho**: ele só repinta com um evento. Com os cabos do PC ligados a tela fica na última imagem; tirar **os dois cabos**
fez o menu original voltar. Evitar pintar o framebuffer nos diagnósticos e pensar em como devolver a tela ao firmware.
