# Proposta — Branch Convention (nome, contrato e resolução)

**Status:** Rascunho para validação externa (antes de atualizar PRD/ADD/ADR)
**Data:** 2026-08-23
**Origem:** sessão de design em conversa; nenhum documento oficial foi alterado ainda.

**Resolve:** ADD-0001, Seção 12, Questão em aberto #1 ("Contrato exato do componente `branch-strategy`... e onde a convenção correspondente é configurada").
**Elimina:** `work.conventions.json` (ADD-0001, Seção 5) — órfão, nunca referenciado em nenhum outro ponto do documento.

Cada decisão abaixo traz uma tag de classificação (`[REQUIREMENT]` / `[ARCHITECTURAL_DECISION]` / `[ARCHITECTURAL_DESIGN]`) para facilitar a distribuição posterior entre PRD, ADR e ADD.

***

## 1. Renomear `branch-strategy` → `branch-convention`

`[ARCHITECTURAL_DESIGN]` (e possível ajuste terminológico no PRD, ver Seção 8)

"Strategy" sugere um escopo maior do que o componente de fato cobre (regra de merge, cadência de release, tagging) — o que existe é só template de nome de branch. "git-conventions" foi cogitado e descartado por ir na direção oposta: abriria escopo para convenção de commit, de PR title etc., sem caso de uso real hoje. `branch-convention` nomeia exatamente o que o componente faz.

***

## 2. Componente `branch-convention` é declarativo, não executável

`[ARCHITECTURAL_DECISION]` + `[ARCHITECTURAL_DESIGN]` — candidato a nova ADR

Diferente de `starter`/`importer`/`linker`, o `branch-convention` não roda código: é dado estático no manifesto (nome da convenção + lista de prefixos/templates, ex. `feature/{slug}`). Não tem `entrypoint`, `runtime` nem participa do handshake `describe` (ADR-0006) nem do cache por versão (ADR-0005, Seção 8 do ADD).

Justificativa: o próprio catálogo de convenções não precisa de lógica alguma (não há detecção dinâmica — ver Seção 3); forçar um script só para manter uniformidade com os outros tipos de componente seria complexidade artificial, na contramão de RNF-1 (simplicidade radical do núcleo).

**Risco identificado e proposta de mitigação:** tratar isso como "só mais um `component` na mesma lista dos outros três" obriga toda a lógica de manifesto/describe/cache a ganhar branches condicionais por `type` — exceção espalhada pelo núcleo em vez de contida. Proposta: tornar a divisão explícita no schema do manifesto, com dois grupos nomeados — `components[]` continua sendo exclusivamente o que roda (starter/importer/linker); um segundo array (nome provisório: `conventions[]`) é declaradamente dado estático, sem contrato de execução algum. Isso substitui "um componente atípico no meio dos outros" por "duas categorias de contribuição, cada uma com sua própria regra", e deixa a porta aberta para futuros tipos de contribuição puramente declarativa sem reabrir esta discussão.

*Nome do array (`conventions[]` vs. alternativa) não está fechado — ponto aberto para quem revisar.*

***

## 3. Um `branch-convention` ativo por repositório, sem detecção automática

`[ARCHITECTURAL_DESIGN]`

Já coerente com o PRD (Roadmap/Fora do escopo v1: "suporte a múltiplas estratégias de branch simultâneas dentro do mesmo projeto") — nenhuma mudança de requisito aqui.

Modelo de resolução: não há "match" nem "candidatura" de plugin contra o repositório (diferente do starter, ADR-0004). Na primeira `work start` de um repositório sem convenção memorizada, o Work lista **todas** as convenções declaradas por todos os componentes `branch-convention` habilitados, e o usuário escolhe uma. A escolha é memorizada (Seção 6) e reaproveitada nas próximas execuções.

Colisão de *nome* entre plugins (dois plugins declarando uma convenção chamada `gitflow`) é resolvida pelo mecanismo de alias já existente no manifesto (`<alias-do-pacote>/<nome>`, ADD-0001 Seção 4) — **não** pelo mecanismo de colisão/memorização da ADR-0004, que resolve um problema diferente (disputa de *pattern matching* entre starters sobre um argumento).

***

## 4. Trocar a convenção memorizada de um repositório

`[ARCHITECTURAL_DESIGN]`

Comando dedicado mínimo, reaproveitando o padrão de escape hatch já estabelecido pela ADR-0004 (`work plugin forget-choice`), em vez de flag em `work start` ou uma TUI nova dedicada a convenções.

***

## 5. Novo agrupamento de comandos: `work memory`

`[ARCHITECTURAL_DESIGN]`

Comando novo, **flat** (não aninhado sob um grupo `manage` genérico — avaliado e descartado: não agrega informação, e teria custo de renomear uma superfície já documentada na ADR-0004 sem ganho claro).

`work memory` agrupa as duas famílias de "escolhas que o Work lembrou por você e você pode inspecionar/desfazer":

* `priority` / `forget-choice` — hoje descritos em `work plugin` (ADR-0004, Seção 7 do ADD); passam a viver sob `work memory`.
* a troca de convenção por repositório (Seção 4 acima).

Racional: mecanismos de persistência diferentes por baixo (colisão de starter vs. escolha explícita de convenção), mas o mesmo conceito do ponto de vista do usuário — vale a pena nomear o conceito, não o mecanismo.

*Implica mover `priority`/`forget-choice` para fora do namespace `work plugin` — ADR-0004 e ADD-0001 Seção 7 precisam refletir isso.*

***

## 6. Chave de memorização da convenção por repositório

`[ARCHITECTURAL_DESIGN]`

Resolução em camadas, na ordem:

1. **URL do remote** (ex. `origin`) — estável a mover/reclonar o repositório local; principal.
2. **Hash do commit raiz** (`git rev-list --max-parents=0 HEAD`) — usado quando não há remote configurado. Preferido sobre caminho local porque é estável a mover/renomear/reclonar o diretório, e git já é invocado in-process (ADR-0000) — sem custo de infraestrutura nova.
3. **Caminho local absoluto** — último recurso, só quando (2) não é confiável: repositório em clone raso (`git clone --depth=1`) *e* sem remote. Clone raso quebra a invariância do commit raiz (a "raiz" observada é a fronteira do shallow fetch, não o commit real de início do histórico) — mas na prática, um clone raso quase sempre tem remote (normalmente vem de CI ou de um clone rápido a partir de origem remota), então a interseção "raso + sem remote" é rara.

**Caso "zero commits" (`HEAD` unborn):** não precisa de tratamento específico aqui — antes de chegar à resolução de convenção, `work start` já falha por falta de base branch (RF-6 pressupõe uma lista de branches remotas/locais para oferecer, que não existe em um repositório sem nenhum commit). Não é uma lacuna deste design; é uma precondição geral do fluxo.

### 6.1. Trade-off em aberto — precisa de decisão de produto

Chave por commit raiz faz com que um **fork** compartilhe a mesma identidade do repositório original (mesmo histórico até a divergência) — herdaria automaticamente a convenção memorizada do upstream em vez de perguntar de novo. Pode ser desejável (herdar a convenção do projeto-pai) ou não (fork é "outro projeto" para o usuário). Não decidido nesta sessão — precisa de escolha explícita antes de fechar a ADR.

***

## 7. Remoção de `work.conventions.json`

`[ARCHITECTURAL_DESIGN]`

Substituído por uma tabela nova em `work.db` (ex. `repo_branch_convention(repo_key, convention_name, chosen_at)`), seguindo o mesmo padrão de persistência do resto do estado do sistema (ADD-0001, Seção 5). Nenhum arquivo de configuração editado à mão para isso.

***

## 8. Impacto nos documentos oficiais (para quem for aplicar depois da validação)

* **PRD:** não precisa de novo requisito — RF-5, o Objetivo de extensibilidade (linha 36), o Não-objetivo #5 e a jornada 7.1 já cobrem a intenção ao nível de produto. Único ponto de atenção: o termo de glossário "Branch Strategy" usa esse nome; se `branch-convention` for adotado na arquitetura, vale alinhar o termo do glossário para não ter dois nomes para o mesmo conceito entre camadas.
* **ADR novo(s):** (a) componente declarativo vs. executável no manifesto (Seção 2 acima) e a divisão `components[]`/`conventions[]`; (b) modelo de resolução por interview-and-remember do `branch-convention`, com a chave em camadas (Seção 6) — decisão distinta da ADR-0004 (que é especificamente sobre colisão de *pattern matching* de starters), não uma reinterpretação dela.
* **ADD-0001:** atualizar Seção 4 (schema do manifesto), Seção 5 (remover `work.conventions.json`, adicionar tabela de memorização), Seção 7 (mover `priority`/`forget-choice` para `work memory`), Seção 12 (fechar Questão #1, referenciando a(s) ADR(s) nova(s)).
* **ADR-0004:** ajustar título/texto para deixar explícito que o mecanismo ali descrito é específico de colisão por pattern matching de starters, já que agora existe um segundo mecanismo de "escolha memorizada" (convenção) que **não** o reutiliza.
