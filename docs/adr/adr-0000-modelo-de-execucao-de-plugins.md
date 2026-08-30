# ADR-0000: Modelo de Execução de Plugins — Subprocessos Externos com Contrato Padronizado, Não Biblioteca In-Process

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs/prd.md` — RNF-1, RNF-2, RNF-7, RF-22

## Contexto

O PRD estabelece que, fora o Git, o núcleo não embute conhecimento de nenhuma ferramenta específica (RNF-1, RNF-2, RF-22): "o núcleo só conhece contratos e executáveis externos — a única invocação direta e interna é o próprio Git" é uma decisão arquitetural que decorre desse princípio, não um requisito de produto em si — por isso vive aqui, não no PRD. Esta ADR dá a essa decisão o registro formal que as demais ADRs deste conjunto (0001 em diante) já assumem como premissa.

O problema de fundo: como o Work deve executar a lógica de domínio de um plugin (interpretar um argumento, enriquecer contexto, relacionar recursos externos) sem que um bug ou comportamento malicioso desse plugin comprometa a estabilidade ou a segurança do núcleo?

## Decisão

O `git` é a única ferramenta que o núcleo do Work invoca diretamente, in-process. Todo o restante do comportamento de domínio que envolve lógica — qualquer plugin que cumpra o papel de Starter, Importer ou Linker — roda como um processo externo, separado do processo do Work, comunicando-se por um contrato de entrada e saída bem definido e publicamente documentado. O núcleo nunca importa, carrega ou executa código de plugin dentro do seu próprio processo.

Uma contribuição de plugin sem lógica alguma — o catálogo de convenções de branch (ADR-0012) — não se enquadra nessa decisão por não ter comportamento de domínio a executar: é dado estático do manifesto, não um processo. A decisão aqui registrada é sobre como o Work roda o que precisa ser executado, não uma exigência de que toda contribuição de plugin seja executável.

Os detalhes concretos do contrato (formato do payload, transporte, convenção de código de saída) estão descritos em `docs/add/add-0001-work-system-architecture.md`, Seção 11 — esta ADR registra apenas a decisão de que a execução é sempre por processo externo, não a mecânica exata dessa comunicação.

Essa escolha é o que torna possível a neutralidade de linguagem decidida em ADR-0006: um modelo de execução in-process amarraria necessariamente os plugins à linguagem/runtime do núcleo.

## Alternativas consideradas

* **Plugin in-process, carregado como biblioteca/extensão dentro do processo do Work** (modelo VS Code). Rejeitada: um plugin com bug ou comportamento malicioso derruba ou compromete o próprio processo do Work, e amplia a superfície de código que o usuário precisa confiar sem revisão — o oposto direto de RNF-7. Esse modelo também amarraria plugins à linguagem/runtime do núcleo, o que um levantamento de precedentes de mercado (git subcommands, credential helpers, LSP, `asdf`, `gh extension`, Terraform) mostra não ser necessário: toda ferramenta cujo modelo é "protocolo + subprocesso externo" é agnóstica de linguagem por design — a única exceção observada (VS Code) é justamente o modelo aqui rejeitado.
* **Plugins compilados/linkados estaticamente no binário do Work.** Rejeitada: exigiria recompilar e republicar o próprio Work para adicionar um plugin de terceiro, o oposto de RNF-2 (extensibilidade sem fricção) e RF-22.

## Consequências

**Positivas:** um plugin com bug nunca derruba o processo do Work; a superfície de código que o usuário precisa auditar sem revisão fica restrita ao núcleo; nenhuma amarração de linguagem é imposta a quem escreve um plugin (ver ADR-0006).

**Negativas / trade-offs:** cada invocação de plugin paga o custo de criação de um processo separado — aceitável porque essas invocações acontecem em pontos discretos do fluxo (início de um Work, hooks de finalização), nunca em um loop quente.
