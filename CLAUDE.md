# x39-powkiddy-launcher

Lançador e emuladores próprios para o Powkiddy X39 (SoC Actions ATM7051, Cortex-A5, Linux 3.10), sem alterar o
firmware original. Leia o `README.md` antes de mexer.

## Regras de trabalho (combinadas com o dono do aparelho)

- Comunicar em português do Brasil.
- **Pedir OK antes de rodar qualquer coisa no console** (ADB), cada vez. Compilar e testar no PC não precisa.
- **Não gravar na memória interna (NAND) nem no cartão SD original.** Nada de atualizar firmware. Só RAM (`/tmp/x39`)
  e um segundo cartão SD só nosso.
- O gancho de boot (`test.sh` (fora do repositório), único arquivo que iria à NAND) **só se instala com OK explícito, com o dono
  presente**, em etapas (veja o README). Não contornar bloqueios do sistema de permissões (por exemplo, `kill -9` do
  `manager` real foi negado: o assumir o `manager` nunca foi rodado; o modo `suave` com `SIGSTOP` também foi abandonado, veja acima).
- **Backup antes de qualquer alteração no aparelho (regra fixa).** Antes de gravar algo na NAND, instalar o gancho, aplicar
  um `update.zip`/`.fw` ou qualquer coisa que altere o aparelho: (1) rodar `instalacao/verifica-backup.sh <pasta-do-backup> --console`;
  (2) se não houver backup ou algo não bater, rodar `instalacao/backup-nand.sh <pasta>` (só lê) e conferir; (3) **dizer ao dono o
  resultado e pedir OK**. Sem backup conferido, não se grava nada. Enquanto a NAND nunca foi gravada o próprio aparelho é a fonte do
  backup; depois de qualquer gravação o backup antigo passa a ser a única cópia do estado original. O backup é um dump do
  aparelho do dono e nunca vai para o repositório.
- **Nunca enviar sinais (`SIGSTOP`, `SIGKILL`…) a processos do firmware** (`manager`, `launcher`, `msg_server`,
  `audio_service`): o `manager` os supervisiona e reinicia o sistema (3 reinícios espontâneos em 24/10/2026).
- Console com dois ADB: usar sempre `-s 0123456789ABCDEF`. O ADB só funciona com os dois cabos USB ligados.
- Se recompilar o `x39jail`, rodar `./test-nand.sh` antes de usá-lo.

## Armadilhas

- Go: nunca abrir `/dev/input/*` com `os.OpenFile` (epoll derruba a sessão ADB); usar `syscall.Open/Read`.
- `adb shell` não devolve código de saída remoto: decidir pela saída.
- Compilar com `-mcpu=cortex-a5`; nunca `-march=armv7ve` (A5 não tem `sdiv/udiv`).
- `/dev/mem` nunca entra na jaula.
