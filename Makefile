# Compila os programas deste repositório para o X39 (ARMv7, sem CGO) e para o PC.
# Os cores e o RetroArch se compilam com build/build-retroarch.sh e build/build-cores.sh (precisam da toolchain).
GOARM_ENV = GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0
GOFLAGS_REL = -ldflags "-s -w" -trimpath

.PHONY: all launcher jail jail-instalar evlog test clean

all: launcher jail evlog

# lançador (Go, só biblioteca padrão)
launcher:
	cd launcher && $(GOARM_ENV) go build $(GOFLAGS_REL) -o launcher-arm .

# jaula: x39jail roda no console (ARM); x39jail-host é a versão para o PC (testes).
# Gera jail/x39jail-novo: NÃO substitui a jaula já testada. Depois de ./test-nand.sh passar com ela, rode
# "make jail-instalar" (copia para ./x39jail e pacote/x39jail, que é o que vai para o cartão).
jail:
	cd jail && $(GOARM_ENV) go build $(GOFLAGS_REL) -o x39jail-novo .
	cd jail && go build -o x39jail-host-novo .

jail-instalar:
	cp jail/x39jail-novo x39jail
	cp jail/x39jail-novo pacote/x39jail
	cp jail/x39jail-host-novo x39jail-host

# lê eventos de entrada do console (diagnóstico)
evlog:
	cd tools/diagnostico/evlog && $(GOARM_ENV) go build $(GOFLAGS_REL) -o evlog-arm .
	cd tools/diagnostico/fbwake && $(GOARM_ENV) go build $(GOFLAGS_REL) -o fbwake-arm .

test:
	cd launcher && go vet ./... && go test ./...

clean:
	rm -f launcher/launcher-arm x39jail x39jail-host pacote/x39jail tools/diagnostico/evlog/evlog-arm
