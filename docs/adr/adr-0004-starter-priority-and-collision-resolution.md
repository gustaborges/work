# ADR-0004: Prioridade de Plugins Starter e Resolução de Colisão

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs/prd.md` — RF-1, RF-3, RNF-3, Seção 11 (métrica de plugins da comunidade)
**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seção 7 (resolução de colisão) e Seção 9 (comandos de escape hatch, sob `work memory`)

## Contexto

Definir prioridade entre starters puramente pela ordem de um array de configuração, sem sinalização ao usuário de que outros plugins também casariam com o mesmo argumento, não escala com instalação dinâmica de plugins de terceiros (ADR-0002): "em que posição um plugin novo entra?" fica pior a cada plugin novo instalado — e a própria métrica de sucesso do produto é o número de plugins criados pela comunidade (Seção 11 do PRD).

## Decisão

Resolução em duas camadas, sem lista de ordem manual:

* **Camada `fallback`** — plugins que casam qualquer argumento. Só um pode estar habilitado por vez; habilitar um segundo é erro detectado no momento da habilitação, não em runtime.
* **Camada `specific`** — plugins com um padrão de reconhecimento próprio, sem ordem entre si. Em `work start <arg>`, todo padrão `specific` habilitado é avaliado localmente contra o argumento, sem subprocesso (RNF-4):
  * exatamente um casa → invoca direto;
  * nenhum casa → cai no único plugin fallback habilitado;
  * mais de um casa → colisão: reaproveita o mesmo componente de seleção de TUI já previsto para desambiguação contribuição-vs-fork (RF-3) para perguntar ao usuário, e memoriza a escolha (RF-1).
* **Chave de memorização = o conjunto exato de plugins que colidiram** naquele match — nunca um "padrão" ou glob inferido a partir dos reconhecedores de cada plugin. Uma colisão futura só reaproveita a escolha memorizada quando o casamento (já executado a cada invocação) produzir esse mesmo conjunto de plugins de novo. Nenhuma tentativa é feita de comparar "especificidade" entre reconhecedores — essa comparação não é bem-fundamentada para padrões livres (ver Alternativas).
* Existe um escape hatch explícito para antecipar uma preferência sem esperar a colisão ocorrer, e um comando para limpar uma memorização existente — ambos sob o agrupamento `work memory` (ADR-0011), junto de qualquer outra escolha que o Work lembre em nome do usuário. Sintaxe exata em `add-0001` §9.

Esta ADR resolve especificamente colisão de *pattern matching* entre starters sobre o mesmo argumento. A escolha de convenção de branch por repositório (ADR-0011) é acionada por um gatilho diferente — primeiro uso de um repositório, não uma colisão de reconhecedores — e memorizada por chave própria; ela não reaproveita o mecanismo desta ADR.

## Alternativas consideradas

* **A — ordem manual explícita, plugin novo sempre no fim.** Simples, mas não escala com o crescimento do ecossistema; dois plugins de terceiros com reconhecedores colidentes dependem do usuário lembrar de reordenar manualmente — rejeitada.
* **B — eliminar ordem por completo via "reconhecedor mais específico vence".** Remove o conceito de ordem manual, mas "mais específico" é ambíguo para um reconhecedor de formato livre — reintroduz não-determinismo, ferindo RNF-3 — rejeitada.

## Consequências

**Positivas:** plugin novo nunca exige reordenação por padrão; determinismo preservado (mesma colisão → mesma resposta memorizada); colisão só aparece quando de fato ocorre, não como modelagem antecipada.

**Negativas / trade-offs:** precisa de um mecanismo de memorização + comando de override — complexidade adicional, mas isolada e só encontrada por quem realmente tem uma colisão real.
