# Otimização de Motor Distribuído e Cache de RAM

Este documento descreve a infraestrutura de dados implementada para maximizar a eficiência do cluster DITector através de sincronia de cache e gestão persistente de tarefas.

---

## 1. Fila de Tarefas Persistente (MongoDB Backed)

Abandonamos a recursão em memória em favor de uma fila de tarefas física na coleção `crawler_keywords`.
*   **Atomicidade:** O uso de `FindOneAndUpdate` garante que múltiplos nós (Gama e A9) nunca processem o mesmo prefixo DFS simultaneamente.
*   **Self-Healing:** Ao iniciar, o sistema reseta automaticamente qualquer tarefa presa no estado `processing`, garantindo 100% de resumibilidade após quedas de energia.
*   **Priorização Dinâmica:** Tarefas falhas são re-enfileiradas com prioridade reduzida (ordenação por idade), permitindo que o sistema foque em áreas produtivas antes de re-tentar prefixos problemáticos.

---

## 2. Aquecimento de Cache (RAM Pre-loading)

Para resolver o problema de latência em arquiteturas distribuídas, implementamos o **Cache Warm-up**:
*   **O Problema:** Nós secundários (A9) baixavam dados que o Nó principal (Gama) já possuía, saturando a banda de rede com duplicatas.
*   **A Solução:** No boot, o Crawler carrega todos os nomes de repositórios existentes no banco de dados para uma `sync.Map` local em RAM.
*   **Escala:** 5.2 Milhões de registros ocupam ~300MB de RAM. O sistema suporta até 100 Milhões de registros gastando menos de 6GB de memória.
*   **Eficiência Local:** A verificação de duplicatas agora ocorre em microssegundos na RAM, eliminando requisições desnecessárias ao MongoDB e economizando I/O de disco.

---

## 3. Telemetria e Monitoramento de Eficiência

O sistema agora gera métricas em tempo real para auditoria de performance:
*   **Efficiency Metric:** Loga a porcentagem de "novidade" de cada prefixo (Novos / Baixados). Isso permite identificar áreas da árvore DFS que estão saturadas.
*   **Vazão Instantânea:** O `repoWriter` loga o número real de inserções bem-sucedidas no banco a cada 5 segundos.
*   **Estado de Fila:** Monitoramento constante do balanceamento de carga entre os nós do cluster.

---

## 4. Estabilidade de Pipeline (IO Deadlock Fix)

Identificou-se que o redirecionamento de logs via shell (`>> log.txt`) causava travamentos totais quando os buffers de shell lotavam.
*   **Solução:** Remoção de redirecionamentos manuais. Agora o Docker Engine gerencia o `stdout` de forma assíncrona, garantindo que o programa Go nunca fique bloqueado por espera de escrita em disco local para logs.

---

**Veredito de Engenharia:** A combinação de cache em RAM e fila atômica transformou o cluster em um motor de descoberta autônomo e resiliente, capaz de processar datasets de dezenas de milhões de registros com consumo linear de recursos.
