# ADR-0010: Convenção de Branch — Contribuição Declarativa, Não Componente Executável

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs/prd.md` — RF-5, RF-23, RF-24, RNF-1, RNF-3
**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seção 4, Seção 11

## Contexto

Um plugin pode declarar uma ou mais convenções de nomenclatura de branch (ex: `gitflow`, cada uma com sua lista de prefixos). Diferente de Starter, Importer e Linker (ADR-0000, ADR-0001), essa contribuição não precisa examinar nada em tempo de execução para existir: o catálogo de convenções que um plugin oferece é conhecido inteiramente no momento em que o manifesto é escrito, não no momento em que o Work roda. A questão é se essa contribuição deve seguir o mesmo contrato de execução por processo externo (entrypoint, runtime, handshake `describe`, cache por versão — ADR-0000, ADR-0001, ADR-0005, ADR-0006) que rege os três papéis executáveis, ou se deve ser modelada de outra forma.

## Decisão

Convenção de branch é dado estático do manifesto, não um componente executável. Um plugin declara suas convenções em um array próprio, `conventions[]`, irmão de `components[]` mas com um contrato completamente diferente: cada entrada é só um nome e uma lista de prefixos/templates (ex: `feature/{slug}`) — sem `entrypoint`, sem `runtime`, sem participar do handshake `describe` nem do cache por versão que existem para os componentes executáveis. O Work nunca invoca um processo para obter ou validar uma convenção de branch; ele lê o array diretamente do manifesto no momento da instalação, do mesmo jeito que lê `name`/`version` do pacote.

O manifesto passa a ter, portanto, duas categorias de contribuição com regras próprias — o que é executado (`components[]`: Starter, Importer, Linker) e o que é dado estático (`conventions[]`) — em vez de uma quarta variante de componente que precisaria de tratamento condicional por `type` espalhado pela lógica de manifesto, registro e execução (Seção 4, Seção 11 do ADD).

## Alternativas consideradas

* **Branch-convention como um quarto tipo de componente executável**, recebendo `repo_path` via stdin e devolvendo o nome da convenção pelo mesmo contrato de Starter/Importer/Linker. Rejeitada para v1: obrigaria toda invocação de convenção a pagar o custo de um processo (spawn, handshake `describe`, cache por versão) para um papel que, hoje, não tem lógica alguma a rodar — não há detecção contra o repositório (ADR-0011), só apresentação de um catálogo estático para escolha do usuário. Forçar um script só para manter uniformidade sintática com os outros três papéis seria complexidade artificial, na contramão de RNF-1.
* **Convenção de branch como um script plugável que apenas devolve um valor fixo quando o autor não quiser implementar detecção real** (ex: um entrypoint trivial que sempre retorna `"gitflow"`). Tecnicamente viável — a complexidade de um componente executável é sempre opcional e decidida pelo autor do plugin, não imposta pelo núcleo (mesmo argumento de ADR-0000/ADR-0006 para os demais papéis) — mas rejeitada como padrão da v1 por não entregar, hoje, nenhuma capacidade que a entrevista de primeiro uso (ADR-0011) já não entregue: sem detecção real contra o repositório, um script que só devolve um nome fixo equivale a perguntar ao usuário uma vez e lembrar a resposta, com a diferença de exigir subprocesso, `runtime` e cache de versão para o mesmo resultado. Essa alternativa continua disponível como evolução futura: nada nesta decisão impede que uma versão futura introduza um componente executável de detecção dinâmica de convenção — que devolveria o nome da convenção diretamente, eliminando a entrevista — sem reabrir esta ADR, desde que modelado como um novo papel executável em `components[]`, não como mudança de `conventions[]`. Candidato registrado no roadmap do PRD.
* **Manter só `components[]`, com `type: "branch-convention"` e campos `entrypoint`/`runtime` opcionais** (ausentes = tratado como estático). Rejeitada: esconderia a distinção real dentro de campos opcionais em vez de expô-la na forma do schema, obrigando toda a lógica de manifesto/registro/execução a checar presença/ausência de campo por `type` — exatamente a ramificação condicional que a separação em dois arrays evita.

## Consequências

**Positivas:** nenhuma lógica condicional por `type` de componente é necessária no núcleo para acomodar convenções de branch; convenções de branch nunca pagam custo de subprocesso, preflight de interpretador ou cache de handshake; a porta para um futuro componente de detecção dinâmica (papel executável, à parte deste array) permanece aberta sem exigir revisão desta decisão.

**Negativas / trade-offs:** o manifesto ganha uma segunda categoria de contribuição para autores de plugin entenderem, em vez de uma única lista uniforme de componentes; evoluir o schema de `conventions[]` no futuro (ex: templates com variáveis além de `{slug}`) segue a mesma disciplina incremental já adotada para os schemas de payload por papel (ADR-0001, Seção 4 do ADD) — adição de campo, não quebra de compatibilidade.
