# Firmware oficial do Powkiddy X39 (para recuperação)

O firmware é um programa do fabricante. O código do repositório não o inclui, mas há um **espelho sem alterações** na release
[firmware-oficial-x39-v2.0-220905](https://github.com/leandroveronezi/x39-powkiddy-launcher/releases/tag/firmware-oficial-x39-v2.0-220905),
guardado só para recuperação, caso o link original saia do ar. Abaixo estão a origem e os hashes para conferir.

## O que serve para o X39 comum (480×272, ATM7051, DDR 128 MB)
Versão 2.0.00.220905.2021 (05/09/2022).

| Arquivo | Tamanho | SHA-256 |
|---|---|---|
| `atm7051_game_hdmi_ddr128_480272_024_x39_024V3_LCD0r_V2.0_220905.fw` | 76.371.840 | `389757018e289f004c0eaf8c68a13c85900c35c2f79f1711056cb5e8df3d11c0` |
| `update.zip` (pacote OTA, formato PATO) | 61.798.912 | `5615ecf98d5dff0cbd40d219500f6f75d8c2efc526a0f6a749cd46cdcaebf3d5` |
| `log.txt` (notas do fabricante) | 117 | `5d0b127c66c5337438a691311a6e76419700bc04ee50b71b28734960524f0bdd` |

- **Origem:** pasta pública do Google Drive "X39 软件" (id `1FeyJGh4qYEVf9tOtFtw5pYXhIC5d4Y_O`, subpasta
  `1SEjNVcCEKnlWGhj433OueEIYFivkd4r5`), citada no tópico do Reddit r/SBCGaming "Fix black screen on powkiddy x39 pro...".
- O `.fw` é um contêiner Actions (`ACTSCOMPOUNDFW`) para a **Actions PAD Product Tool** (Windows) em modo ADFU.
- O `update.zip` é aplicado pelo **recovery**: ele grava as partições sozinho se o arquivo estiver na raiz do cartão.
  **Nunca copie esse arquivo para a raiz do cartão sem querer atualizar.**

## Cuidados
- **Não é uma restauração idêntica**: é mais novo que o firmware de jan/2022 que veio em um aparelho de teste (carga 400→800 mA,
  brilho padrão 750→450, tabela da bateria, um 2º modo de tela, kernel de 2022-09). Para devolver o aparelho exatamente como era,
  vale o backup da NAND feito antes de qualquer teste (guardado fora do repositório, é um dump do aparelho do dono).
- **O update do X39 Pro não serve:** tem logo 480×854, outro modelo.
- Confira sempre os hashes acima antes de usar: `sha256sum -c` com as linhas da tabela.

## Entrar em ADFU (modo de gravação do chip)
Aparelho **desligado**, segurar a tecla "para baixo" do direcional e ligar os dois cabos USB (A1 no PC e USB-C no carregador).
No PC aparece `10d6:10d6`. Os detalhes e o estado da cadeia de boot estão em `docs/boot-e-recuperacao.md`.
