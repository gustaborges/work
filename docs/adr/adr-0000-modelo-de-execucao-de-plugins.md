# ADR-0000: Modelo de Execução de Plugins — Subprocessos Externos com Contrato Padronizado, Não Biblioteca In-Process

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs-v2/prd.md` — RNF-1, RNF-2, RNF-7, RF-22
**Supersede:** `docs/prd.md` (v1), Seção 11 ("Arquitetura em Alto Nível")

## Contexto

O rascunho v1 do PRD trazia, em sua Seção 11, um princípio de arquitetura embutido no próprio documento de produto: "o núcleo só conhece contratos e executáveis externos — a única invocação direta e interna é o próprio Git". Isso é uma decisão arquitetural, não um requisito de produto, e por isso não pertence mais ao PRD (ver `docs-v2/prd.md`, que a remove). Esta ADR existe para dar a essa decisão o registro formal que ela sempre teve implicitamente, já que todas as demais ADRs deste conjunto (0001 em diante) a assumem como premissa.

O problema de fundo: como o Work deve executar a lógica de domínio de um plugin (interpretar um argumento, enriquecer contexto, descobrir convenção de branch) sem que um bug ou comportamento malicioso desse plugin comprometa a estabilidade ou a segurança do núcleo?

## Decisão

O `git` é a única ferramenta que o núcleo do Work invoca diretamente, in-process. Todo o restante do comportamento de domínio — qualquer plugin que cumpra o papel de Starter, Importer, Linker ou descoberta de Branch Strategy — roda como um processo externo, separado do processo do Work, comunicando-se por um contrato de entrada e saída bem definido e publicamente documentado. O núcleo nunca importa, carrega ou executa código de plugin dentro do seu próprio processo.

Os detalhes concretos do contrato (formato do payload, transporte, ação de handshake de capabilities) estão descritos em `docs-v2/add/add-0001-work-system-architecture.md`, Seção 9 — esta ADR registra apenas a decisão de que a execução é sempre por processo externo, não a mecânica exata dessa comunicação.

Essa escolha é o que torna possível a neutralidade de linguagem decidida em ADR-0006: um modelo de execução in-process amarraria necessariamente os plugins à linguagem/runtime do núcleo.

## Alternativas consideradas

* **Plugin in-process, carregado como biblioteca/extensão dentro do processo do Work** (modelo VS Code). Rejeitada: um plugin com bug ou comportamento malicioso derruba ou compromete o próprio processo do Work, e amplia a superfície de código que o usuário precisa confiar sem revisão — o oposto direto de RNF-7. Esse modelo também amarraria plugins à linguagem/runtime do núcleo, o que um levantamento de precedentes de mercado (git subcommands, credential helpers, LSP, `asdf`, `gh extension`, Terraform) mostra não ser necessário: toda ferramenta cujo modelo é "protocolo + subprocesso externo" é agnóstica de linguagem por design — a única exceção observada (VS Code) é justamente o modelo aqui rejeitado.
* **Plugins compilados/linkados estaticamente no binário do Work.** Rejeitada: exigiria recompilar e republicar o próprio Work para adicionar um plugin de terceiro, o oposto de RNF-2 (extensibilidade sem fricção) e RF-22.

## Consequências

**Positivas:** um plugin com bug nunca derruba o processo do Work; a superfície de código que o usuário precisa auditar sem revisão fica restrita ao núcleo; nenhuma amarração de linguagem é imposta a quem escreve um plugin (ver ADR-0006).

**Negativas / trade-offs:** cada invocação de plugin paga o custo de criação de um processo separado — aceitável porque essas invocações acontecem em pontos discretos do fluxo (início de um Work, hooks de finalização), nunca em um loop quente.
