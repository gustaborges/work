# ADR-0012: Modelo de Componentes e Manifesto de Plugins

**Status:** Aceito

**Data:** 2026-08-29

**Contexto de produto:** `docs/prd.md` — RF-1, RF-2, RF-8, RF-22, RF-26 a RF-32, RNF-1, RNF-2, RNF-3 e RNF-7

**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seções 4, 5, 7 e 9 a 11

## Contexto

O modelo anterior misturava papel de componente, capacidades abertas e hooks definidos por Starter. Isso deixa a elegibilidade de extensões dependente de convenções pouco explícitas e permite que o plugin descreva fluxo de controle que pertence ao Work. Também não separa com precisão a necessidade de dados de uma extensão da forma pela qual ela foi ativada.

O produto precisa que um pacote possa declarar Starters, Repository Locators, Importers, Linkers e convenções de branch sem que o núcleo conheça a integração. Um pacote também precisa ser uma unidade coerente de instalação e manutenção: uma integração pode reunir componentes que fazem sentido apenas juntos. Ao mesmo tempo, o núcleo deve decidir, sem executar código, se uma operação pode rodar, quais dados ela recebe e quando seus efeitos podem ser incorporados.

## Decisão

Um plugin é um pacote versionado e a unidade atômica de instalação, atualização, habilitação e remoção. Seu `plugin.json`, lido pelo Work no pipeline de instalação, declara zero ou mais componentes executáveis em `components[]` e zero ou mais convenções declarativas em `conventions[]`. Cada componente tem `role` obrigatório: `starter`, `repository-locator`, `importer` ou `linker`. `role` é o discriminador semântico: determina o contrato, os campos válidos do manifesto e as operações que o Work pode solicitar. Não existe campo `invocation` ou equivalente; protocolo e forma de ativação não são identidade do componente.

A identidade de componente é seu `name` lógico, nunca o arquivo que o implementa. O `name` do pacote é uma proposta de alias local: o registro instalado garante unicidade para o usuário e, quando nomes de componente colidem entre pacotes habilitados, o Work os qualifica como `<alias>/<nome>`.

`conventions[]` permanece separado de `components[]`: uma convenção é uma contribuição estática, sem `role`, `entrypoint` ou `runtime`. Sem detecção dinâmica na v1, executar um processo para obter um catálogo já conhecido no manifesto só acrescentaria runtime, preflight e caminhos condicionais ao núcleo.

O manifesto descreve estaticamente o que o componente é, as operações que oferece, pontos de ativação e dados requeridos. O Work controla lifecycle, seleção de Starter, publicação de eventos, elegibilidade, projeção de inputs, subprocessos, persistência de metadata/links e incorporação de arquivos. Plugins implementam comportamento de domínio, não o fluxo de controle do Work.

Repository Locators declaram `accepts` não vazio com os campos de Repository Reference que sabem consumir. O core os considera apenas quando estão na Repository Resolution Policy, o plugin está habilitado e algum campo aceito está presente; a policy e as raízes de busca são controladas pelo usuário conforme ADR-0015. Importers declaram eventos em `on`, disponibilidade manual em `manual` e inputs em `inputs`. Linkers declaram uma `key`, descoberta opcional em `discover` e associação manual opcional em `manual`. Eventos são definidos pelo core; `starters` é filtro de ativação, nunca input. Antes de qualquer subprocesso, o core avalia a elegibilidade estaticamente e não executa componente apenas para descobrir se ele deveria rodar.

Links e metadata são espaços distintos. Inputs usam `link:<key>` ou `meta:<key>`, com `:optional` quando aplicável. Links possuem um valor por chave e são atualizados por upsert: a última origem vence. Importers escrevem em diretório temporário exclusivo e seus artefatos só são incorporados depois de validação completa de colisões.

O evento v1 `start:finalized` é processado em fases: persistir dados do Starter, executar/persistir Linkers elegíveis e, então, executar Importers elegíveis. Não há ordem garantida entre componentes da mesma fase.

## Alternativas consideradas

* **Manter `capabilities` e hooks comandados pelo Starter.** Rejeitada: capacidade aberta não descreve com precisão operação, ativação e inputs, e hooks no Starter acoplam indevidamente extensões ao iniciador que as disparou.
* **Executar componentes para decidir elegibilidade.** Rejeitada: transforma ausência comum de dados em subprocesso desnecessário, torna o lifecycle menos previsível e introduz efeitos antes de o core decidir que a operação é válida.
* **Receber o diretório real do Work no Importer.** Rejeitada: impede validação integral de colisões e permite alteração parcial antes de o core controlar a incorporação.
* **Modelar convenções como quarto role.** Rejeitada: convenções não são executáveis e não precisam de runtime, entrada ou processo; separar os dois arrays torna essa diferença visível no schema, em vez de espalhar condicionais por manifesto, registro e execução.
* **Um pacote por componente.** Rejeitada: força contribuições naturalmente acopladas, como reconhecer e importar dados de um pull request, a serem versionadas e instaladas separadamente.

## Consequências

**Positivas:** o core decide operações de modo previsível e auditável; contratos são mínimos por role; dados de integração são preservados sem ampliar o modelo do núcleo; Importers não sobrescrevem arquivos do Work silenciosamente.

**Trade-offs:** o manifesto fica mais expressivo e requer validação discriminada por role; a resolução de repositório ganha uma policy explícita; extensões automáticas podem falhar depois da criação do Work, portanto o Work conclui com aviso em vez de desfazer um ambiente já materializado.

## Relação com decisões anteriores

Esta ADR consolida as decisões de pacote/manifesto e convenções declarativas antes registradas separadamente. Ela substitui o modelo que usava `type`, `capabilities` ou `hooks`, bem como o contrato de hook anterior. ADRs 0014 a 0016 estendem este modelo com o role `repository-locator` e seu contrato. As decisões de subprocessos externos, runtime explícito e colisão de Starters continuam em ADRs próprios.
