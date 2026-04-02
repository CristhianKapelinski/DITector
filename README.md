# DITector - Large-Scale Security Measurement of Docker Image Ecosystem

## Project Goal
Este projeto visa realizar um **escaneamento dinâmico em larga escala** (~100.000 containers) do Docker Hub para identificar vulnerabilidades de rede usando o **OpenVAS**.

A estratégia de seleção e priorização baseia-se no framework **DITector** e na implementação de um crawler distribuído de alta performance.

### Metodologia de Pesquisa
1.  **Crawling Distribuído:** Crawler em Go com suporte a DFS (Depth-First Search) para varrer todos os 12M+ repositórios do Docker Hub.
2.  **Resiliência Industrial:** Tratamento nativo de erros HTTP 429 (Rate Limit), login sincronizado e rotação de identidades.
3.  **Priorização por Impacto:** Construção de grafo no Neo4j para selecionar imagens com maior "Dependency Weight" e "Pull Count".
4.  **Filtro de Rede:** Identificação de containers que expõem portas (EXPOSE) para scan via OpenVAS.

---

## 🚀 Como Executar o Crawler (Modo Pesquisa)

### 1. Configuração (Importante)
Certifique-se de que o arquivo `config.yaml` tenha as configurações de proxy vazias para conexões diretas (a menos que use um pool de proxies real):
```yaml
proxy:
  http_proxy: ''
  https_proxy: ''
```

### 2. Contas do Docker Hub
Crie o arquivo `accounts.json` na raiz:
```json
[
  { "username": "seu_user", "password": "seu_password" }
]
```

### 3. Execução via Docker Compose (Recomendado)
Sobe o MongoDB, Neo4j e o Crawler automaticamente:
```bash
docker compose up -d
```

### 4. Meet-in-the-Middle (Multi-Máquina)
Para acelerar o processo entre várias máquinas, use a flag `--seed`:

**Máquina 1 (Começa em A):**
```bash
go run main.go crawl --workers 20 --seed 'a' --accounts accounts.json
```

**Máquina 2 (Começa em N):**
```bash
go run main.go crawl --workers 20 --seed 'n' --accounts accounts.json
```

---

## 📊 Monitoramento
Acompanhe a descoberta em tempo real:
```bash
tail -f *.log | grep "Discovered repository"
```

Verifique a contagem no MongoDB:
```bash
mongosh localhost:27017/dockerhub_data --eval 'db.repositories_data.countDocuments()'
```

---
*Veja o [CHANGELOG.md](./CHANGELOG.md) para detalhes técnicos das melhorias de performance e estabilidade.*
