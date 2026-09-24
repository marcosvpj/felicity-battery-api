# Deploy — Felicity Battery API

## Visão geral

Deploy sem Docker: bin em systemd, Caddy faz reverse proxy HTTPS na frente da porta 8080.

```
push → main
  └── CI: go vet + go build
  └── CD: cross-compile linux/amd64 → scp pro VPS → systemctl restart felicitybattery
```

---

## 1. Configurar secrets no GitHub

Em **Settings → Secrets and variables → Actions**, adicione:

| Secret | Valor |
|--------|-------|
| `VPS_HOST` | IP do VPS Contabo |
| `VPS_USER` | Usuário SSH (ex: `root`) |
| `VPS_SSH_KEY` | Conteúdo da chave privada SSH (`cat ~/.ssh/id_ed25519`) |

---

## 2. Setup na VPS (já feito)

```
/root/felicity-battery/felicity-battery   # binário
/root/felicity-battery/data/              # histórico (battery.jsonl)
```

Serviço systemd `felicitybattery.service` roda o binário e reinicia sozinho em caso de crash.
Credenciais (`FELICITY_USER`/`FELICITY_PASS`/`FELICITY_DEVICE`) ficam definidas direto na unit
(`Environment=`), não em `.env`. Caddy já faz o reverse proxy HTTPS pra porta 8080 — o workflow
não mexe nisso.

---

## 3. Deploy automático (via CI/CD)

Qualquer push pra `main` dispara o workflow `.github/workflows/deploy.yml`:

1. Cross-compila `GOOS=linux GOARCH=amd64`
2. `scp` o binário pro VPS em `/root/felicity-battery-deploy/`
3. Via SSH: guarda um backup (`felicity-battery.bak`), sobrescreve o binário, `systemctl restart felicitybattery`
4. Espera até 30s o serviço responder em `/api/health` (aceita 200 ou 503 — 503 só significa que
   ainda não rodou o primeiro poll, o que é normal logo após o restart); se não responder, o job
   falha e imprime os últimos logs do `journalctl`.

```bash
# Na sua máquina local
git add felicity-battery/
git commit -m "feat: ..."
git push origin main
# → GitHub Actions faz o resto
```

Acompanhe em **Actions** no GitHub.

Rollback manual, se precisar:

```bash
ssh root@SEU_VPS_IP
cp ~/felicity-battery/felicity-battery.bak ~/felicity-battery/felicity-battery
systemctl restart felicitybattery
```

---

## 4. Dev local

```bash
cd felicity-battery

# Modo CLI — uma leitura
./felicity-battery -user EMAIL -pass SENHA -device SN

# Modo CLI — polling contínuo com histórico
./felicity-battery -user EMAIL -pass SENHA -device SN -watch

# Modo servidor
./felicity-battery -user EMAIL -pass SENHA -device SN -serve :8080
```

`-device` (ou `FELICITY_DEVICE`) é opcional — se omitido, a API retorna o primeiro dispositivo da conta.

### Com Docker (só dev local, produção não usa)

```bash
cd felicity-battery

# Build local (usa a tag felicity-battery:dev)
docker compose build

# Subir
FELICITY_USER=email FELICITY_PASS=senha FELICITY_DEVICE=SN docker compose up
```

---

## 5. Endpoints da API

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| GET | `/api/status` | Leitura mais recente |
| GET | `/api/history` | Histórico completo (JSONL → JSON) |
| GET | `/api/health` | Status do serviço |

### Parâmetros de `/api/history`

| Parâmetro | Tipo | Default | Descrição |
|-----------|------|---------|-----------|
| `from` | RFC3339 | — | Registros com `ts >= from` |
| `to` | RFC3339 | — | Registros com `ts <= to` |
| `limit` | int | 500 | Máximo de registros (hard cap: 10000) |
| `offset` | int | 0 | Pular N registros (paginação) |

Exemplos:

```bash
# Últimas 24h (12 leituras/h × 24h = 288)
curl https://SEU_VPS_IP:8080/api/history?limit=288

# Dia específico
curl "https://SEU_VPS_IP:8080/api/history?from=2026-04-13T00:00:00Z&to=2026-04-13T23:59:59Z"

# Exportação completa
curl "https://SEU_VPS_IP:8080/api/history?limit=10000"
```

---

## 6. Manutenção

```bash
# Ver logs
journalctl -u felicitybattery -f

# Reiniciar
systemctl restart felicitybattery

# Ver uso de disco do histórico
du -sh ~/felicity-battery/data/battery.jsonl
```

---

## Flags do binário

| Flag | Default | Descrição |
|------|---------|-----------|
| `-user` | `$FELICITY_USER` | E-mail da conta Felicity |
| `-pass` | `$FELICITY_PASS` | Senha da conta |
| `-device` | `$FELICITY_DEVICE` | Serial do dispositivo (usa o primeiro se omitido) |
| `-serve` | — | Inicia servidor HTTP (ex: `:8080`) |
| `-watch` | `false` | Polling contínuo no modo CLI |
| `-history` | `data/battery.jsonl` | Caminho do arquivo de histórico |
| `-load` | `0` | Carga fixa em W para projeção de autonomia |
