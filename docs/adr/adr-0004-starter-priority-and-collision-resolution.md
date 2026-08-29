# ADR-0004: Prioridade de Plugins Starter e Resolução de Colisão

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs/prd.md` — RF-1, RF-3, RNF-3, Seção 11 (métrica de plugins da comunidade)
**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seção 7 (resolução de colisão)

## Contexto

Definir prioridade entre starters puramente pela ordem de um array de configuração, sem sinalização ao usuário de que outros plugins também casariam com o mesmo argumento, não escala com instalação dinâmica de plugins de terceiros (ADR-0002): "em que posição um plugin novo entra?" fica pior a cada plugin novo instalado — e a própria métrica de sucesso do produto é o número de plugins criados pela comunidade (Seção 11 do PRD).

## Decisão

Resolução em duas camadas, sem lista de ordem manual:

* **Camada `fallback`** — plugins que casam qualquer argumento. Só um pode estar habilitado por vez; habilitar um segundo é erro detectado no momento da habilitação, não em runtime.
* **Camada `specific`** — plugins com um padrão de reconhecimento próprio, sem ordem entre si. Em `work start <arg>`, todo padrão `specific` habilitado é avaliado localmente contra o argumento, sem subprocesso (RNF-4):
  * exatamente um casa → invoca direto;
  * nenhum casa → cai no único plugin fallback habilitado;
  * mais de um casa → colisão: o Work informa ao usuário que há mais de um plugin candidato para aquele argumento, lista quais, e reaproveita o mesmo componente de seleção de TUI já previsto para desambiguação contribuição-vs-fork (RF-3) para o usuário escolher qual usar (RF-1).
* **A escolha não é persistida.** A mesma colisão é resolvida com a mesma pergunta explícita a cada `work start` — não há memorização, comando de prioridade nem comando de "esquecer". Nenhuma tentativa é feita de comparar "especificidade" entre reconhecedores para decidir automaticamente — essa comparação não é bem-fundamentada para padrões livres (ver Alternativas).

Esta ADR resolve especificamente colisão de *pattern matching* entre starters sobre o mesmo argumento, sempre na hora e sem estado. A escolha de convenção de branch por repositório (ADR-0011) é acionada por um gatilho diferente — primeiro uso de um repositório — e essa sim é memorizada, por chave e comando próprios; decisão separada, sem relação com esta.

## Alternativas consideradas

* **A — ordem manual explícita, plugin novo sempre no fim.** Simples, mas não escala com o crescimento do ecossistema; dois plugins de terceiros com reconhecedores colidentes dependem do usuário lembrar de reordenar manualmente — rejeitada.
* **B — eliminar ordem por completo via "reconhecedor mais específico vence".** Remove o conceito de ordem manual, mas "mais específico" é ambíguo para um reconhecedor de formato livre — reintroduz não-determinismo, ferindo RNF-3 — rejeitada.
* **C — memorizar a escolha por conjunto exato de plugins colidentes, com comandos de escape hatch (`work memory priority` / `work memory forget-choice`).** Era a decisão anterior desta ADR. Rejeitada por custo/benefício: uma colisão real exige dois plugins `specific` com padrões sobrepostos habilitados ao mesmo tempo — situação rara — e o mecanismo cobrava uma chave de persistência dedicada, dois subcomandos novos e um agrupamento (`work memory`) para hospedá-los. Perguntar de novo a cada ocorrência é um atrito pequeno e localizado em quem de fato tem a colisão, e mantém o núcleo menor (RNF-1).

## Consequências

**Positivas:** plugin novo nunca exige reordenação por padrão; determinismo preservado (mesma colisão → a mesma pergunta explícita, sem estado escondido influenciando a resolução); colisão só aparece quando de fato ocorre, não como modelagem antecipada; núcleo menor — nenhuma tabela de prioridade, nenhum subcomando de override.

**Negativas / trade-offs:** uma colisão real volta a perguntar a cada `work start` com o mesmo argumento ambíguo. Aceitável: exige dois plugins `specific` com padrões sobrepostos habilitados simultaneamente (raro), e o usuário resolve na hora, sem precisar antecipar configuração de prioridade.
