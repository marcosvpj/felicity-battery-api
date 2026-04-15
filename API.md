# Felicity Battery — API Reference

API pública para leitura de dados da bateria LiFePO4 FAL24171-EU.
Serve de base para o desenvolvimento do dashboard offgrid.

**Base URL:** `http://SEU_VPS_IP:8080`

---

## Visão geral

| Endpoint | Descrição |
|----------|-----------|
| `GET /api/status` | Leitura mais recente da bateria |
| `GET /api/history` | Histórico de leituras com filtros |
| `GET /api/health` | Status do serviço |

**Características gerais:**
- Todas as respostas são `Content-Type: application/json`
- CORS habilitado: `Access-Control-Allow-Origin: *`
- A bateria envia dados a cada **5 minutos** — essa é a granularidade máxima do histórico
- Todos os timestamps estão em **UTC, formato RFC3339** (ex: `2026-04-13T00:34:08Z`)

---

## Objeto `Reading`

Estrutura central retornada por `/api/status` e dentro de `/api/history`. Todos os valores já estão em unidades humanas — nenhuma conversão necessária no frontend.

```json
{
  "ts":            "2026-04-13T00:34:08Z",
  "device_sn":     "074502417125380378",
  "soc":           26.0,
  "soh":           100.0,
  "capacity_ah":   171.0,
  "volt_v":        26.05,
  "curr_a":        -2.5,
  "power_w":       -65.13,
  "temp_max_c":    19.0,
  "temp_min_c":    19.0,
  "cell_volts_mv": [3253, 3254, 3252, 3255, 3255, 3253, 3255, 3253],
  "cell_temps_c":  [19.0, 19.0, 19.0, 19.0],
  "status":        "NM"
}
```

### Campos

| Campo | Tipo | Unidade | Descrição |
|-------|------|---------|-----------|
| `ts` | string (RFC3339) | — | Timestamp da leitura em UTC |
| `device_sn` | string | — | Serial do dispositivo |
| `soc` | number | % | State of Charge — carga atual da bateria |
| `soh` | number | % | State of Health — saúde da bateria (100% = nova) |
| `capacity_ah` | number | Ah | Capacidade usável atual |
| `volt_v` | number | V | Tensão do banco de baterias |
| `curr_a` | number | A | Corrente — **negativo = descarregando**, positivo = carregando |
| `power_w` | number | W | Potência — **negativo = descarregando**, positivo = carregando |
| `temp_max_c` | number | °C | Temperatura máxima entre as células |
| `temp_min_c` | number | °C | Temperatura mínima entre as células |
| `cell_volts_mv` | number[] | mV | Tensão individual de cada célula ativa (slots vazios excluídos) |
| `cell_temps_c` | number[] | °C | Temperatura de cada sensor ativo |
| `status` | string | — | Estado do BMS — ver tabela abaixo |
| `hours_left` | number \| null | h | Horas até a bateria esvaziar à taxa de descarga atual; `null` quando carregando ou corrente < 1 A |
| `hours_to_full` | number \| null | h | Horas até a bateria chegar a 100% à taxa de carga atual; `null` quando descarregando ou corrente < 1 A |

### Valores de `status`

| Valor | Significado |
|-------|-------------|
| `"NM"` | Normal — operação normal |
| `""` | Desconhecido |

### Convenção de sinal

```
curr_a > 0  →  carregando   (energia entrando)
curr_a < 0  →  descarregando (energia saindo)
curr_a ≈ 0  →  idle

power_w segue a mesma convenção.
```

### Limites operacionais desta bateria

| Parâmetro | Valor |
|-----------|-------|
| Tensão nominal | 24 V |
| Capacidade nominal | 171 Ah |
| Células ativas | 8 (slots 9–16 não populados) |
| Limite de corrente de carga | 120 A |
| Limite de corrente de descarga | 120 A |
| Tensão de corte de carga | 28,8 V |
| Tensão de corte de descarga | 24,0 V |

---

## `GET /api/status`

Retorna a leitura mais recente da bateria.

### Resposta — 200 OK

Objeto `Reading` completo (ver acima).

```json
{
  "ts":            "2026-04-13T00:34:08Z",
  "device_sn":     "074502417125380378",
  "soc":           26.0,
  "soh":           100.0,
  "capacity_ah":   171.0,
  "volt_v":        26.05,
  "curr_a":        -2.5,
  "power_w":       -65.13,
  "temp_max_c":    19.0,
  "temp_min_c":    19.0,
  "cell_volts_mv": [3253, 3254, 3252, 3255, 3255, 3253, 3255, 3253],
  "cell_temps_c":  [19.0, 19.0, 19.0, 19.0],
  "status":        "NM"
}
```

### Resposta — 503 Service Unavailable

Retornado quando o serviço ainda não completou o primeiro poll (ex: logo após o boot do container) ou quando há falha de comunicação com a API Felicity.

```json
{
  "error": "no data yet"
}
```

### Exemplo

```js
const res = await fetch('http://SEU_VPS_IP:8080/api/status')
const reading = await res.json()

console.log(`SOC: ${reading.soc}%`)
console.log(`${reading.curr_a < 0 ? 'Descarregando' : 'Carregando'} ${Math.abs(reading.power_w)}W`)
```

---

## `GET /api/history`

Retorna o histórico de leituras, do mais recente para o mais antigo.

### Query parameters

| Parâmetro | Tipo | Default | Descrição |
|-----------|------|---------|-----------|
| `from` | string (RFC3339) | — | Incluir apenas registros com `ts >= from` |
| `to` | string (RFC3339) | — | Incluir apenas registros com `ts <= to` |
| `limit` | integer | `500` | Máximo de registros retornados (hard cap: `10000`) |
| `offset` | integer | `0` | Pular os N primeiros registros após filtro (paginação) |

### Resposta — 200 OK

```json
{
  "count": 2,
  "records": [
    {
      "ts":            "2026-04-13T00:34:08Z",
      "soc":           26.0,
      "volt_v":        26.05,
      "curr_a":        -2.5,
      "power_w":       -65.13,
      "temp_max_c":    19.0,
      "temp_min_c":    19.0,
      "cell_volts_mv": [3253, 3254, 3252, 3255, 3255, 3253, 3255, 3253],
      "cell_temps_c":  [19.0, 19.0, 19.0, 19.0],
      "status":        "NM",
      "device_sn":     "074502417125380378",
      "soh":           100.0,
      "capacity_ah":   171.0
    },
    {
      "ts": "2026-04-12T23:59:08Z",
      "..."
    }
  ]
}
```

- `count` — número de registros **nesta resposta** (após filtros e limit), não o total histórico
- `records` — array de objetos `Reading`, ordenados do mais novo para o mais antigo

Quando não há dados: `{ "count": 0, "records": [] }`

### Resposta — 400 Bad Request

```json
{ "error": "invalid 'from': use RFC3339" }
{ "error": "invalid 'to': use RFC3339" }
{ "error": "invalid 'limit'" }
{ "error": "invalid 'offset'" }
```

### Exemplos de uso

```js
const BASE = 'http://SEU_VPS_IP:8080'

// Últimas 24 horas (12 leituras/h × 24h = 288)
const res = await fetch(`${BASE}/api/history?limit=288`)
const { records } = await res.json()

// Últimos 7 dias
const res = await fetch(`${BASE}/api/history?limit=2016`)  // 12 × 24 × 7

// Dia específico
const from = '2026-04-13T00:00:00Z'
const to   = '2026-04-13T23:59:59Z'
const res = await fetch(`${BASE}/api/history?from=${from}&to=${to}`)

// Paginação — página 2 com 100 registros por página
const res = await fetch(`${BASE}/api/history?limit=100&offset=100`)

// Exportação completa
const res = await fetch(`${BASE}/api/history?limit=10000`)
```

### Frequência e volume de dados

| Período | Registros aprox. |
|---------|-----------------|
| 1 hora | 12 |
| 24 horas | 288 |
| 7 dias | 2.016 |
| 30 dias | 8.640 |
| 1 ano | ~105.000 |

---

## `GET /api/health`

Verifica se o serviço está operacional e comunicando com a API Felicity.

### Resposta — 200 OK (serviço saudável)

```json
{
  "ok":         true,
  "last_poll":  "2026-04-13T00:34:08Z",
  "last_error": null,
  "data_stale": false
}
```

### Resposta — 503 Service Unavailable (serviço degradado)

```json
{
  "ok":         false,
  "last_poll":  "2026-04-13T00:20:00Z",
  "last_error": "login failed: bad credentials",
  "data_stale": true
}
```

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `ok` | boolean | `true` quando o último poll foi bem-sucedido e há menos de 10 min |
| `last_poll` | string \| null | Timestamp do último poll tentado (sucesso ou falha) |
| `last_error` | string \| null | Mensagem de erro do último poll, ou `null` se bem-sucedido |
| `data_stale` | boolean | `true` quando `last_poll` é nulo ou há mais de 10 min |

**Regras:**
- `ok = true` exige: `last_error == null` **e** `last_poll` dentro de 10 min
- `data_stale = true` sozinho não implica erro — pode ser startup ou poll em andamento

### Exemplo — banner de status no dashboard

```js
async function checkHealth() {
  const res = await fetch('http://SEU_VPS_IP:8080/api/health')
  const { ok, data_stale, last_poll } = await res.json()

  if (!ok && data_stale && !last_poll) return 'Conectando...'
  if (!ok) return 'Dados desatualizados — verifique a conexão'
  return null  // tudo ok, não exibir banner
}
```

---

## CORS

Todos os endpoints aceitam requisições cross-origin. Os headers retornados em toda resposta:

```
Access-Control-Allow-Origin:  *
Access-Control-Allow-Methods: GET, OPTIONS
Access-Control-Allow-Headers: Content-Type
```

Requisições `OPTIONS` (preflight) retornam `204 No Content` imediatamente.

---

## Referências para o dashboard

### Gráficos sugeridos

| Gráfico | Campos | Unidade |
|---------|--------|---------|
| SOC ao longo do tempo | `ts` × `soc` | % |
| Potência ao longo do tempo | `ts` × `power_w` | W |
| Tensão da bateria | `ts` × `volt_v` | V |
| Temperatura | `ts` × `temp_max_c`, `temp_min_c` | °C |
| Tensão por célula (snapshot) | `cell_volts_mv[0..7]` | mV |

### Cálculos úteis

```js
// Energia restante estimada
const energyWh = (reading.soc / 100) * reading.capacity_ah * reading.volt_v

// Tempo restante até vazio / até cheio — já calculado pela API
// reading.hours_left   → número de horas até esvaziar (null se carregando)
// reading.hours_to_full → número de horas até 100% (null se descarregando)
//
// Cálculo equivalente no cliente (caso precise recomputar):
const remainingAh = (reading.soc / 100) * reading.capacity_ah
const hoursLeft = reading.curr_a < -1
  ? remainingAh / Math.abs(reading.curr_a)
  : null
const hoursToFull = reading.curr_a > 1
  ? ((1 - reading.soc / 100) * reading.capacity_ah) / reading.curr_a
  : null

// Desvio máximo entre células (indicador de balanceamento)
const cellMax = Math.max(...reading.cell_volts_mv)
const cellMin = Math.min(...reading.cell_volts_mv)
const imbalanceMv = cellMax - cellMin
```

### Intervalos de referência

| Métrica | Crítico | Atenção | Normal |
|---------|---------|---------|--------|
| SOC | < 20% | 20–50% | > 50% |
| Célula (mV) | < 3000 | 3000–3200 | > 3200 |
| Temperatura | > 40°C | 30–40°C | ≤ 30°C |
| Desvio entre células | > 50 mV | 20–50 mV | < 20 mV |
