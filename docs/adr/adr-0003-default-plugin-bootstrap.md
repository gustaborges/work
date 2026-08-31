# ADR-0003: Bootstrap do Pacote de Plugin de Referência no Primeiro Uso

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs/prd.md` — RF-10, RF-24, Seção 2 (motivação), RNF-1, RNF-7
**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seção 6

## Contexto

Um núcleo 100% vazio faz `work start` "de fábrica" não fazer nada, contradizendo a própria proposta de valor do produto (eliminar atrito de preparação — PRD Seção 2). Embutir a lógica de resolução padrão como código do núcleo violaria diretamente RNF-1/RNF-7 e o princípio de execução por processo externo (ADR-0000) — passaria a existir comportamento de domínio vivendo dentro da superfície de confiança mínima, sem poder ser desinstalado/substituído.

## Decisão

Um pacote oficial de referência contém, no mínimo, um Starter fallback para referências a repositório local, um Repository Locator baseado em filesystem e uma convenção de branch `freeform` (ADRs 0012, 0014 e 0015). Ele é instalado automaticamente no primeiro uso, usando **exatamente o mesmo mecanismo** de instalação de qualquer plugin de terceiro (ADR-0002) — nunca como código embutido no núcleo. Para não depender de rede, o pacote vai embutido nos assets do binário/release como *seed*, mas ainda é "instalado" através do pipeline normal (grava manifesto, registro e entrypoint — ver `add-0001` §6); o Locator padrão entra inicialmente na policy global. O usuário pode desinstalar esse pacote como qualquer outro, e ele desaparece de fato, sem deixar comportamento fantasma no núcleo.

## Alternativas consideradas

* **A — núcleo 100% vazio.** Máxima pureza em relação a RNF-1/RNF-7, mas onboarding péssimo — rejeitada.
* **B — lógica default embutida no núcleo.** Zero fricção, mas viola ADR-0000/RNF-1/RNF-7 diretamente: comportamento de domínio vivendo na superfície que o usuário precisa confiar sem revisão, não auditável, não desinstalável — rejeitada.

## Consequências

**Positivas:** onboarding funciona imediatamente sem violar a arquitetura; também serve como *dogfooding* do próprio contrato de manifesto/registro.

**Negativas / trade-offs:** aumenta o tamanho do binário/release e exige manter o seed compatível com o contrato de plugin — custo aceito para garantir primeiro uso funcional, offline e sem lógica de domínio embutida no núcleo.
