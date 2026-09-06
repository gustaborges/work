# Roadmap de Entregas Verticais — Work v1

**Status:** Proposta

**Data:** 2026-09-04

**Fontes:** `docs/prd.md`, `docs/add/add-0001-work-system-architecture.md` e ADRs aceitos

***

## 1. Objetivo

Este roadmap transforma o escopo da v1 em entregas verticais: cada fatia termina em uma jornada executável, demonstrável e útil para alguém. Uma fatia inclui o mínimo necessário de CLI/TUI, domínio, Git, persistência, contratos de plugin, mensagens de erro e testes; componentes técnicos isolados não são tratados como marcos de produto.

O roadmap define ordem e critérios de saída, não datas. Datas dependem de capacidade da equipe, cadência e nível de cobertura desejado. A sequência reduz risco primeiro no núcleo da proposta de valor — criar uma worktree determinística — e depois expande o mesmo fluxo para localização, extensões e operação completa.

***

## 2. Linha de entrega

| Fatia | Resultado utilizável | Marco |
| --- | --- | --- |
| F1 — Primeiro Work local | Criar e entrar em um Work a partir de um caminho Git local | Walking skeleton |
| F2 — Ciclo diário | Retomar e arquivar Works sem perder o snapshot | Alpha local |
| F3 — Encontrar o clone | Iniciar por nome ou referência usando roots e policy de Locators | Beta local |
| F4 — Novas origens por plugin | Instalar um plugin e iniciar Works por uma nova sintaxe/origem | Beta extensível |
| F5 — Contexto automático | Descobrir links e importar artefatos ao finalizar `work start` | v1 feature preview |
| F6 — Contexto sob demanda | Inspecionar links e executar Linkers/Importers manuais | v1 feature complete |
| F7 — Operação sustentável | Administrar plugins, Locators, policy e convenções com segurança | v1 release candidate |

As fatias são cumulativas. Nenhuma fatia pode quebrar as jornadas demonstráveis das anteriores.

***

## 3. Fatias verticais

### F1 — Primeiro Work local

**Resultado para o usuário:** `work start /caminho/do/repo` produz uma branch válida, uma worktree isolada e um snapshot canônico; ao final, o usuário consegue trabalhar no novo checkout.

**Demonstração de ponta a ponta:**

1. Em uma instalação limpa, executar `work start <path>`.
2. Definir a raiz de workspace no primeiro uso.
3. Escolher base branch, slug e o prefixo `freeform` fornecido pelo pacote de referência.
4. Ver o Work materializado em `in-progress`, com `worktree/` e `work-state.json` coerentes.
5. Confirmar que nome inválido ou branch incompatível permite correção sem deixar estado parcial.

**Inclui:** estrutura inicial do binário Go/Cobra; home TUI inicial em `work`; `work start [source]` com coleta interativa ou origem direta; configuração do workspace; pipeline mínimo de instalação e registro necessário ao bootstrap offline; pacote de referência com Starter local e convenção `freeform`; invocação do Starter por subprocesso; validação direta de `repository.path`; seleção de base/slug/prefixo; validação nativa de refs pelo Git; criação de branch e worktree; escrita atômica do snapshot; projeção inicial em SQLite; integração de shell necessária para posicionar o terminal.

**Critério de saída:** a jornada funciona em instalação limpa e é transacional até a materialização. Falhas antes da criação não deixam branch, diretório de Work, snapshot ou registro órfão. O estado criado pode ser reaberto e validado por teste de integração.

**Requisitos cobertos:** RF-2 (caminho direto), RF-4 a RF-7, RF-9, RF-10, RF-24, RF-33, RF-36, RF-37; base de RF-50 a RF-52 e de RNF-1, RNF-3, RNF-5, RNF-6, RNF-7 e RNF-10.

### F2 — Ciclo diário: retomar e arquivar

**Resultado para o usuário:** Works deixam de ser descartáveis; podem ser retomados por recência e arquivados preservando contexto.

**Demonstração de ponta a ponta:**

1. Criar dois Works com F1.
2. Executar `work resume`, selecionar o menos recente e confirmar atualização da ordem de acesso; repetir com `work resume <work>` sem seleção.
3. Executar `work archive`, selecionar um ou mais Works e confirmar a operação; repetir com alvos explícitos sem remover a confirmação.
4. Ver as worktrees removidas, os snapshots preservados em `archived` e o índice coerente.
5. Apagar ou corromper a projeção de teste e reconstruí-la a partir dos snapshots.

**Inclui:** expansão da home TUI; listagem por `last_accessed_at`; seleção TUI e alvos diretos; resolução segura do Work atual; atualização atômica de acesso; multi-seleção e confirmação de archive; remoção da worktree via Git; movimentação dos demais arquivos; reconciliação/rebuild de `work.db`.

**Critério de saída:** criar → retomar → arquivar é uma jornada repetível, e a perda da projeção SQLite não implica perda do estado canônico.

**Requisitos cobertos:** RF-11 a RF-15 e RF-34.

**Marco:** ao final de F2 existe um **alpha local** útil sem plugins de terceiros.

### F3 — Encontrar o clone local

**Resultado para o usuário:** o caminho absoluto deixa de ser necessário; `work start <nome-ou-referência>` encontra um clone nas raízes configuradas segundo uma policy explícita.

**Demonstração de ponta a ponta:**

1. Configurar duas raízes de busca separadas do workspace.
2. Iniciar um Work usando apenas o nome de um repositório.
3. Observar o filesystem Locator padrão encontrar um único clone e concluir o fluxo F1.
4. Repetir com dois matches e escolher explicitamente o repositório correto.
5. Demonstrar `sem resultado`, `candidato inválido` e `falha do Locator` como resultados distintos.

**Inclui:** Repository Reference transitória; contrato e invocação de Repository Locator; roots configuráveis; policy ordenada mínima; avaliação de elegibilidade por `accepts`; projeção apenas dos campos aceitos; validação e deduplicação de candidatos; short-circuit; seleção de ambiguidade; filesystem Locator offline do pacote de referência; hub TUI `work repository`; comandos não interativos mínimos `work repository locator list`, `work repository policy list|add|remove|move|replace` e `work repository root list|add|remove|replace`.

**Critério de saída:** caminho direto e localização por policy convergem para a mesma jornada de criação, sem heurística escondida e sem usar as worktrees do próprio Work como catálogo implícito.

**Requisitos cobertos:** RF-39, RF-42 a RF-44, RF-46 e RF-49; primeira entrega de RF-40 e RF-45; RNF-8 e RNF-9.

**Marco:** ao final de F3 existe um **beta local** que atende o início de trabalho sem exigir que o usuário saiba o caminho do clone.

### F4 — Novas origens por plugin

**Resultado para o usuário:** uma origem não conhecida pelo core pode ser instalada e usada para criar um Work completo, inclusive nos modos contribuição e fork.

**Demonstração de ponta a ponta:**

1. Instalar um plugin-fixture local com `work plugin install <path> --link` e um pacote proveniente de repositório remoto com `work plugin install <source>`.
2. Executar `work start <arg-específico>` e selecionar o Starter quando dois patterns colidirem.
3. Resolver a Repository Reference pelo pipeline de F3.
4. Escolher `contribution` e criar checkout da branch resolvida sem slug/convenção.
5. Repetir em `fork`, escolher convenção/prefixo e criar branch própria.
6. Listar a origem e a referência fixada do plugin instalado.

**Inclui:** manifesto completo e validação por role; instalação local fixada ou vinculada e instalação remota com referência fixada; alias e colisão; registro gerado; runtime/entrypoint portável; matching de Starter e fallback; colisão sem prioridade oculta; protocolo de saída do Starter; `start_modes`; base branch fornecida ou selecionada; catálogo de convenções; escolha e memorização por identidade de repositório; hub `work convention` e comandos `work convention show|set`.

**Critério de saída:** um plugin de teste escrito fora do core adiciona uma nova origem sem mudança no binário, e todos os caminhos de sucesso chegam às mesmas garantias de worktree e snapshot de F1.

**Requisitos cobertos:** RF-1, RF-3, RF-6, RF-16, RF-17, RF-21 a RF-23 e RF-25; completa a separação exigida por RF-2; RNF-2.

**Marco:** ao final de F4 existe um **beta extensível** e o principal pressuposto arquitetural — domínio fora do core — está provado por integração.

### F5 — Contexto automático no início

**Resultado para o usuário:** um Work criado por uma origem pode nascer com links e artefatos úteis, sem passos manuais e sem permitir que uma extensão corrompa o Work já criado.

**Demonstração de ponta a ponta:**

1. Um Starter publica metadata e um link no snapshot inicial.
2. Em `start:finalized`, um Linker elegível descobre outro valor e o core faz upsert.
3. Um Importer consome o link recém-descoberto, escreve em staging e incorpora os artefatos sem colisão.
4. Um segundo Importer tenta colidir com um arquivo existente e nenhuma parte de sua saída é incorporada.
5. Uma extensão automática falha; `work start` termina com aviso, mantendo o Work utilizável.

**Inclui:** seções `meta` e `links`; Semantic Conventions e namespace privado; evento `start:finalized`; filtro por Starter; resolução de inputs obrigatórios/opcionais; projeção mínima de dados; fases Linker → persistência → Importer; upsert e proveniência; staging exclusivo; preflight de todas as colisões; limpeza; diagnósticos e semântica de falha não destrutiva.

**Critério de saída:** o pipeline automático é determinístico e testado tanto em sucesso quanto em falha. Plugins recebem somente os inputs declarados, nunca escrevem diretamente no snapshot, e nenhum Importer pode produzir incorporação parcial.

**Requisitos cobertos:** RF-8, RF-26 a RF-29, RF-32 e RF-35.

### F6 — Contexto sob demanda

**Resultado para o usuário:** depois da criação, é possível consultar relações e acrescentar contexto manualmente ao Work atual.

**Demonstração de ponta a ponta:**

1. Dentro de um Work, executar `work status` e apresentar estado e links sem alterar snapshot, acesso recente ou proveniência; repetir fora dele com alvo explícito.
2. Executar `work link`, selecionar apenas Linkers manuais elegíveis e associar um valor não vazio.
3. Executar `work import`, selecionar apenas Importers manuais elegíveis e incorporar seus artefatos pelo mesmo staging de F5.
4. Executar os três comandos fora de um Work e receber erro acionável sem alteração de estado.

**Inclui:** detecção do Work atual e resolução de alvo explícito; apresentação somente de operações disponíveis e elegíveis; associação manual sem subprocesso; importação manual reutilizando elegibilidade/staging; `status` estritamente read-only e com `--json`; mensagens para ausência de candidatos e inputs.

**Critério de saída:** os fluxos manuais reutilizam os mesmos contratos e garantias do fluxo automático e não criam um segundo modelo de estado ou execução.

**Requisitos cobertos:** RF-30, RF-31 e RF-38.

**Marco:** ao final de F6 a v1 está **feature complete** nas jornadas de Work e de enriquecimento.

### F7 — Operação sustentável da plataforma

**Resultado para o usuário:** o ecossistema pode evoluir sem que instalação, atualização, desabilitação ou remoção altere silenciosamente a resolução de repositórios.

**Demonstração de ponta a ponta:**

1. Habilitar e desabilitar um plugin, preservando a referência indisponível de seu Locator na policy.
2. Instalar um novo Locator e confirmar que ele não entra automaticamente na policy.
3. Adicionar, remover e reordenar Locators por comandos e pela TUI, com a mesma configuração efetiva.
4. Executar `work plugin update --check` sem mudar a instalação e aplicar `work plugin update <plugin>` ou `work plugin update --all` explicitamente após validação do novo manifesto.
5. Tentar remover um plugin referenciado pela policy e exigir tratamento explícito da dependência.

**Inclui:** `plugin list|install|enable|disable|update|uninstall`; checagem por `update --check`; uninstall consistente; estados de Locator instalado, habilitado, na policy e indisponível; operações completas de policy e roots; paridade entre TUI e comandos não interativos; confirmação interativa e flags explícitas para automação; validação transacional de registro/configuração.

**Critério de saída:** toda mutação administrativa é explícita, inspecionável e recuperável por reinstalação/configuração; nenhuma instalação muda precedência e nenhuma remoção deixa referência silenciosamente quebrada.

**Requisitos cobertos:** RF-18 a RF-20, RF-40, RF-41, RF-45, RF-47 e RF-48.

**Marco:** ao final de F7 existe um **release candidate da v1**.

***

## 4. Gates transversais

Estes itens não formam uma fatia horizontal separada. Entram no critério de pronto de toda fatia que tocar a superfície correspondente:

* **Determinismo:** sem prioridade, fallback ou mutação de policy implícitos.
* **Integridade:** operações que alteram snapshot, índice, filesystem e Git têm testes de falha e não deixam estado parcial evitável.
* **Contrato de processo:** stdin/stdout, códigos de saída, stderr e respostas inválidas têm testes de contrato com fixtures autocontidas.
* **Portabilidade:** paths, execução de runtimes, rename atômico e integração de shell são testados nos sistemas operacionais oficialmente suportados.
* **Auditabilidade:** decisões e diagnósticos importantes identificam Work, componente e operação sem gravar segredos ou conteúdo desnecessário.
* **UX:** toda jornada é alcançável pela home TUI; hubs mostram o comando direto equivalente; caminhos interativos são navegáveis por teclado; comandos destinados à automação possuem `--json` nas leituras, saída e exit codes estáveis e nunca tentam abrir TUI.
* **Regressão:** a demonstração automatizada de cada fatia anterior permanece verde.

O release candidate passa ainda por instalação limpa, upgrade entre versões suportadas, interrupção/cancelamento nos pontos de mutação, reindexação a partir de snapshots e testes com repositórios Git reais contendo branches locais/remotas ambíguas.

***

## 5. Decisões necessárias antes de iniciar F1

Três contratos precisam ser fechados no plano de implementação da primeira fatia:

1. **Integração de shell para RF-9 e RF-12.** Um processo filho não altera o diretório do shell pai. É necessário escolher e documentar o contrato, por exemplo função de shell, comando de inicialização que emite `cd`, ou subshell administrado.
2. **Matriz de sistemas operacionais e shells suportados.** RNF-6 exige portabilidade, mas o conjunto suportado precisa ser explícito para definir CI e comportamento de paths, symlinks e execução.
3. **Formato autocontido do pacote de referência.** RF-10 proíbe dependência externa no primeiro uso; portanto, seus executáveis não podem pressupor um runtime ausente. O empacotamento precisa garantir essa propriedade em todas as plataformas suportadas.

Antes de F5 também deve existir uma primeira versão publicada das Semantic Conventions usadas pelos plugins-fixture; sem isso, a validação de chaves públicas fica subespecificada.

***

## 6. O que não deve antecipar a v1

Assinatura formal de plugins, telemetria, sincronização entre máquinas, interface gráfica, streaming de progresso, timeout imposto pelo core, detecção automática de convenção e integrações oficiais adicionais continuam fora destas fatias. Fixtures de integração podem simular GitHub, GitLab ou um issue tracker, mas não transformam uma integração oficial em dependência para provar os contratos do core.

***

## 7. Próximo passo

Detalhar somente F1 em plano de implementação e tarefas pequenas, mantendo F2–F7 como outcomes. Ao final de F1, revisar o roadmap com dados reais de complexidade, portabilidade e transações Git/filesystem antes de estimar datas para os marcos seguintes.
