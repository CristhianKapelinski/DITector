# Registro de Alterações Arquiteturais (Post-Fork)

## 1. Visão Geral
Este documento descreve as modificações de engenharia realizadas no framework DITector para suportar pesquisas de segurança em larga escala no ecossistema Docker Hub.

## 2. Transição de Linguagem e Concorrência
*   **Mudança:** Substituição do motor de crawling original por uma implementação nativa em Go.
*   **Racional:** Adoção de *Goroutines* e *Channels* para maximizar o rendimento de I/O, permitindo a paralelização de centenas de requisições por segundo, superando as limitações de performance da implementação anterior.

## 3. Implementação do IdentityManager
*   **Mudança:** Introdução de um módulo centralizado para gestão de identidades (contas Docker Hub) e pools de proxies.
*   **Racional:** Implementação do princípio da *Separação de Preocupações (SoC)*. O crawler agora é agnóstico à autenticação, solicitando tokens JWT e clientes HTTP rotacionados de forma transparente, mitigando bloqueios por *Rate Limit* (HTTP 429).

## 4. Evolução da Pipeline de Dados (ETL)
*   **Mudança:** Reengenharia do estágio de `Build` para operar em modo *Worker Pool* paralelo.
*   **Racional:** O estágio de extração de infraestrutura (layers) era o principal gargalo. A nova arquitetura processa múltiplos manifestos simultaneamente e aplica filtros heurísticos antes da persistência no Neo4j, reduzindo o tempo de processamento de meses para dias.

## 5. Idempotência e Persistência
*   **Mudança:** Padronização das operações de escrita no MongoDB e Neo4j utilizando a lógica de *Upsert*.
*   **Racional:** Garantir a resiliência do sistema. Em caso de falha catastrófica, o sistema pode ser reiniciado sem gerar duplicatas ou inconsistências no grafo de dependências.
