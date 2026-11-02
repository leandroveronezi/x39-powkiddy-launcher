# Estado do projeto e próximos passos (atualizado em 02/11/2026)

Leia primeiro o `README.md` (visão geral) e o `CLAUDE.md` (regras de trabalho). Este arquivo diz onde paramos.

## O que está pronto e foi testado no console
- **Lançador em Go** (`launcher/`): menu por sistemas, capas, tela do console, controles, jogo pelo RetroArch dentro da jaula.
  Testado em jogo real (Atari 2600, Combat) pelo fluxo completo, sem nada gravado na NAND.
- **Jaula `x39jail`** (`jail/`): isola o RetroArch (sem rede, só os caminhos permitidos).
- **Dois jogadores:** o console é o jogador 1 (analógico e "+") e um controle USB na USB-A 2 é o jogador 2.
  Dependeu de um patch no SDL do fork (`build/patches/sdl-eixos-no-joystick-proprio.diff`) e do `pacote/app/retroarch.cfg`.
- **Fone de ouvido** desliga o alto-falante; áudio corrigido (patch tinyalsa) e velocidade calibrada para os cores testados.
- **Sistemas, cores, capas e controles vêm do cartão** (`cartao-base/`), com 20 sistemas descritos por JSON.
- Segurança: nada é gravado na memória interna. O gancho de boot **não está instalado e seus scripts saíram do
  repositório de propósito** (ver `README.md`, "Organização dos arquivos").

## Como retomar os testes no console
1. Ligue o console com **os dois cabos USB** (USB-C e USB-A 1). Controle USB opcional na USB-A 2.
2. `export ADB=/caminho/do/adb` (o padrão dos scripts é `adb` no PATH). O serial do console é `0123456789ABCDEF`.
3. Compile: `make all` (precisa do Go) e os binários de `build/` (toolchain ARM, veja `build/env.sh`).
4. `instalacao/teste-sem-instalar.sh jogo <rom>` envia tudo para a RAM; depois `... entrar` abre o lançador.
   Nunca mexer com um jogo aberto (o script se recusa).
- Regras: pedir OK antes de rodar algo no console; não gravar na NAND; não mandar sinais a processos do firmware.

## Próximos passos, por ordem
1. **Cartão de teste:** montar um cartão com `instalacao/monta-cartao.sh` e usar ROMs/capas de verdade. Se não houver
   leitor de cartão no PC, dá para enviar a estrutura direto para o cartão que estiver dentro do console, via
   `adb push` para `/mnt/card` (só com um cartão de teste, nunca o original, e depois de **ler** o que há nele).
2. **Calibrar os cores novos** com ROMs reais (velocidade e áudio, como em "Calibração dos cores" no README):
   7800, 5200, Lynx, PC Engine, WonderSwan, Neo Geo Pocket, MSX, ColecoVision, Intellivision, Game & Watch,
   Odyssey2, Channel F, Pokémon Mini, ZX Spectrum, Amstrad, Doom (precisa de WAD/BIOS onde aplicável).
   Ferramentas em `tools/diagnostico/` (`corebench`, `corewrap`, `sdljoy`, `evlog`).
3. Medir **bateria** em jogo e testar **HDMI** (`fb1`, `/sys/class/switch/hdmi`).
4. **Recuperação segura antes de qualquer boot automático:** ainda não há caminho de recuperação testado ponta a ponta.
   Provado: o modo ADFU entra (aparelho desligado, D-pad para baixo e os dois USB, aparece `10d6:10d6`).
   Falta um PC/VM com Windows e a ferramenta de gravação para um teste real. Só então considerar o gancho (`docs/boot-e-recuperacao.md`).
5. Menor: a tela original do firmware só se redesenha depois de tirar os dois cabos USB.

## Lições de processo (valem para a próxima sessão)
- **Medir antes de supor.** Os maiores avanços vieram de ferramentas de medição (`sdljoy`, leitura direta do `event1`),
  não de teorias. Faça uma pergunta simples e um teste de cada vez.
- **Avise o usuário antes de iniciar um teste que exige ação dele** (apertar botões, mexer em cabos), e rode em primeiro
  plano. O `adbd` do aparelho mata processos em segundo plano quando a sessão fecha.
- O `adbd` antigo não devolve código de saída; não confie em `$?` de comandos remotos.
- RAM: manter `/tmp` abaixo de ~70%; acima disso a jaula falhou em silêncio.
- Respostas curtas e em português do Brasil, sem tratar o usuário como leigo.
