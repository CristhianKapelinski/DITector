# Resiliência de Rede e Evasão de Detecção (Anti-Bot)

Este documento detalha as contramedidas de rede implementadas no DITector para viabilizar a raspagem massiva de dados sem disparar mecanismos de defesa (WAF/Cloudflare) do Docker Hub.

---

## 1. Impersonation de Navegador (Browser Mimicry)

A stack de rede padrão do Go (`net/http`) possui uma assinatura digital (JA3 Fingerprint) facilmente identificável como script. Para mitigar isso, aplicamos três camadas de camuflagem:

### 1.1. TLS JA3 Fingerprinting
Ajustamos o `tls.Config` para emular o comportamento do Chrome 120:
*   **Cipher Suites:** Seleção manual de cifras modernas (`AES_128_GCM`, `CHACHA20_POLY1305`) em ordem de preferência do cliente.
*   **Curvas Elípticas:** Uso prioritário de `X25519` e `P256`.
*   **Protocolo:** Desativação do suporte a HTTP/2 (`TLSNextProto`) para evitar o efeito de "Tarpit" (bloqueio silencioso de multiplexing) e garantir que timeouts de aplicação funcionem corretamente em conexões atômicas.

### 1.2. Headers de Alta Fidelidade
A inclusão de "Client Hints" e headers contextuais para simular uma navegação real:
*   `Sec-Ch-Ua`: Identifica a versão exata do navegador.
*   `Sec-Fetch-Mode`: Define o contexto da requisição como `cors`.
*   `Referer`: Simula a origem da busca a partir da página oficial do Docker Hub.

---

## 2. Isolamento de Identidade (Sticky UA)

Para evitar a correlação de múltiplas contas vindo do mesmo IP, implementamos a **Identidade Persistente por Conta**:
*   Cada conta JWT é vinculada a um User-Agent fixo e exclusivo no momento do carregamento.
*   **Diferenciação por Nó:** O Nó 1 (Gama) emula identidades Windows, enquanto o Nó 2 (A9) emula identidades Linux/Mac. Isso dilui o rastro estatístico do cluster.

---

## 3. Gestão de Bloqueios e Backoff

### 3.1. Tratamento Automático de 401/403/429
*   **401 (Unauthorized):** O sistema detecta a expiração do token, invalida a sessão e força o re-login imediato através do `IdentityManager`.
*   **403 (Forbidden):** Indica um "Bot Score" alto. O worker entra em sono profundo de **15 minutos** para esfriar o IP.
*   **429 (Rate Limit):** Aplica rotação de identidade imediata com um delay de segurança de 15 segundos.

### 3.2. Body Draining (TCP Reuse)
Implementamos o dreno total do corpo da resposta (`io.Copy(io.Discard, resp.Body)`) antes de fechar cada conexão. No Go, isso é obrigatório para que o socket TCP seja devolvido ao pool e reutilizado (Keep-Alive), evitando a abertura massiva de sockets que é um sinalizador clássico de bots.

---

**Resultado:** Estas medidas reduziram drasticamente a frequência de CAPTCHAs e permitiram uma vazão constante de ~18.000 repositórios por minuto sem interrupções manuais.
