# PRD — Work

**Um orquestrador de contexto para desenvolvimento agêntico local.**

**Status:** Rascunho v2

**Autor:** Gustavo Carvalho

**Data:** 2026-08-29
**Supersede:** `docs/prd.md` (v1)

***

## 1. Sumário executivo

Work é um CLI/TUI que orquestra o início, a retomada e o encerramento de unidades de trabalho de desenvolvimento. Cada Work recebe uma Git Worktree isolada, um arquivo de metadados e uma área própria para artefatos que plugins podem acrescentar. O núcleo conhece Git e o lifecycle do Work; toda lógica específica de uma origem, integração ou enriquecimento é fornecida por plugins externos.

Além de resolver a origem de um Work, plugins podem publicar metadata e links, descobrir relações externas e importar artefatos. O Work avalia estaticamente quando cada extensão pode rodar, fornece somente os dados declarados por ela e preserva o controle sobre estado, lifecycle e arquivos do Work.

***

## 2. Problema e motivação

Quem trabalha com agentes de IA localmente, em múltiplos repositórios, branches e pull requests, repete trabalho mecânico: criar worktrees, encontrar a base correta, recolher contexto e manter artefatos fora da árvore versionada. Fluxos como iniciar uma feature, contribuir em um PR e retomar um trabalho têm origens diferentes, mas precisam do mesmo ambiente determinístico.

Work reduz esse atrito com um ponto de entrada único e extensível. Um plugin pode reconhecer um link de PR, informar a base e publicar o link correspondente; outros componentes do mesmo pacote podem descobrir relações complementares ou importar contexto sem que o núcleo passe a conhecer GitHub, GitLab, Jira ou qualquer integração específica.

***

## 3. Objetivos

* Preparar um ambiente de trabalho de forma determinística, sem depender de inferência de IA.
* Isolar todo Work em Git Worktree própria, sem interferir no checkout principal.
* Permitir que plugins acrescentem contexto e relações externas sem controlar o lifecycle do Work.
* Manter o núcleo pequeno, auditável e independente de integrações específicas.
* Oferecer extensibilidade por pacotes instaláveis, sem alterar o código do Work.

***

## 4. Não-objetivos

* Work não é um agente de IA nem toma decisões de desenvolvimento.
* Work não hospeda repositórios nem substitui Git.
* O núcleo não implementa integrações específicas de forge, issue tracker ou ferramenta externa.
* O staging de Importers não é sandbox de segurança: plugins continuam sendo processos executados com as permissões do usuário.
* Work não impõe uma convenção única de branch; a escolha continua sendo do usuário por repositório.

***

## 5. Público-alvo

* Desenvolvedores que alternam entre tarefas e repositórios usando agentes de IA locais.
* Mantenedores e contribuidores de projetos open source que revisam ou assumem PRs de terceiros.
* Times que usam Git Worktrees, mas hoje fazem sua preparação manualmente.
* Autores de plugins para novas origens, integrações e formas de enriquecer um Work.

***

## 6. Conceitos centrais

| Termo | Definição |
| --- | --- |
| **Work** | Unidade de trabalho associada a uma worktree, branch, diretório próprio e metadados locais. |
| **Plugin** | Pacote instalável que declara zero ou mais componentes executáveis e/ou convenções de branch. É instalado, atualizado, habilitado e removido como uma unidade. |
| **Componente** | Contribuição executável declarada em `components[]`. Seu `role` determina contrato, campos de manifesto e operações disponíveis. |
| **Starter** | Componente que resolve o argumento de `work start`. |
| **Importer** | Componente que produz arquivos e diretórios para incorporação ao Work. |
| **Linker** | Componente responsável por uma chave de link; pode descobrir seu valor e/ou permitir associação manual. |
| **Convenção de branch** | Contribuição declarativa em `conventions[]`, com nomes e prefixos; não é componente executável. |
| **Evento** | Ponto de lifecycle definido e publicado pelo Work, ao qual Importers e descobertas de Linkers podem se inscrever. |
| **Link** | Relação entre um Work e um recurso externo, identificada por chave namespaced, como `github.pull_request`. |
| **Metadata** | Informação conhecida sobre o Work ou publicada por componentes, separada do espaço de links. |
| **Input** | Dado que um componente declara querer receber, no formato `link:<key>` ou `meta:<key>`, opcionalmente com o sufixo `:optional`. |
| **Worktree** | Árvore de trabalho Git isolada criada para o Work. |
| **Slug** | Identificador curto e legível usado na branch e no nome do Work. |

***

## 7. Casos de uso / jornadas

### 7.1 Iniciar um desenvolvimento novo

O usuário roda `work start <nome parcial do repo>`. O Starter resolve o repositório; como não retorna `start_modes`, o Work cria um Work novo. O usuário escolhe slug, convenção e prefixo de branch e recebe uma worktree pronta.

### 7.2 Contribuir ou fazer fork de um pull request

O usuário roda `work start <link do PR>`. O Starter resolve o repositório, a base e retorna `start_modes: ["contribution", "fork"]`. O Work oferece esses modos. Em contribuição, faz checkout da branch resolvida sem pedir slug ou convenção; em fork, segue a jornada de branch própria. O Starter pode publicar o link do PR e metadata útil; em `start:finalized`, componentes elegíveis podem descobrir links e importar contexto.

### 7.3 Retomar e arquivar trabalhos

Com `work resume`, o usuário seleciona um Work em ordem de acesso recente. Com `work archive`, seleciona trabalhos ativos, confirma, e o Work destrói as worktrees e arquiva os demais arquivos e metadados.

### 7.4 Importar contexto manualmente

Dentro de um Work, o usuário roda `work import`, escolhe um Importer manual disponível e o Work executa-o somente se todos os inputs obrigatórios estiverem presentes. Os arquivos produzidos são incorporados após validação de colisões.

### 7.5 Associar um link manualmente

Dentro de um Work, o usuário roda `work link`, escolhe um Linker manual disponível e informa um valor. O Work persiste o valor na chave do Linker.

### 7.6 Gerenciar plugins e convenções

O usuário instala plugins de origem remota ou local, lista-os, habilita/desabilita, atualiza e remove pacotes. Pode também inspecionar ou trocar a convenção de branch memorizada para o repositório atual com `work convention` e `work convention set`.

***

## 8. Requisitos funcionais

### `work start <arg>`

* **RF-1.** O Work deve selecionar estaticamente, entre Starters habilitados, aquele cujo `pattern` reconhece o argumento; múltiplos matches devem ser apresentados ao usuário, sem memorização da escolha.
* **RF-2.** O Starter escolhido deve receber o argumento e devolver os dados que conseguiu resolver, incluindo obrigatoriamente o caminho do repositório local.
* **RF-3.** Quando a resposta do Starter contiver `start_modes`, o Work deve oferecer exatamente os modos retornados. A ausência desse campo significa criação de Work novo. No modo contribuição, o Work faz checkout da branch resolvida e não executa as etapas de slug, convenção e prefixo; no modo fork, o fluxo segue como Work novo.
* **RF-4.** Fora do modo contribuição, o Work deve solicitar um slug.
* **RF-5.** Fora do modo contribuição, o Work deve apresentar os prefixos da convenção de branch resolvida para escolha.
* **RF-6.** Se o Starter não fornecer base branch, o Work deve permitir selecionar uma branch remota ou local.
* **RF-7.** O Work deve criar a worktree, seu diretório e `work-meta.json`; plugins podem acrescentar outros arquivos e diretórios, que não são estrutura criada ou interpretada pelo núcleo.
* **RF-8.** Após materializar a estrutura e metadados de núcleo, o Work deve publicar `start:finalized`, executar Linkers e Importers inscritos e elegíveis na ordem definida pela arquitetura e mostrar o progresso.
* **RF-9.** Ao concluir, o comando deve reposicionar o terminal no diretório da worktree recém-criada.
* **RF-10.** No primeiro uso, o Work deve disponibilizar ao menos um Starter funcional para repositório local sem instalação manual.
* **RF-23.** No primeiro uso de um repositório fora do modo contribuição, se houver mais de uma convenção habilitada, o usuário deve escolher uma e a escolha deve ser memorizada para esse repositório.
* **RF-24.** No primeiro uso, o Work deve disponibilizar ao menos uma convenção de branch por padrão.

### Extensões do Work

* **RF-26.** Antes de executar uma operação de Importer ou descoberta de Linker, o Work deve avaliar estaticamente habilitação, forma de ativação, inscrição no evento, filtro de Starter e inputs obrigatórios; componente inelegível não deve ser executado.
* **RF-27.** O Work deve projetar ao subprocesso somente os inputs declarados pelo componente; inputs opcionais ausentes devem ser omitidos.
* **RF-28.** Para cada execução de Importer, o Work deve fornecer diretório temporário exclusivo, validar todas as colisões antes de alterar o Work e nunca sobrescrever arquivos silenciosamente.
* **RF-29.** Falha automática de Linker ou Importer após a criação do Work deve concluir `work start` com aviso e diagnóstico, sem desfazer o Work. Uma descoberta bem-sucedida sem valor não é erro.
* **RF-30.** Dentro de um Work, `work import` deve oferecer Importers que declaram `manual` e estejam elegíveis, e executar a operação `import` selecionada.
* **RF-31.** Dentro de um Work, `work link` deve oferecer Linkers que declaram `manual`, solicitar um valor não vazio e persistir esse valor sob a chave do Linker.
* **RF-32.** Links e metadata devem ser persistidos separadamente. Cada chave de link mantém um único valor; publicação pelo Starter, descoberta e associação manual fazem upsert, e a última origem vence.

### `work resume` e `work archive`

* **RF-11.** `work resume` deve listar trabalhos existentes do mais recentemente acessado ao mais antigo e abrir o selecionado.
* **RF-12.** A seleção em `work resume` deve atualizar o acesso recente e reposicionar o terminal na worktree.
* **RF-13.** `work archive` deve listar trabalhos ativos com multi-seleção.
* **RF-14.** Após confirmação, deve destruir as worktrees selecionadas e mover os demais arquivos para a área de arquivados.
* **RF-15.** O estado persistido deve refletir o arquivamento.

### Gestão de plugins e convenções

* **RF-16.** O usuário deve poder instalar plugin de origem remota ou caminho local.
* **RF-17.** O usuário deve poder listar plugins instalados.
* **RF-18.** O usuário deve poder habilitar ou desabilitar plugin sem desinstalá-lo.
* **RF-19.** O usuário deve poder verificar e aplicar atualização explicitamente; comandos de Work não verificam atualizações automaticamente.
* **RF-20.** O usuário deve poder desinstalar plugin.
* **RF-21.** Conflito de alias de plugin na instalação deve falhar explicitamente, sem sobrescrita ou renomeação silenciosa.
* **RF-25.** O usuário deve poder inspecionar e trocar a convenção de branch memorizada a partir de qualquer clone do repositório.

### Extensibilidade

* **RF-22.** Todo comportamento além da orquestração de worktrees via Git deve ser delegado a plugins externos, descobertos e instalados pelo usuário.

***

## 9. Requisitos não funcionais

* **RNF-1. Simplicidade radical do núcleo.** O Work permanece pequeno e auditável; lógica de domínio vive em plugins.
* **RNF-2. Extensibilidade sem fricção.** Novo plugin não exige mudança no código do Work.
* **RNF-3. Determinismo.** Preparação e elegibilidade são previsíveis e não dependem de IA.
* **RNF-4. Zero custo de raciocínio de IA.** Nenhuma jornada do CLI requer LLM.
* **RNF-5. Auditabilidade do estado.** Work, metadata, links e artefatos permanecem rastreáveis localmente.
* **RNF-6. Portabilidade.** O comportamento é consistente nos sistemas operacionais suportados.
* **RNF-7. Confiança mínima necessária.** A origem de plugin é escolhida explicitamente pelo usuário e o núcleo nunca carrega código de plugin no próprio processo.

***

## 10. Experiência do usuário

No fluxo de `work start`, o Work resolve e apresenta escolhas necessárias, cria o Work, persiste a resposta do Starter e executa extensões pós-criação. Progresso de Linkers e Importers é exibido; erros automáticos são reportados como avisos com diagnóstico, pois o Work já está disponível.

Todas as escolhas — Starters concorrentes, modo de início, convenção, prefixo, base branch, retomada, arquivamento, Importer e Linker manual — usam componentes de TUI consistentes e navegáveis por teclado. `work import` e `work link` são executados no Work atual; fora dele, falham com mensagem clara.

***

## 11. Métricas de sucesso

* Tempo entre `work start <arg>` e ambiente pronto, incluindo extensões pós-criação.
* Número de plugins criados pela comunidade.
* Redução de artefatos de IA/spec commitados acidentalmente.
* Uso recorrente de `work resume`.
* Taxa de extensões que se tornam elegíveis e concluem sem intervenção manual.

***

## 12. Riscos e mitigações

| Risco | Mitigação |
| --- | --- |
| Plugin malicioso ou defeituoso | Origem explícita, processos externos e nenhuma permissão adicional implícita. |
| Ambiguidade de Starter | Escolha explícita a cada colisão, sem prioridade oculta. |
| Importação destrutiva | Staging e detecção de todas as colisões antes da incorporação. |
| Extensão sem dados suficientes | Elegibilidade estática; o componente não é executado apenas para descobrir a ausência de input. |
| Perda de contexto no arquivamento | Arquivamento preserva metadata, links e artefatos; somente a worktree é destruída. |

***

## 13. Roadmap / fora do escopo da v1

**Fora do escopo da v1:** plugins de integração além do conjunto mínimo de referência; múltiplas convenções simultâneas por repositório; sincronização entre máquinas; interface gráfica; sandbox de segurança para plugins.

**Candidatos futuros:** telemetria local opcional; integrações oficiais de forge; componente executável para detectar automaticamente convenção de branch; novas operações e eventos de lifecycle definidos pelo núcleo.
