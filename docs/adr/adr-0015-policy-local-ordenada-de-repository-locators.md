# ADR-0015: Policy Local Ordenada de Repository Locators

**Status:** Aceito

**Data:** 2026-08-31

**Contexto de produto:** `docs/prd.md` — RF-40 a RF-48 e RNF-9

**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seção 7.2

## Contexto

Uma máquina pode ter vários mecanismos de localização, como aliases, índice corporativo e filesystem. A preferência entre esses mecanismos é uma propriedade do ambiente do usuário; prioridade no manifesto, ordem de instalação ou agregação de resultados introduziriam precedência implícita e arbitragem sem autoridade comum.

## Decisão

Manter uma Repository Resolution Policy global, declarativa e ordenada, formada por referências qualificadas `<alias>/<component>`. A resolução percorre a policy por Chain of Responsibility: Locator inelegível é ignorado; zero matches avança; um match válido conclui; múltiplos matches válidos são escolhidos pelo usuário e encerram a chain; falha operacional interrompe a resolução. A v1 não agrega resultados, não possui scores, nem `continue_on_locator_error`.

Instalar ou habilitar plugin não modifica a policy. Remover um Locator da policy somente deixa de usá-lo; não cria um estado individual de habilitação. Desabilitar o plugin torna seus Locators indisponíveis, mas preserva suas posições na policy. Desinstalar plugin remove, mediante confirmação explícita, as referências afetadas na mesma alteração consistente.

A TUI e os comandos diretos são agrupados em `work repository`: `policy` gerencia a sequência, `locator list` mostra Locators instalados e `roots` administra raízes de busca.

## Consequências

O comportamento é determinístico, auditável e não muda silenciosamente quando plugins são instalados. Em contrapartida, o usuário precisa incluir e ordenar Locators de propósito, e uma falha de Locator configurado interrompe a resolução na v1.

## Alternativas rejeitadas

* Prioridade numérica no manifesto ou ordem de instalação: seriam implícitas e instáveis.
* Plugin oficial sempre em posição fixa: a preferência pertence ao usuário.
* Executar todos e agregar resultados: exige arbitragem, aumenta custo e torna o resultado sensível aos plugins instalados.
* Policies por Starter: recriam acoplamento entre origem e localização na v1.
