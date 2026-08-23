# PRD — Work

**Um orquestrador de contexto para desenvolvimento agêntico local.**

Status: Rascunho v2
Autor: Gustavo Carvalho
Data: 2026-08-23
Supersede: `docs/prd.md` (v1)

***

## 1. Sumário Executivo

Work é um CLI/TUI que orquestra o início, a retomada e o encerramento de qualquer unidade de trabalho de desenvolvimento — uma feature nova, uma contribuição em pull request, o fork de um pull request de terceiros. Ele isola cada trabalho em uma Git Worktree própria, monta ao redor dela uma estrutura padronizada para artefatos de IA, rascunhos de especificação e notas de pesquisa, e entrega tudo isso pronto para uso — sem gastar raciocínio ou tokens de um agente de IA preparando o próprio ambiente, e sem poluir o repositório principal com esses artefatos. Fora o Git, o Work não embute conhecimento de nenhuma ferramenta específica: tudo que ele faz além disso é resolvido por meio de plugins externos, instaláveis e mantidos pela comunidade.

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
| **Plugin**          | Pacote instalável e distribuído externamente que cumpre um ou mais dos papéis abaixo (Starter, Importer, Linker, descoberta de convenção de branch). É instalado, atualizado e removido como uma unidade.                                  |
| **Starter**         | Papel que um plugin pode cumprir: interpretar o argumento de `work start` e resolver o caminho do repositório local correspondente. É a capability obrigatória de todo plugin que cumpre esse papel — é a peça que sabe lidar com um tipo específico de origem (issue, link de PR, nome parcial de repositório). |
| **Contrato**        | Acordo de entrada e saída publicamente documentado entre o Work e um plugin, que permite ao Work invocá-lo sem conhecer sua implementação.                                                                                                  |
| **Hook**            | Ponto de extensão acionado em um momento específico do ciclo de vida de um Work (ex: `onFinalize`), que aciona um ou mais plugins que cumprem o papel de Importer.                                                                          |
| **Importer**        | Papel que um plugin pode cumprir: enriquecer a pasta de contexto de um Work recém-criado — por exemplo, importando descrição, diff e comentários de um PR.                                                                                  |
| **Linker**          | Papel que um plugin pode cumprir: relacionar um Work a recursos externos (ex: vincular a issue original, a thread de discussão).                                                                                                            |
| **Worktree**        | A árvore de trabalho Git isolada, criada pelo Work para cada unidade de trabalho, apontando para uma nova branch.                                                                                                                           |
| **Slug**            | Identificador curto e legível escolhido pelo usuário para nomear um Work, usado na branch e no nome da pasta.                                                                                                                               |
| **Branch Strategy** | A convenção de nomenclatura de branches do projeto de destino (ex: `gitflow`), descoberta automaticamente por um plugin que cumpre esse papel.                                                                                              |

***

## 7. Casos de Uso / Jornadas

### 7.1 Iniciar um desenvolvimento novo

O usuário quer começar uma feature nova em um repositório que já existe localmente. Ele roda `work start <nome parcial do repo>`, escolhe um slug e um prefixo de branch, e cai direto no diretório da nova worktree, pronto para abrir o agente de IA.

### 7.2 Contribuir em um pull request existente

O usuário recebe um link de pull request para revisar ou continuar diretamente sobre a branch já existente do PR. Ele roda `work start <link do PR>`. Como o mesmo link pode significar tanto uma contribuição quanto um fork, o Work pergunta ao usuário qual modo deseja sempre que o plugin indicar suporte aos dois (RF-3). Ao escolher contribuição, a worktree faz checkout diretamente na branch do PR — sem criar uma nova branch — e a pasta de contexto já é enriquecida automaticamente com a descrição, o diff e os comentários do PR.

### 7.3 Fazer fork de um pull request de terceiros

O usuário quer assumir ou propor mudanças sobre um PR de outra pessoa a partir de uma branch própria. O reconhecimento do argumento e a escolha de modo seguem o mesmo fluxo do item anterior; ao optar por fork, o usuário escolhe slug e prefixo normalmente (como em 7.1), mas a base branch é resolvida automaticamente pelo plugin — tipicamente a própria branch do PR.

### 7.4 Retomar um trabalho em andamento

Passado um tempo, o usuário quer voltar a um Work específico. Ele roda `work resume`, vê a lista de trabalhos ordenada do mais recente ao mais antigo, seleciona um e é levado diretamente ao diretório daquele Work.

### 7.5 Arquivar trabalhos concluídos

O usuário quer limpar sua área de trabalho. Ele roda `work archive`, seleciona (multi-seleção) os trabalhos concluídos, confirma, e o Work destrói as worktrees correspondentes e move o conteúdo para a área de arquivados — preservando todo o histórico de contexto.

### 7.6 Instalar e gerenciar plugins

O usuário quer que o Work reconheça um novo tipo de origem — por exemplo, links do Jira interno da empresa. Ele aponta o Work para a origem desse plugin (um repositório ou um caminho local) com `work plugin install`, e a partir daí ele passa a estar disponível nos fluxos de `work start`/`resume`/`archive`, sem precisar editar nenhuma configuração à mão. Com `work plugin list` o usuário vê tudo que tem instalado; pode habilitar ou desabilitar um plugin sem removê-lo, atualizar quando o autor publicar uma nova versão, ou remover com `work plugin uninstall` quando não precisar mais dele.

***

## 8. Requisitos Funcionais

### `work start <arg>`

* **RF-1.** O comando deve identificar, entre os plugins instalados e habilitados capazes de iniciar um trabalho, qual deve processar o argumento informado — de forma determinística. Quando mais de um plugin for capaz de processar o mesmo argumento, o Work deve perguntar ao usuário qual utilizar e lembrar a escolha, de modo que a mesma situação de ambiguidade não precise ser perguntada novamente no futuro.
* **RF-2.** O plugin escolhido deve processar o argumento e retornar o caminho do repositório local resolvido — essa é a capability obrigatória de todo plugin que cumpre o papel de Starter.
* **RF-3.** Se o plugin indicar que o argumento resolvido pode originar tanto uma contribuição quanto um fork, o Work deve perguntar ao usuário qual modo deseja antes de prosseguir. No modo contribuição, a worktree faz checkout diretamente na branch resolvida pelo plugin, sem passar pelas etapas de slug e prefixo de branch (RF-4 e RF-5). No modo fork, o fluxo segue normalmente.
* **RF-4.** Fora do modo contribuição, o Work deve solicitar ao usuário um slug para o Work.
* **RF-5.** Fora do modo contribuição, o Work deve descobrir a estratégia de branch do projeto de destino delegando a descoberta a um plugin que cumpra esse papel, e apresentar ao usuário os prefixos de branch correspondentes para escolha.
* **RF-6.** Se o plugin não fornecer a base branch, o Work deve perguntar ao usuário, via TUI, se deseja partir de uma branch remota ou local, apresentando a lista correspondente para seleção.
* **RF-7.** O Work deve inicializar a unidade de trabalho em uma pasta padronizada, contendo a worktree na branch resolvida (com o remote apontando para ela) e um arquivo de metadados do Work.
* **RF-8.** Ao final da inicialização, o Work deve disparar o hook de finalização definido pelo plugin (quando existir), permitindo que plugins que cumprem o papel de Importer enriqueçam a pasta de contexto do Work.
* **RF-9.** Ao concluir, o comando deve mudar o diretório ativo do terminal para o diretório recém-criado do Work.
* **RF-10.** Na primeira utilização, o Work deve garantir que exista ao menos um plugin funcional capaz de iniciar um trabalho a partir de um repositório local — sem exigir que o usuário configure ou instale nada manualmente antes do primeiro uso.

### `work resume`

* **RF-11.** O comando deve listar todos os trabalhos existentes, ordenados do mais recentemente acessado ao mais antigo.
* **RF-12.** Ao selecionar um trabalho na TUI, o Work deve mudar o diretório ativo do terminal para o diretório daquele Work.

### `work archive`

* **RF-13.** O comando deve listar, com multi-seleção, todos os trabalhos ativos.
* **RF-14.** Após confirmação, o Work deve destruir as worktrees dos trabalhos selecionados e mover suas pastas para a área de arquivados, preservando todos os metadados e arquivos de contexto.
* **RF-15.** O estado persistido do Work deve ser atualizado para refletir o arquivamento.

### Gestão de Plugins

* **RF-16.** O usuário deve poder instalar um plugin a partir de uma origem remota ou de um caminho local.
* **RF-17.** O usuário deve poder listar todos os plugins instalados.
* **RF-18.** O usuário deve poder habilitar ou desabilitar um plugin instalado sem precisar desinstalá-lo.
* **RF-19.** O usuário deve poder verificar se há atualização disponível para um plugin e aplicá-la — sempre como ação explícita; `work start`, `work resume` e `work archive` nunca devem disparar essa verificação por conta própria.
* **RF-20.** O usuário deve poder desinstalar um plugin.
* **RF-21.** Se a instalação de um plugin resultar em conflito de nome com um plugin já instalado, o Work deve falhar de forma explícita e deixar o usuário escolher como resolver — nunca sobrescrever ou renomear automaticamente em silêncio.

### Extensibilidade

* **RF-22.** Todo comportamento além da orquestração de worktrees via Git deve ser delegado a plugins externos, descobertos e instalados pelo usuário — nunca embutido no núcleo do Work.

***

## 9. Requisitos Não-Funcionais

* **RNF-1. Simplicidade radical do núcleo.** O Work deve permanecer pequeno e auditável; toda complexidade de domínio (interpretar issues, PRs, repositórios) vive fora do núcleo, em plugins.
* **RNF-2. Extensibilidade sem fricção.** Adicionar um novo plugin deve ser possível sem tocar no código do Work.
* **RNF-3. Determinismo.** A preparação de um ambiente de trabalho não deve depender de inferência de um agente de IA; deve ser reprodutível e previsível.
* **RNF-4. Zero custo de raciocínio de IA na preparação.** Nenhuma etapa de `work start`, `work resume` ou `work archive` deve exigir que um LLM seja consultado.
* **RNF-5. Auditabilidade do estado.** Todo trabalho, ativo ou arquivado, deve ter seu estado e histórico rastreáveis localmente.
* **RNF-6. Portabilidade.** O Work deve funcionar de forma consistente entre diferentes sistemas operacionais suportados.
* **RNF-7. Confiança mínima necessária.** A superfície de código que o usuário precisa confiar sem revisão é a menor possível — o núcleo do Work. Instalar um plugin deve sempre exigir uma origem explicitamente escolhida pelo usuário; o Work nunca descobre ou instala um plugin por conta própria.

***

## 10. Experiência do Usuário

Fluxo típico de `work start`:

1. Usuário roda `work start <argumento>` no terminal.
2. Work identifica silenciosamente qual plugin reconhece o argumento e o invoca.
3. Work exibe o caminho do repositório resolvido e pede o slug do Work (prompt interativo).
4. Work descobre a estratégia de branch do projeto e apresenta a lista de prefixos disponíveis para escolha (seleção interativa).
5. Se necessário, Work pergunta a origem da base branch (remota ou local) e apresenta a lista correspondente para escolha.
6. Work exibe uma confirmação com o resumo do que será criado (repositório, branch, slug, localização).
7. Work cria a worktree, a estrutura de pastas e dispara os hooks de finalização — mostrando progresso em tempo real caso algum plugin leve tempo para rodar.
8. Terminal já é reposicionado no novo diretório de trabalho, pronto para o usuário iniciar seu agente de IA com o contexto já preparado.

Toda interação de seleção (plugins concorrentes, prefixos de branch, base branch, retomada, arquivamento) deve ser feita via componentes de TUI consistentes — navegação por teclado, sem exigir que o usuário memorize flags. A gestão de plugins (7.6) segue o mesmo paradigma consistente de TUI do restante do produto.

***

## 11. Métricas de Sucesso

* **Tempo até ambiente pronto para IA:** tempo entre `work start <arg>` e o terminal reposicionado no diretório do Work, com contexto já importado.
* **Número de plugins criados pela comunidade** além dos oferecidos por padrão.
* **Redução de artefatos de IA/spec commitados acidentalmente** no repositório principal dos projetos que adotam o Work.
* **Adoção:** número de instalações/estrelas no repositório público, uso recorrente medido por `work resume`.
* **Taxa de retomada de trabalho:** proporção de trabalhos iniciados que são retomados via `work resume` ao invés de recriados do zero.

***

## 12. Riscos e Mitigações

| Risco                                                                       | Mitigação                                                                                                                                                                           |
| --------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Plugin externo malicioso ou mal escrito compromete dados do usuário         | Instalação sempre a partir de uma origem explicitamente indicada pelo usuário, nunca de descoberta automática; nenhuma permissão implícita concedida a um plugin além do que ele mesmo requisita ao rodar; Work nunca muda o comportamento de um plugin já instalado sem ação explícita do usuário (RF-19). |
| Ambiguidade entre múltiplos plugins cujo padrão casa com o mesmo argumento  | Resolução determinística mesmo havendo mais de um plugin candidato: o usuário é consultado quando necessário e a escolha é lembrada, sem exigir configuração manual antecipada de prioridade (RF-1).                |
| Perda de contexto valioso ao arquivar um trabalho                          | Arquivamento preserva integralmente pastas de contexto e metadados, nunca descarta dados.                                                                                           |
| Divergência entre a branch strategy descoberta e a realidade do projeto     | Plugin de descoberta é substituível por projeto; usuário sempre revisa a lista de prefixos antes de confirmar.                                                                      |
| Núcleo crescer além do escopo por pressão de features                       | Qualquer nova capacidade que não seja orquestração de worktree é candidata a plugin, não a código do núcleo (RF-22) — tratado como restrição de design, não sugestão. |

***

## 13. Roadmap / Fora do Escopo da v1

**Fora do escopo da v1:**

* Plugins específicos além de um conjunto mínimo de referência (ex: um plugin de fallback para repositório local).
* Suporte a múltiplas estratégias de branch simultâneas dentro do mesmo projeto.
* Sincronização de estado entre múltiplas máquinas do mesmo usuário.
* Interface gráfica (além da TUI).

**Candidatos a versões futuras:**

* Métricas e telemetria opcionais de uso local.
* Integração nativa com múltiplos provedores de forge (GitLab, Bitbucket) via plugins oficiais adicionais.

***

## Apêndice A — Pontos em Aberto de Produto

1. **Escopo exato de um estado "em revisão".** Não está definido se existe uma jornada distinta de "em revisão", separada de "em andamento" e "arquivado", nem qual fluxo a populariam (um comando próprio, do tipo `work review`? Um hook específico disparado por outro comando?). Nenhum RF desta versão cobre esse estado — precisa de uma decisão de produto antes de entrar em uma versão futura.
