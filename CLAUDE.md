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
- **Nunca enviar sinais (`SIGSTOP`, `SIGKILL`…) a processos do firmware** (`manager`, `launcher`, `msg_server`,
  `audio_service`): o `manager` os supervisiona e reinicia o sistema (3 reinícios espontâneos em 24/10/2026).
- Console com dois ADB: usar sempre `-s 0123456789ABCDEF`. O ADB só funciona com os dois cabos USB ligados.
- Se recompilar o `x39jail`, rodar `./test-nand.sh` antes de usá-lo.

## Armadilhas

- Go: nunca abrir `/dev/input/*` com `os.OpenFile` (epoll derruba a sessão ADB); usar `syscall.Open/Read`.
- `adb shell` não devolve código de saída remoto: decidir pela saída.
- Compilar com `-mcpu=cortex-a5`; nunca `-march=armv7ve` (A5 não tem `sdiv/udiv`).
- `/dev/mem` nunca entra na jaula.
