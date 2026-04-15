# Deploy — Felicity Battery API

## Visão geral

O CI/CD usa GitHub Actions + GitHub Container Registry (GHCR):

```
push → main
  └── CI: go vet + go build
  └── CD: build imagem Docker → push GHCR → SSH VPS → docker compose up
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

## 2. Setup inicial no VPS (uma vez)

```bash
# Criar estrutura de diretórios
mkdir -p ~/felicity-battery/data

# Criar o .env com credenciais
cat > ~/felicity-battery/.env << 'EOF'
FELICITY_USER=seu@email.com
FELICITY_PASS=suasenha
DOCKER_IMAGE=ghcr.io/SEU_USUARIO_GITHUB/felicity-battery:latest
EOF

# Copiar o docker-compose.yml (da sua máquina local)
scp felicity-battery/docker-compose.yml user@SEU_VPS_IP:~/felicity-battery/
```

Se o repositório GitHub for **privado**, autentique o Docker no VPS para pull do GHCR:

```bash
# No VPS — gere um token em github.com/settings/tokens (escopo: read:packages)
echo SEU_GITHUB_TOKEN | docker login ghcr.io -u SEU_USUARIO_GITHUB --password-stdin
```

Se o repositório for **público**, o GHCR é público automaticamente — sem autenticação necessária.

---

## 3. Primeiro deploy manual (opcional)

Para validar o setup antes de abrir o CI/CD:

```bash
# No VPS
cd ~/felicity-battery
docker compose pull
docker compose --env-file .env up -d
docker compose logs -f
```

Verificar:

```bash
curl http://localhost:8080/api/health
curl http://localhost:8080/api/status
```

---

## 4. Deploy automático (via CI/CD)

A partir daqui, qualquer push para `main` com mudanças em `felicity-battery/**` dispara o pipeline automaticamente.

```bash
# Na sua máquina local
git add felicity-battery/
git commit -m "feat: ..."
git push origin main
# → GitHub Actions faz o resto
```

Acompanhe em **Actions** no GitHub.

---

## 5. Dev local (sem Docker)

```bash
cd felicity-battery

# Modo CLI — uma leitura
./felicity-battery -user EMAIL -pass SENHA

# Modo CLI — polling contínuo com histórico
./felicity-battery -user EMAIL -pass SENHA -watch

# Modo servidor
./felicity-battery -user EMAIL -pass SENHA -serve :8080
```

### Com Docker local

```bash
cd felicity-battery

# Build local (usa a tag felicity-battery:dev)
docker compose build

# Subir
FELICITY_USER=email FELICITY_PASS=senha docker compose up
```

---

## 6. Endpoints da API

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

## 7. Manutenção

```bash
# Ver logs
docker compose logs -f

# Reiniciar
docker compose restart

# Atualizar imagem manualmente (sem esperar o CI)
docker compose pull && docker compose up -d

# Ver uso de disco do histórico
du -sh ~/felicity-battery/data/battery.jsonl
```

---

## Flags do binário

| Flag | Default | Descrição |
|------|---------|-----------|
| `-user` | `$FELICITY_USER` | E-mail da conta Felicity |
| `-pass` | `$FELICITY_PASS` | Senha da conta |
| `-device` | `074502417125380378` | Serial do dispositivo |
| `-serve` | — | Inicia servidor HTTP (ex: `:8080`) |
| `-watch` | `false` | Polling contínuo no modo CLI |
| `-history` | `data/battery.jsonl` | Caminho do arquivo de histórico |
| `-load` | `0` | Carga fixa em W para projeção de autonomia |
