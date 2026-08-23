# PRD — Work

**Um orquestrador de contexto para desenvolvimento agêntico local.**

Status: Rascunho v1
Autor: Gustavo Carvalho
Data: 2026-08-23

***

## 1. Sumário Executivo

Work é um CLI/TUI que orquestra o início, a retomada e o encerramento de qualquer unidade de trabalho de desenvolvimento — uma feature nova, uma contribuição em pull request, o fork de um pull request de terceiros. Ele isola cada trabalho em uma Git Worktree própria, monta ao redor dela uma estrutura padronizada para artefatos de IA, rascunhos de especificação e notas de pesquisa, e entrega tudo isso pronto para uso — sem gastar raciocínio ou tokens de um agente de IA preparando o próprio ambiente, e sem poluir o repositório principal com esses artefatos. Fora o Git, o Work não embute conhecimento de nenhuma ferramenta específica: tudo que ele faz além disso é resolvido por meio de contratos com executáveis externos, plugáveis e mantidos pela comunidade.

***

## 2. Problema e Motivação

Hoje, quem trabalha com agentes de IA localmente — em múltiplos repositórios, múltiplas branches, múltiplos pull requests simultâneos — enfrenta um atrito repetitivo:

* **Contexto disperso.** Cada nova tarefa exige recriar manualmente uma worktree, entender qual branch de base usar, redescobrir a convenção de nomenclatura do projeto e recolher o contexto relevante (descrição de issue, diff de PR, comentários) espalhado entre GitHub, terminal e anotações soltas.
* **Poluição do repositório principal.** Artefatos de IA, rascunhos de spec e notas de pesquisa acabam commitados, ignorados via `.gitignore` de forma improvisada, ou perdidos entre branches — nunca têm um lugar próprio.
* **Custo de preparação pago em tokens de IA.** Sem uma estrutura determinística, é comum delegar ao próprio agente de IA a tarefa de descobrir o repositório certo, criar a worktree, entender a convenção de branch e organizar o ambiente — gastando raciocínio e tokens em trabalho mecânico e repetível.
* **Fragmentação entre fluxos de trabalho.** Iniciar um desenvolvimento novo, revisar um PR de terceiros e retomar um fork de PR são jornadas com formas de descoberta diferentes, mas hoje não compartilham nenhuma ferramenta ou convenção comum.

O Work resolve isso fornecendo um ponto de entrada único e determinístico para qualquer tipo de trabalho, delegando a parte específica de cada tipo de origem (issue, PR, repositório local, ou qualquer outra fonte futura) a plugins especializados — mantendo o núcleo do produto simples e estável.

***

## 3. Objetivos

* **Determinismo na preparação de ambiente.** Do comando inicial ao diretório de trabalho pronto, nenhuma etapa depende de um agente de IA "adivinhar" o que fazer.
* **Isolamento total via Git Worktrees.** Cada trabalho vive em sua própria worktree, sem interferir no checkout principal do repositório nem em outros trabalhos em andamento.
* **Um lar padronizado para artefatos de IA.** Toda unidade de trabalho ganha automaticamente uma estrutura própria para contexto, specs e notas — fora da árvore versionada do projeto de destino.
* **Extensibilidade total via plugins.** Qualquer nova origem de trabalho, qualquer nova forma de enriquecer o contexto, qualquer nova estratégia de branch deve poder ser adicionada por um plugin externo, sem alterar o núcleo do Work.
* **Um produto simples o suficiente para ser open source.** O núcleo deve ser pequeno, auditável e digno de publicação pública — não uma plataforma monolítica.

***

## 4. Não-Objetivos

* O Work **não é um agente de IA** e não toma decisões de desenvolvimento por conta própria — ele prepara o terreno para que um agente (ou uma pessoa) trabalhe.
* O Work **não hospeda nem gerencia repositórios** — ele opera sobre repositórios Git já existentes, locais ou remotos.
* O Work **não substitui o Git** — ele o invoca para orquestrar worktrees, mas todo o versionamento, merge e histórico continuam sendo responsabilidade do Git.
* O Work **não implementa, por si só, integrações específicas** (GitHub, GitLab, Jira, etc.) — essas integrações existem exclusivamente como plugins externos ao núcleo.
* O Work **não impõe uma convenção de branch única** — ele descobre e respeita a convenção de cada projeto.

***

## 5. Público-Alvo

* Desenvolvedores que trabalham com agentes de IA localmente e alternam com frequência entre múltiplas tarefas, repositórios e pull requests.
* Mantenedores e contribuidores de projetos open source que revisam ou assumem PRs de terceiros regularmente.
* Times que já adotam Git Worktrees como padrão de isolamento, mas hoje o fazem manualmente.
* Autores de ferramentas e plugins que queiram estender o Work para novas origens de trabalho (ex: Jira, Linear, GitLab).

***

## 6. Conceitos Centrais (Glossário)

| Termo               | Definição                                                                                                                                                                                                                                   |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Work**            | Uma unidade de trabalho: um desenvolvimento novo, uma contribuição em PR, um fork de PR — sempre associada a uma worktree, uma branch e uma pasta de contexto próprias.                                                                     |
| **Starter**         | Plugin externo responsável por interpretar o argumento de `work start` e resolver o caminho do repositório local correspondente. É a peça que sabe lidar com um tipo específico de origem (issue, link de PR, nome parcial de repositório). |
| **Contrato**        | O acordo de entrada/saída em JSON entre o Work e um executável externo (starter, importer, linker, script de descoberta de branch strategy). O Work não conhece a implementação do outro lado — apenas o contrato.                          |
| **Hook**            | Ponto de extensão acionado em um momento específico do ciclo de vida de um Work (ex: `onFinalize`), que dispara um ou mais executáveis externos (tipicamente importers).                                                                    |
| **Importer**        | Plugin que enriquece a pasta de contexto de um Work recém-criado — por exemplo, importando descrição, diff e comentários de um PR.                                                                                                          |
| **Linker**          | Plugin responsável por relacionar um Work a recursos externos (ex: vincular a issue original, a thread de discussão).                                                                                                                       |
| **Worktree**        | A árvore de trabalho Git isolada, criada pelo Work para cada unidade de trabalho, apontando para uma nova branch.                                                                                                                           |
| **Slug**            | Identificador curto e legível escolhido pelo usuário para nomear um Work, usado na branch e no nome da pasta.                                                                                                                               |
| **Branch Strategy** | A convenção de nomenclatura de branches do projeto de destino (ex: `gitflow`), descoberta automaticamente por um script externo.                                                                                                            |

***

## 7. Casos de Uso / Jornadas

### 7.1 Iniciar um desenvolvimento novo

O usuário quer começar uma feature nova em um repositório que já existe localmente. Ele roda `work start <nome parcial do repo>`, escolhe um slug e um prefixo de branch, e cai direto no diretório da nova worktree, pronto para abrir o agente de IA.

### 7.2 Contribuir em um pull request existente

O usuário recebe um link de pull request para revisar ou continuar diretamente sobre a branch já existente do PR. Ele roda `work start <link do PR>`. Como o mesmo link pode significar tanto uma contribuição quanto um fork, o Work pergunta ao usuário qual modo deseja sempre que o starter indicar suporte aos dois (RF-3). Ao escolher contribuição, a worktree faz checkout diretamente na branch do PR — sem criar uma nova branch — e a pasta de contexto já é enriquecida automaticamente com a descrição, o diff e os comentários do PR.

### 7.3 Fazer fork de um pull request de terceiros

O usuário quer assumir ou propor mudanças sobre um PR de outra pessoa a partir de uma branch própria. O reconhecimento do argumento e a escolha de modo seguem o mesmo fluxo do item anterior; ao optar por fork, o usuário escolhe slug e prefixo normalmente (como em 7.1), mas a base branch é resolvida automaticamente pelo starter — tipicamente a própria branch do PR.

### 7.4 Retomar um trabalho em andamento

Passado um tempo, o usuário quer voltar a um Work específico. Ele roda `work resume`, vê a lista de trabalhos ordenada do mais recente ao mais antigo, seleciona um e é levado diretamente ao diretório daquele Work.

### 7.5 Arquivar trabalhos concluídos

O usuário quer limpar sua área de trabalho. Ele roda `work archive`, seleciona (multi-seleção) os trabalhos concluídos, confirma, e o Work destrói as worktrees correspondentes e move o conteúdo para a área de arquivados — preservando todo o histórico de contexto.

***

## 8. Requisitos Funcionais

### `work start <arg>`

* **RF-1.** O comando deve aceitar um argumento genérico e percorrer os starters plugados, na ordem de prioridade configurada, até encontrar um que reconheça o argumento.
* **RF-2.** O starter escolhido deve processar o argumento e retornar o caminho do repositório local resolvido — essa é a capability obrigatória de todo starter.
* **RF-3.** Se o starter indicar que o argumento resolvido pode originar tanto uma contribuição quanto um fork, o Work deve perguntar ao usuário qual modo deseja antes de prosseguir. No modo contribuição, a worktree faz checkout diretamente na branch resolvida pelo starter, sem passar pelas etapas de slug e prefixo de branch (RF-4 e RF-5). No modo fork, o fluxo segue normalmente.
* **RF-4.** Fora do modo contribuição, o Work deve solicitar ao usuário um slug para o Work.
* **RF-5.** Fora do modo contribuição, o Work deve descobrir a estratégia de branch do projeto de destino delegando a descoberta a um script externo configurável, e apresentar ao usuário os prefixos de branch correspondentes para escolha.
* **RF-6.** Se o starter não fornecer a base branch, o Work deve perguntar ao usuário, via TUI, se deseja partir de uma branch remota ou local, apresentando a lista correspondente para seleção.
* **RF-7.** O Work deve inicializar a unidade de trabalho em uma pasta padronizada, contendo a worktree na branch resolvida (com o remote apontando para ela) e um arquivo de metadados do Work.
* **RF-8.** Ao final da inicialização, o Work deve disparar o hook de finalização definido pelo starter (quando existir), permitindo que importers enriqueçam a pasta de contexto do Work.
* **RF-9.** Ao concluir, o comando deve mudar o diretório ativo do terminal para o diretório recém-criado do Work.

### `work resume`

* **RF-10.** O comando deve listar todos os trabalhos existentes, ordenados do mais recentemente acessado ao mais antigo.
* **RF-11.** Ao selecionar um trabalho na TUI, o Work deve mudar o diretório ativo do terminal para o diretório daquele Work.

### `work archive`

* **RF-12.** O comando deve listar, com multi-seleção, todos os trabalhos ativos.
* **RF-13.** Após confirmação, o Work deve destruir as worktrees dos trabalhos selecionados e mover suas pastas para a área de arquivados, preservando todos os metadados e arquivos de contexto.
* **RF-14.** O estado persistido do Work deve ser atualizado para refletir o arquivamento.

### Extensibilidade

* **RF-15.** Todo comportamento além da orquestração de worktrees via Git deve ser delegado a executáveis externos plugáveis, descobertos e configurados pelo usuário — nunca embutidos no núcleo do Work.

***

## 9. Requisitos Não-Funcionais

* **RNF-1. Simplicidade radical do núcleo.** O Work deve permanecer pequeno e auditável; toda complexidade de domínio (interpretar issues, PRs, repositórios) vive fora do núcleo, em plugins.
* **RNF-2. Extensibilidade sem fricção.** Adicionar um novo starter, importer ou linker deve ser possível sem tocar no código do Work — apenas seguindo o contrato publicado.
* **RNF-3. Determinismo.** A preparação de um ambiente de trabalho não deve depender de inferência de um agente de IA; deve ser reprodutível e previsível.
* **RNF-4. Zero custo de raciocínio de IA na preparação.** Nenhuma etapa de `work start`, `work resume` ou `work archive` deve exigir que um LLM seja consultado.
* **RNF-5. Auditabilidade do estado.** Todo trabalho, ativo ou arquivado, deve ter seu estado e histórico rastreáveis localmente.
* **RNF-6. Portabilidade.** O Work deve funcionar de forma consistente entre diferentes sistemas operacionais suportados.
* **RNF-7. Confiança mínima necessária.** A superfície de código que o usuário precisa confiar sem revisão é a menor possível — o núcleo do Work — já que qualquer outro comportamento vem de plugins que o próprio usuário escolhe instalar.

***

## 10. Experiência do Usuário

Fluxo típico de `work start`:

1. Usuário roda `work start <argumento>` no terminal.
2. Work identifica silenciosamente qual starter reconhece o argumento e o invoca.
3. Work exibe o caminho do repositório resolvido e pede o slug do Work (prompt interativo).
4. Work descobre a estratégia de branch do projeto e apresenta a lista de prefixos disponíveis para escolha (seleção interativa).
5. Se necessário, Work pergunta a origem da base branch (remota ou local) e apresenta a lista correspondente para escolha.
6. Work exibe uma confirmação com o resumo do que será criado (repositório, branch, slug, localização).
7. Work cria a worktree, a estrutura de pastas e dispara os hooks de finalização — mostrando progresso em tempo real caso algum importer leve tempo para rodar.
8. Terminal já é reposicionado no novo diretório de trabalho, pronto para o usuário iniciar seu agente de IA com o contexto já preparado.

Toda interação de seleção (starters concorrentes, prefixos de branch, base branch, retomada, arquivamento) deve ser feita via componentes de TUI consistentes — navegação por teclado, sem exigir que o usuário memorize flags.

***

## 11. Arquitetura em Alto Nível (Visão de Produto)

O princípio de design central do Work é: **o núcleo só conhece contratos e executáveis externos — a única invocação direta e interna é o próprio Git.**

Essa escolha é deliberada, não incidental:

* **O core nunca quebra por causa de um plugin.** Como toda integração específica (GitHub, GitLab, Jira, convenções de branch de um projeto específico) roda como processo externo comunicando por um contrato JSON bem definido, um plugin com bug ou desatualizado não compromete a estabilidade do Work.
* **Manutenção comunitária natural.** Qualquer pessoa pode escrever, publicar e manter um starter, importer ou linker sem depender de um merge no repositório do Work — o mesmo padrão que tornou ecossistemas de plugins de outras ferramentas de linha de comando bem-sucedidos.
* **Superfície de confiança mínima.** Um usuário só precisa auditar o núcleo do Work uma vez; os plugins que instala são uma escolha explícita e isolada.
* **Sem vendor lock-in de IA.** O Work não sabe nada sobre qual agente de IA será usado depois — ele apenas entrega um ambiente pronto e sai de cena.

Os detalhes de implementação dessa arquitetura (linguagem, frameworks, formato exato dos contratos, protocolo de invocação dos scripts) estão descritos no **Apêndice A**, por serem decisões de "como" e não de produto.

***

## 12. Métricas de Sucesso

* **Tempo até ambiente pronto para IA:** tempo entre `work start <arg>` e o terminal reposicionado no diretório do Work, com contexto já importado.
* **Número de plugins criados pela comunidade** (starters, importers, linkers) além dos oferecidos por padrão.
* **Redução de artefatos de IA/spec commitados acidentalmente** no repositório principal dos projetos que adotam o Work.
* **Adoção:** número de instalações/estrelas no repositório público, uso recorrente medido por `work resume`.
* **Taxa de retomada de trabalho:** proporção de trabalhos iniciados que são retomados via `work resume` ao invés de recriados do zero.

***

## 13. Riscos e Mitigações

| Risco                                                                       | Mitigação                                                                                                                                                                           |
| --------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Plugin externo malicioso ou mal escrito compromete dados do usuário         | Contrato bem definido e documentado; execução isolada por processo; nenhuma permissão implícita concedida a um plugin além do que ele mesmo requisita ao rodar.                     |
| Ambiguidade entre múltiplos starters cujo padrão casa com o mesmo argumento | Ordem de prioridade explícita e configurável pelo usuário; primeiro starter que casa vence, de forma determinística.                                                                |
| Perda de contexto valioso ao arquivar um trabalho                           | Arquivamento preserva integralmente pastas de contexto e metadados, nunca descarta dados.                                                                                           |
| Divergência entre a branch strategy descoberta e a realidade do projeto     | Script de descoberta é substituível por projeto; usuário sempre revisa a lista de prefixos antes de confirmar.                                                                      |
| Núcleo crescer além do escopo por pressão de features                       | Qualquer nova capacidade que não seja orquestração de worktree é candidata a plugin, não a código do núcleo — princípio da Seção 11 tratado como restrição de design, não sugestão. |

***

## 14. Roadmap / Fora do Escopo da v1

**Fora do escopo da v1:**

* Starters, importers e linkers específicos além de um conjunto mínimo de referência (ex: um starter de fallback para repositório local).
* Suporte a múltiplas estratégias de branch simultâneas dentro do mesmo projeto.
* Sincronização de estado entre múltiplas máquinas do mesmo usuário.
* Interface gráfica (além da TUI).

**Candidatos a versões futuras:**

* Marketplace/catálogo de plugins da comunidade.
* Métricas e telemetria opcionais de uso local.
* Integração nativa com múltiplos provedores de forge (GitLab, Bitbucket) via starters oficiais adicionais.

***

## Apêndice A — Notas Técnicas e Stack

*Esta seção documenta decisões de implementação. Ela existe para dar direção técnica ao time, mas não é o contrato de produto — mudanças aqui não devem, por si só, exigir revisão do PRD.*

### A.1 Stack

* **Linguagem:** Go — binário único, portável, sem runtime externo.
* **CLI:** [Cobra](https://github.com/spf13/cobra) — framework padrão de mercado para CLIs Go, usado por `kubectl`, `gh`, `hugo`, entre outros.
* **TUI:** [Bubble Tea](https://github.com/charmbracelet/bubbletea) — framework de TUI idiomático em Go, para as telas interativas de seleção (starters concorrentes, prefixos de branch, base branch, `work resume`, `work archive`).
* **Persistência de estado:** SQLite embutido (via driver Go puro, sem dependência de CGO/lib externa), guardado em `~/.work/work.db`.
* **Única invocação direta de ferramenta externa dentro do núcleo:** o binário `git`, para criação/destruição de worktrees e resolução de branches remotas/locais.

### A.2 Layout de `~/.work`

```
~/.work/
  starters/
    github_issue/github_issue_starter.py
    github_pull_request/github_pull_request_starter.py
    default_starter.py
  linkers/
  importers/
  work.json
  work.start.json
  work.conventions.json
  work.db
```

Plugins podem ser escritos em qualquer linguagem — o contrato é via stdin/stdout JSON e código de saída do processo, não uma API de biblioteca Go. Isso é o que garante a "manutenção externa" mencionada na Seção 11.

### A.3 `work.json` — configuração geral

```json
{
  "workspace_dir": "~/workspace",
  "in-progress.directory.name": "in-progress",
  "review.directory.name": "reviews",
  "archived.directory.name": "archived"
}
```

Define a raiz onde os trabalhos são materializados (`workspace_dir`) e o nome das subpastas usadas por estado (em andamento, em revisão, arquivado).

### A.4 `work.start.json` — configuração do comando `start`

```json
{
  "version": "1.0",
  "starters": [
    {
      "name": "github_issue_starter.py",
      "description": "Can handle github issue links to initialize work",
      "enabled": true,
      "pattern": "^https://github\\.com/[^/]+/[^/]+/issues/\\d+$"
    },
    {
      "name": "github_pull_request_starter.py",
      "description": "Can handle github pull request links to initialize work",
      "enabled": true,
      "pattern": "^https://github\\.com/[^/]+/[^/]+/pull/\\d+$",
      "capabilities": [
        "generate_work_slug",
        "provide-base-branch",
        "disambiguate_contribution_fork"
      ],
      "hooks": {
        "onFinalize": ["importers.pr-metadata-importer"]
      }
    },
    {
      "name": "default_starter.py",
      "description": "Can locate a local repository based on plain text",
      "enabled": true
    }
  ]
}
```

**Decisões que fecham lacunas do rascunho original:**

* `pattern` ausente (ou vazio) é interpretado como **fallback universal** — casa com qualquer argumento. Só faz sentido para um starter no fim da lista de prioridade (ex: `default_starter.py`).
* A ordem do array `starters` **é** a ordem de prioridade de checagem — sem campo separado de prioridade.
* `pattern` evita instanciar/chamar cada script apenas para perguntar "você lida com isso?" — o Work casa o regex localmente e só invoca o processo do starter cujo padrão bateu.
* `capabilities` é uma lista aberta de strings; hoje o Work reconhece `generate_work_slug` (starter pode sugerir um slug, usuário ainda confirma/edita), `provide-base-branch` (starter resolve a base branch sozinho, pulando o RF-6) e `disambiguate_contribution_fork` (o argumento pode originar tanto uma contribuição quanto um fork — ex: um link de PR — e o Work deve perguntar ao usuário qual modo deseja antes de prosseguir, conforme RF-3). Novas capabilities podem ser adicionadas sem quebrar starters existentes — capability desconhecida é ignorada.
* `hooks.onFinalize` é uma lista de identificadores `<tipo>.<nome>` (ex: `importers.pr-metadata-importer`), resolvidos dentro das pastas plugáveis correspondentes (`~/.work/importers/`, etc.).

### A.5 `work.conventions.json` — convenções de gitflow/branch

```json
{
  "gitflow": {
    "prefixes": [
      { "name": "Feature", "prefix": "feature" },
      { "name": "Enhancement", "prefix": "enhancement" },
      { "name": "Bugfix", "prefix": "fix" },
      { "name": "Release", "prefix": "release" },
      { "name": "Hotfix", "prefix": "hotfix" }
    ]
  }
}
```

Suporta múltiplas estratégias nomeadas (`gitflow` é uma delas); o script de descoberta de branch strategy retorna a chave (`"gitflow"` ou uma chave customizada) usada para indexar este arquivo.

### A.6 Contrato de execução dos scripts externos

Todo executável externo (starter, importer, linker, script de descoberta de branch strategy) segue o mesmo protocolo:

* **Entrada:** um único payload JSON via stdin, contendo o argumento bruto e o contexto relevante (ex: caminho do repositório já resolvido, quando aplicável).
* **Saída:** um único payload JSON via stdout, no formato acordado por tipo de plugin (ex: um starter retorna `{ "repo_path": "...", "slug": "...", "base_branch": "..." }`, campos opcionais conforme as `capabilities` declaradas).
* **Código de saída:** `0` para sucesso; qualquer valor não-zero é tratado como "este plugin não conseguiu resolver o argumento" (para starters) ou como falha de hook (para importers/linkers), sem derrubar o comando do Work — apenas reportando o erro ao usuário.
* **Sem estado compartilhado implícito:** cada invocação é um processo isolado; nada é assumido sobre o ambiente além de variáveis padrão do shell do usuário.

### A.7 `work-meta.json`

Criado dentro de cada pasta de Work (`~/workspace/in-progress/<repo-name>_<branch-name>/work-meta.json`):

```json
{
  "slug": "meu-trabalho",
  "branch": "feature/meu-trabalho",
  "base_branch": "main",
  "starter": "github_pull_request_starter.py",
  "created_at": "2026-08-23T00:00:00Z",
  "last_accessed_at": "2026-08-23T00:00:00Z",
  "status": "in-progress"
}
```

### A.8 SQLite (`~/.work/work.db`)

Tabela principal `works`, espelhando `work-meta.json` de cada trabalho para permitir listagem rápida (ordenação por `last_accessed_at`) sem varrer o filesystem — usada por `work resume` (RF-10) e `work archive` (RF-12). `last_accessed_at` é atualizado toda vez que `work resume` seleciona aquele trabalho.

### A.9 Layout de diretórios de estado

```
~/workspace/
  in-progress/<repo-name>_<branch-name>/
    worktree/
    context/
    work-meta.json
  reviews/
  archived/<yyyymmdd>_<repo-name>_<branch-name>/
    worktree-removida (worktree é destruída; conteúdo de contexto preservado)
    context/
    work-meta.json
```

***

## Apêndice B — Perguntas em Aberto

Pontos assumidos como decisão de design razoável, mas ainda não validados na prática — candidatos a revisão em uma futura spec técnica (via `speckit-specify`):

1. **Mecanismo exato de descoberta da branch strategy.** O rascunho define que "um script externo descobre" a estratégia, mas não define o contrato de entrada desse script (provavelmente recebe `repo_path` e retorna a chave da estratégia) nem onde ele é configurado (dentro de `work.json`? Um novo `work.branch-strategy.json`?).
2. **Resolução de conflito quando múltiplos starters casam o mesmo argumento além do fallback.** Hoje a mitigação é "ordem de prioridade determinística", mas não há sinalização ao usuário de que outros starters também casariam.
3. **Formato de erro do usuário quando nenhum starter (além do fallback) resolve o argumento.** Deve cair silenciosamente no `default_starter.py`, ou alertar explicitamente que nenhum starter especializado reconheceu o argumento?
4. **Escopo exato de `reviews/`** — o rascunho original menciona `review.directory.name` em `work.json`, mas não descreve nenhum fluxo que popule essa pasta (nem `work start`, nem `work archive` a referenciam diretamente). Precisa de um comando próprio (`work review`?) ou é preenchida por um hook específico?
5. **Versionamento do contrato JSON entre Work e plugins.** `work.start.json` já tem um campo `version`; falta definir a política de compatibilidade quando o Work evolui o contrato e plugins antigos ainda o implementam na versão anterior.