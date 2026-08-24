# ADR-0011: Resolução de Convenção de Branch por Repositório — Entrevista com Memorização

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs/prd.md` — RF-5, RF-23, RF-24, RF-25, RF-26, RNF-1, RNF-3, Roadmap (descoberta automática de convenção)
**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seção 6, Seção 8, Seção 9

## Contexto

Uma vez que convenção de branch é dado declarativo (ADR-0010), o Work precisa de um modelo para decidir, em `work start`, qual convenção — entre as declaradas por todos os plugins habilitados — vale para o repositório de destino. O PRD já exclui suporte a múltiplas convenções simultâneas dentro do mesmo projeto (Roadmap/Fora do escopo v1); falta decidir como uma única convenção é escolhida e resolvida nas próximas execuções sem repetir a pergunta.

## Decisão

**Modelo de resolução.** Não há *match* nem candidatura de plugin contra o conteúdo do repositório — não existe, hoje, nenhum componente que examine o repositório para inferir sua convenção (ADR-0010). Na primeira `work start` de um repositório sem convenção memorizada, o Work lista todas as convenções declaradas pelos componentes de convenção habilitados e o usuário escolhe uma, reaproveitando o mesmo paradigma de seleção por TUI do resto do produto (PRD, Seção 10). A escolha é memorizada e reaproveitada automaticamente nas próximas execuções nesse mesmo repositório, sem perguntar de novo.

**Colisão de nome entre plugins** (dois plugins declarando uma convenção chamada `gitflow`) é resolvida pelo mecanismo de alias do manifesto (`<alias-do-pacote>/<nome>`, ADR-0001) — não pelo mecanismo de colisão de starters (ADR-0004), que resolve um problema diferente: disputa de *pattern matching* entre reconhecedores sobre um argumento, não escolha entre nomes declarados estaticamente.

**Chave de identidade do repositório**, em camadas:

1. **URL do remote** (ex: `origin`) — estável a mover ou reclonar o repositório local; camada principal.
2. **Hash do(s) commit(s) raiz** (`git rev-list --max-parents=0 HEAD`) — usada quando não há remote configurado. Estável a mover/renomear/reclonar o diretório, e não introduz custo de infraestrutura nova (git já é invocado in-process, ADR-0000). Quando o histórico tem mais de uma raiz (históricos não relacionados mesclados, ex: `--allow-unrelated-histories` ou grafts), os hashes são ordenados e concatenados em uma única chave composta — nunca um deles é escolhido arbitrariamente, preservando o determinismo da chave (RNF-3).
3. **Caminho local absoluto** — último recurso, usado apenas quando a camada 2 não é confiável: repositório em clone raso (`git clone --depth=1`) *e* sem remote configurado. Clone raso quebra a invariância do commit raiz (a "raiz" observada é a fronteira do shallow fetch, não o commit real de início do histórico); a interseção "raso + sem remote" é rara na prática, já que um clone raso normalmente já vem de uma origem remota configurada.

Um repositório sem nenhum commit (`HEAD` unborn) não precisa de tratamento específico aqui: `work start` já falha antes, por falta de base branch (RF-6 pressupõe uma lista de branches para oferecer, inexistente nesse estado) — não é uma lacuna deste modelo, é uma precondição geral do fluxo.

**Fork de PR e identidade de repositório.** Um "fork" no vocabulário do Work (PRD, jornada 7.3) é um checkout de uma nova branch a partir da branch de um pull request existente — nunca a criação de um repositório hospedado distinto. O remote do repositório local permanece o mesmo em qualquer um dos modos de `work start` (7.1, 7.2, 7.3); não existe, no modelo de dados do Work, um cenário em que dois remotes diferentes compartilhem a mesma raiz de histórico e disputem a mesma convenção memorizada. A camada 2 da chave (commit raiz) só entra em jogo na ausência de remote configurado, algo ortogonal ao modo de `work start` escolhido.

**Trocar a convenção memorizada** de um repositório é um comando dedicado mínimo (RF-25), reaproveitando o padrão de escape hatch já estabelecido pela ADR-0004.

**Agrupamento `work memory`.** Um agrupamento de comando novo e flat (não aninhado sob um grupo "manage" genérico) reúne toda escolha que o Work lembra em nome do usuário e que pode ser inspecionada ou desfeita (RF-26): a prioridade e o esquecimento de colisão entre starters (ADR-0004) e a troca de convenção de branch por repositório (acima) vivem sob esse mesmo namespace — mecanismos de persistência diferentes por baixo (colisão de padrão vs. escolha explícita de convenção), mas o mesmo conceito do ponto de vista do usuário. Sintaxe exata em `add-0001` §9.

## Alternativas consideradas

* **Componente executável de detecção dinâmica**, que examina o repositório e devolve o nome da convenção diretamente, eliminando a entrevista. Não é rejeitada como inviável — a complexidade de um script assim é decidida pelo autor do plugin, não imposta pelo núcleo (mesmo raciocínio de ADR-0000/ADR-0006), e poderia inclusive devolver um valor fixo quando detecção real não for implementada. Adiada para uma versão futura (registrada no Roadmap do PRD): v1 resolve inteiramente por entrevista e memorização; um detector dinâmico, quando existir, é um papel executável novo em `components[]` (ADR-0010) que pode pré-preencher ou pular a entrevista sem exigir mudança no modelo de identidade/memorização aqui decidido.
* **Match de padrão sobre o conteúdo do repositório**, nos moldes do que ADR-0004 faz para starters contra um argumento textual. Rejeitada para v1: convenções são dado estático (nome + prefixos), sem nenhum sinal para comparar contra o estado do repositório — essa comparação só é possível com um componente executável de detecção (ver acima), que é exatamente a extensão futura deixada em aberto, não algo a construir agora.
* **Reaproveitar o mecanismo de colisão da ADR-0004** (memorização por conjunto exato de componentes colidentes) para a escolha de convenção. Rejeitada: são problemas diferentes — ADR-0004 resolve ambiguidade de *pattern matching* entre starters sobre um argumento; aqui não há colisão, há uma escolha de primeiro uso sem candidatos concorrendo entre si pelo mesmo sinal. Forçar o mesmo mecanismo confundiria dois conceitos de memória diferentes.
* **Agrupar os escape hatches de convenção sob `work plugin`** em vez de um namespace novo. Rejeitada: mistura gestão de ciclo de vida de plugin (instalar/habilitar/atualizar) com escolhas que o Work lembrou e o usuário pode desfazer — dois modelos mentais diferentes para quem usa o comando. Um grupo `work manage` genérico também foi avaliado e descartado por não agregar informação nenhuma além de `work memory`, com o custo adicional de renomear uma superfície já documentada (ADR-0004) sem ganho claro.
* **Flag em `work start` ou TUI dedicada só para convenção**, em vez de comando de troca. Rejeitada por inconsistência: toda outra escolha memorizada do produto já usa o padrão de comando de escape hatch (ADR-0004); introduzir um segundo padrão só para convenções não tem justificativa própria.

## Consequências

**Positivas:** determinístico (RNF-3) sem depender de heurística de detecção contra o repositório — que seria inerentemente sujeita a falso positivo/negativo; chave de identidade reaproveita git já invocado in-process, sem infraestrutura nova; separação clara entre colisão de nome (alias), colisão de padrão (ADR-0004) e escolha de convenção (aqui), cada uma resolvida pelo mecanismo certo; caminho aberto para detecção dinâmica futura sem reabrir esta decisão.

**Negativas / trade-offs:** primeiro uso de cada repositório custa uma interação extra em relação a uma detecção automática hipotética — mitigado por ser um custo único por repositório (memorizado depois) e por RF-24 garantir que sempre existe ao menos uma convenção disponível, mesmo sem nenhum plugin de terceiro instalado.
