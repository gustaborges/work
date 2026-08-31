# ADR-0014: Separar Resolução de Origem da Localização Local do Repositório

**Status:** Aceito

**Data:** 2026-08-31

**Contexto de produto:** `docs/prd.md` — RF-2, RF-10, RF-39, RF-44 e RNF-8

**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seção 7

## Contexto

O contrato anterior exigia que cada Starter de `work start` produzisse um caminho local. Integrações de forge, issue tracker e outras origens passavam, assim, a conhecer a organização particular de clones na máquina do usuário. Essa organização varia por ambiente e pode usar scanning de filesystem, aliases, índices ou ferramentas locais.

## Decisão

Separar a interpretação da origem da localização física do clone. Um Starter resolve `argument → Repository Reference`, além de branch, modos de início, metadata e links que conhecer. Um componente executável de role `repository-locator` resolve `Repository Reference → candidatos de repositório Git local`.

Se a referência incluir `path`, o core valida o caminho diretamente e não executa Locators. O core continua a autoridade final para validar todo caminho usado no lifecycle Git. Locators não interpretam o argumento, não definem modo/base branch, não publicam metadata ou links, não criam worktrees e não escolhem candidatos.

## Consequências

Starters tornam-se portáveis entre organizações locais e Locators reutilizáveis entre origens, permitindo composição N + M em vez de N × M. O pipeline de início ganha uma fase explícita e o manifesto/protocolo ganham o role `repository-locator`.

## Alternativas rejeitadas

* Fazer cada Starter localizar seu clone: duplica lógica e acopla a integração ao ambiente.
* Implementar todas as estratégias no core: viola a simplicidade e extensibilidade do núcleo.
* Exigir uma ferramenta única de workspace: reduz portabilidade e introduz dependência externa.
