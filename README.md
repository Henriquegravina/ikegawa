# IkeGawa

*[Read this in English](README.en.md)*

GUI de leitura ao vivo e download de memória para multímetros Yokogawa
TY710/TY720/TY530 via o adaptador de comunicação 92015 (protocolo
documentado no manual Yokogawa `IM92015-02E`).

Escrito em Go + [Fyne](https://fyne.io). Funciona em Linux, Windows e macOS.
A própria interface tem um seletor de idioma (Português/English) embutido.

## Funcionalidades

- **Ao Vivo**: seleciona a porta serial, conecta e já começa a ler sozinho
  (não precisa apertar nada), mostra a leitura atual em destaque com cor por
  status (normal/over-range/bateria fraca), contador de leituras/s e /min,
  gráfico ao vivo dos últimos 200 pontos, e exportação do histórico para CSV.
  Se o instrumento parar de responder (ex.: foi desligado), o app detecta em
  poucos segundos, avisa na tela e volta a ler sozinho assim que ele voltar —
  sem travar e sem precisar reconectar a porta.
- **Log local opcional**: checkbox "Gravar log local (CSV)" — quando
  ativado, grava cada leitura ao vivo num arquivo próprio
  (`ikegawa_AAAA-MM-DD_HH-MM-SS.csv`, um por sessão de gravação) na pasta de
  logs configurada (por padrão `~/.config/IkeGawa/logs/` ou equivalente no
  SO). Fica desligado por padrão; isso é independente da memória interna do
  instrumento.
- **Modo PiP**: janela compacta (320×140, sem gráfico, sem abas, sem
  controles) mostrando só o valor atual — pensada para capturar como overlay
  numa live (janela separada, dá pra recortar no OBS/etc como fonte).
- **Memória → Modo Save**: lê um slot específico (0-99) ou baixa todos os 100
  de uma vez, com barra de progresso e exportação CSV.
- **Memória → Modo Logging**: consulta quantidade de registros e intervalo de
  amostragem, baixa todos os registros com barra de progresso/cancelamento, e
  exporta para CSV.
- **Configurações**: trocar o idioma da interface e escolher a pasta onde o
  log local é salvo (ou voltar ao padrão) — as duas escolhas ficam salvas
  entre execuções.

## Estrutura do projeto

```
.
├── main.go, chart.go, csv.go, log.go, memory.go, i18n.go   # app Fyne (pacote main)
├── internal/typroto/                               # protocolo do 92015
├── build/                                           # Dockerfiles + binários compilados (git-ignorados)
├── build.sh, build-linux.sh, build-windows.sh       # build via Docker, sem precisar de Go/gcc local
└── dev-tools/
    └── ty_read.py                                   # script standalone usado para prototipar/depurar o protocolo
```

`dev-tools/ty_read.py` não é usado pelo app — é só uma ferramenta auxiliar em
Python (pyserial) para bisbilhotar o protocolo bruto do instrumento direto no
terminal, sem precisar compilar nada.

## Rodando

```
./build/ikegawa-linux-amd64        # Linux
build\ikegawa-windows-amd64.exe    # Windows
```

## Configurando o adaptador FTDI no Linux

O cabo/adaptador 92015 usa um chip FTDI e aparece como uma porta serial
comum (`/dev/ttyUSB0`, `/dev/ttyUSB1`, etc). Normalmente funciona plug-and-
-play, mas alguns pontos ajudam a diagnosticar problemas:

1. **Confirme que foi reconhecido**: `lsusb` deve listar o adaptador (ele
   pode aparecer como "FTDI" ou, no caso do cabo da Yokogawa, como
   "KYORITSU MODEL 92015" — mesmo VID:PID `12ec:2015`, é o mesmo cabo
   fabricado sob encomenda). O driver `ftdi_sio` já vem no kernel de
   qualquer distro recente e carrega sozinho; confirme com:
   ```
   journalctl -k | grep -i ftdi
   ```
   Deve aparecer algo como `FTDI USB Serial Device converter now attached
   to ttyUSB0`.

2. **Permissão de acesso**: por padrão, `/dev/ttyUSB0` pertence ao grupo
   `dialout`. Adicione seu usuário a esse grupo e faça logout/login (ou
   `newgrp dialout` numa sessão de teste) para não precisar rodar como
   root:
   ```
   sudo usermod -aG dialout $USER
   ```

3. **O dispositivo some ou é "roubado" logo após conectar**: o culpado
   costuma ser o `brltty`, um daemon de acessibilidade (leitor Braille) que
   tenta se conectar automaticamente a qualquer adaptador serial novo,
   inclusive os FTDI. Se você não usa um leitor Braille, desative-o:
   ```
   sudo systemctl stop brltty brltty-udev.service
   sudo systemctl mask brltty brltty-udev.service
   ```

4. **(Opcional) Nome de dispositivo fixo**: se você tem mais de um
   adaptador USB-serial e a porta muda de `ttyUSB0` para `ttyUSB1` etc.
   dependendo da ordem em que são conectados, crie uma regra udev fixando
   um symlink baseado no número de série do adaptador (descubra o serial
   com `udevadm info -a -n /dev/ttyUSB0 | grep serial`):
   ```
   # /etc/udev/rules.d/99-ikegawa.rules
   SUBSYSTEM=="tty", ATTRS{idVendor}=="12ec", ATTRS{idProduct}=="2015", ATTRS{serial}=="SEU_NUMERO_DE_SERIE", SYMLINK+="ikegawa"
   ```
   Depois de salvar:
   ```
   sudo udevadm control --reload-rules && sudo udevadm trigger
   ```
   O dispositivo passa a aparecer também em `/dev/ikegawa`, além do
   `/dev/ttyUSBx` de sempre.

## Compilando

Não é necessário ter Go nem gcc instalados — tudo roda em containers Docker:

```
./build-linux.sh      # só Linux
./build-windows.sh    # só Windows (cross-compile via MinGW)
./build.sh            # os dois
```

Isso gera `build/ikegawa-linux-amd64` e/ou `build/ikegawa-windows-amd64.exe`.

### macOS

Cross-compilar para macOS a partir do Linux exige o SDK da Apple (osxcross),
que não pode ser redistribuído por licença — teria que ser extraído a partir
de uma instalação real do Xcode, ironicamente só possível já tendo um Mac.

Em compensação, compilar *nativamente* num Mac é trivial, já que lá o cgo usa
o clang que já vem com as Ferramentas de Linha de Comando do Xcode:

```
xcode-select --install        # se ainda não tiver
brew install go               # ou baixe em go.dev/dl
go build -o build/ikegawa-macos .
```

(Opcional: `go install fyne.io/tools/cmd/fyne@latest && fyne package -os darwin`
gera um `.app` com ícone e Info.plist em vez de um binário solto.)

## Protocolo

Ver `internal/typroto/typroto.go` — implementação comentada com a referência
ao manual `IM92015-02E`. Configuração serial fixa pelo instrumento: 9600 bps,
8 bits, sem paridade, **2 stop bits**, sem handshake, delimitador `<CR><LF>`.
