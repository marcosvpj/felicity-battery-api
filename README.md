# felicity-battery

Serviço Go que consulta a API cloud da Felicity Solar e expõe os dados da bateria **LiFePO4 FAL24171-EU (24 V / 171 Ah)** via HTTP REST + dashboard web.

---

## Modos de uso

```bash
# Uma leitura (terminal)
./felicity-battery -user EMAIL -pass SENHA -device SN

# Polling contínuo a cada 5 min (terminal + JSONL)
./felicity-battery -user EMAIL -pass SENHA -device SN -watch

# Servidor HTTP + dashboard na porta 8080
./felicity-battery -user EMAIL -pass SENHA -device SN -serve :8080
```

Credenciais também podem ser passadas via variáveis de ambiente:

```bash
export FELICITY_USER=seu@email.com
export FELICITY_PASS=suasenha
```

### Flags disponíveis

| Flag | Default | Descrição |
|------|---------|-----------|
| `-user` | `$FELICITY_USER` | E-mail da conta Felicity |
| `-pass` | `$FELICITY_PASS` | Senha da conta |
| `-device` | — | Serial do dispositivo (usa o primeiro se omitido) |
| `-serve` | — | Inicia servidor HTTP (ex: `:8080`) |
| `-watch` | `false` | Polling contínuo no modo CLI |
| `-history` | `data/battery.jsonl` | Caminho do arquivo de histórico |
| `-load` | `0` | Carga fixa em W para projeção de autonomia |

---

## Build e desenvolvimento

```bash
go build -o felicity-battery .         # compilar
go fmt ./...                           # formatar (sempre antes do commit)
go vet ./...                           # lint
CGO_ENABLED=0 go build -o /dev/null ./...  # verificação CI
```

Sem dependências externas — apenas stdlib Go.

### Docker (local)

```bash
docker compose build
FELICITY_USER=email FELICITY_PASS=senha docker compose up
```

---

## API HTTP

Disponível no modo `-serve`. Base URL: `http://localhost:8080`

| Endpoint | Descrição |
|----------|-----------|
| `GET /` | Dashboard web embutido |
| `GET /api/status` | Leitura mais recente |
| `GET /api/history` | Histórico com filtros `from`/`to`/`limit`/`offset` |
| `GET /api/health` | Status do poller (ok, stale, último erro) |

CORS habilitado (`*`). Granularidade máxima: **1 leitura a cada 5 minutos**.

Documentação completa: [`API.md`](API.md)

---

## Histórico

As leituras são acumuladas em `data/battery.jsonl` (um objeto JSON por linha, JSONL). O arquivo cresce ~105 mil registros/ano (~8 MB). Linhas corrompidas são ignoradas silenciosamente.

---

## Deploy no VPS

CI/CD via GitHub Actions:

```
push → main (felicity-battery/**)
  ├── CI: go vet + go build
  └── CD: build Docker → push GHCR → SSH VPS → docker compose up -d
```

Instruções completas: [`DEPLOY.md`](DEPLOY.md)

---

## Arquitetura

```
Felicity cloud REST API
  └── api.go          →  BatterySnapshot  (campos *string crus)
        └── history.go →  HistoryRecord   (valores tipados, unidades SI)
              ├── display.go  →  terminal dashboard (CLI)
              ├── history.go  →  data/battery.jsonl
              └── server.go   →  HTTP handlers
```

| Arquivo | Responsabilidade |
|---------|-----------------|
| `api.go` | Cliente HTTP, autenticação, `BatterySnapshot` |
| `history.go` | Conversão de tipos, leitura/escrita JSONL |
| `display.go` | Dashboard ANSI no terminal |
| `server.go` | Poller goroutine, `serverState`, handlers HTTP |
| `static.go` | `dashboard.html` embutido como string literal |
| `main.go` | Parsing de flags, ponto de entrada |
