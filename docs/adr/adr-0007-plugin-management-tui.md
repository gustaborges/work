# ADR-0007: `work plugin` como TUI — Camada Fina Sobre Comandos Scriptáveis

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs/prd.md` — Seção 10, RF-16 a RF-21, RNF-4
**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seção 12

## Contexto

Com plugins instaláveis dinamicamente (ADR-0002), o usuário perde o controle mental do que tem instalado — mesmo corrigindo isso com manifesto+registro gerado (ADR-0001/0002), a listagem ainda precisa de alguma interface. O PRD já compromete toda a UX de seleção do resto do produto a componentes de TUI navegáveis por teclado (Seção 10).

## Decisão

`work plugin` sem subcomando abre uma TUI de gestão (telas e atalhos completos em `add-0001` §12). É estritamente uma camada de apresentação sobre os mesmos primitivos expostos como subcomandos diretos e scriptáveis (RF-16 a RF-21) — a TUI nunca mantém estado próprio que diverge do registro que esses comandos leem/escrevem, e nunca dispara checagem de atualização de rede automaticamente ao abrir, apenas via ação explícita do usuário dentro da tela — para não violar o espírito de RNF-4 mesmo que a regra não esteja escopada literalmente a `start`/`resume`/`archive`.

## Alternativas consideradas

* **Só TUI, sem subcomandos diretos.** Rejeitada: quebra casos de uso de scripting/CI/provisionamento de dotfiles (ex: reinstalar um conjunto usual de plugins em uma máquina nova).
* **Só subcomandos diretos, sem TUI.** Rejeitada: não resolve a dor de "não lembro o que tenho instalado" de forma tão direta quanto uma tela navegável única.

## Consequências

**Positivas:** um único paradigma de UX consistente em todo o produto (Seção 10); zero lógica de negócio nova — a TUI é uma visão pura sobre os comandos de RF-16 a RF-21; o caminho scriptável continua disponível para automação.

**Negativas / trade-offs:** código de apresentação a mais para construir e manter sem divergir para uma segunda fonte de verdade — mitigado pela restrição de sempre ler/escrever o mesmo registro que os comandos diretos usam.
