# PRD — Work

**Um orquestrador de contexto para desenvolvimento agêntico local.**

**Status:** Rascunho v3

**Autor:** Gustavo Carvalho

**Data:** 2026-09-07
**Supersede:** `docs/prd.md` (v2)

***

## 1. Sumário executivo

Work é um CLI/TUI que orquestra o início, a retomada e o encerramento de unidades de trabalho de desenvolvimento. Cada Work recebe uma Git Worktree isolada, um snapshot de estado local e uma área própria para artefatos que plugins podem acrescentar. O núcleo conhece Git e o lifecycle do Work; toda lógica específica de uma origem, integração ou enriquecimento é fornecida por plugins externos.

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
* Permitir que qualquer jornada seja descoberta por uma entrada de marca e uma ajuda completa, sem exigir memorização profunda da CLI.

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
| **Work** | Unidade de trabalho associada a uma worktree, branch, diretório próprio e snapshot local de estado. |
| **Plugin** | Pacote instalável que declara zero ou mais componentes executáveis e/ou convenções de branch. É instalado, atualizado, habilitado e removido como uma unidade. |
| **Componente** | Contribuição executável declarada em `components[]`. Seu `role` determina contrato, campos de manifesto e operações disponíveis. |
| **Starter** | Componente que interpreta o argumento de `work start`, resolve a Repository Reference e os demais dados de início que conhecer. |
| **Importer** | Componente que produz arquivos e diretórios para incorporação ao Work. |
| **Linker** | Componente responsável por uma chave de link; pode descobrir seu valor e/ou permitir associação manual. |
| **Repository Reference** | Descrição transitória do repositório resolvido por um Starter: pode conter um caminho local ou dados que permitam localizá-lo. |
| **Repository Locator** | Componente que tenta resolver uma Repository Reference para um ou mais repositórios Git locais, sem interpretar a origem do Work nem definir seu lifecycle. |
| **Repository Resolution Policy** | Sequência global e ordenada de Repository Locators escolhida pelo usuário para a máquina atual. |
| **Repository Search Root** | Diretório configurado no qual estratégias de descoberta podem procurar clones; é distinto da raiz de materialização dos Works. |
| **Convenção de branch** | Contribuição declarativa em `conventions[]`, com nomes e prefixos; não é componente executável. |
| **Evento** | Ponto de lifecycle definido e publicado pelo Work, ao qual Importers e descobertas de Linkers podem se inscrever. |
| **Estado `work`** | Estado de domínio governado e versionado pelo core, exposto seletivamente como `work:<key>`. |
| **Link** | Relação externa de primeira classe entre um Work e um recurso, identificada por chave semântica, como `github.pull_request`. |
| **Metadata** | Informação extensível publicada por componentes, distinta do estado do Work e dos links. |
| **Input** | Dado que um componente declara querer receber, no formato `work:<key>`, `link:<key>` ou `meta:<key>`, opcionalmente com o sufixo `:optional`. |
| **Semantic Convention** | Contrato público que define namespace, chave, significado e representação de um `link` ou `meta` interoperável. |
| **Worktree** | Árvore de trabalho Git isolada criada para o Work. |
| **Slug** | Identificador curto e legível usado na branch e no nome do Work. |

***

## 7. Casos de uso / jornadas

### 7.1 Iniciar um desenvolvimento novo

#### Primeiro uso e raiz de workspace

No primeiro uso que exija criar um Work, se nenhuma raiz de workspace estiver configurada, o Work sugere um diretório padrão e permite que o usuário escolha outro diretório. A escolha é persistida e reutilizada nos próximos comandos. O usuário pode alterar posteriormente a raiz configurada.

O usuário roda `work start [nome ou referência do repo]`. Sem argumento, a TUI coleta a origem; com argumento, o Starter o interpreta e devolve uma Repository Reference. Se ela já contiver um caminho local, o Work valida diretamente o repositório; caso contrário, usa a Repository Resolution Policy configurada para localizar um clone local. Resolvido exatamente um repositório válido, o fluxo de criação do Work prossegue normalmente. Como o Starter não retorna `start_modes`, o Work cria um Work novo. O usuário escolhe slug, convenção e prefixo de branch e recebe uma worktree pronta.

### 7.2 Contribuir ou fazer fork de um pull request

O usuário roda `work start <link do PR>`. O Starter resolve o repositório, a base e retorna `start_modes: ["contribution", "fork"]`. O Work oferece esses modos. Em contribuição, faz checkout da branch resolvida sem pedir slug ou convenção; em fork, segue a jornada de branch própria. O Starter pode publicar o link do PR e metadata útil; em `start:finalized`, componentes elegíveis podem descobrir links e importar contexto.

### 7.3 Retomar e arquivar trabalhos

Com `work resume`, o usuário seleciona um Work em ordem de acesso recente; `work resume <work>` pula a seleção. Com `work archive`, seleciona trabalhos ativos, confirma, e o Work destrói as worktrees e arquiva os demais arquivos e o snapshot de estado; alvos explícitos em `work archive <work...>` pulam apenas a seleção, não a confirmação.

### 7.4 Importar contexto manualmente

Dentro de um Work, o usuário roda `work import`, escolhe um Importer manual disponível e o Work executa-o somente se todos os inputs obrigatórios estiverem presentes. `work import <importer>` pula a seleção. Os arquivos produzidos são incorporados após validação de colisões.

### 7.5 Associar um link manualmente

Dentro de um Work, o usuário roda `work link`, escolhe um Linker manual disponível e informa um valor. Em uso direto, `work link <linker> <valor>` fornece ambos. O Work persiste o valor na chave do Linker.

### 7.6 Inspecionar o Work

Dentro de um Work, o usuário roda `work status` para inspecionar identidade, estado, branch, localização e links persistidos sem executar extensões nem alterar o estado. Fora dele, `work status <work>` inspeciona um alvo explícito.

### 7.7 Gerenciar plugins, descoberta e convenções

O usuário entra em `work plugin` para instalar, listar, habilitar/desabilitar, atualizar e remover pacotes por TUI. `work repository` abre a gestão da descoberta de clones; `work convention` mostra a escolha atual e permite trocá-la. Cada TUI torna visível o comando direto equivalente para automação.

***

## 8. Requisitos funcionais

### `work start [source]`

* **RF-1.** O Work deve selecionar estaticamente, entre Starters habilitados, aquele cujo `pattern` reconhece o argumento; múltiplos matches devem ser apresentados ao usuário, sem memorização da escolha.
* **RF-2.** O Starter escolhido deve receber o argumento e devolver uma Repository Reference, além dos demais dados de início que conseguir resolver. A referência pode conter diretamente o caminho de um repositório local ou identificadores suficientes para uma tentativa posterior de localização. Antes de qualquer materialização do Work, o Work deve resolver a referência para exatamente um repositório Git local válido e acessível.
* **RF-3.** Quando a resposta do Starter contiver `start_modes`, o Work deve oferecer exatamente os modos retornados. A ausência desse campo significa criação de Work novo. No modo contribuição, o Work faz checkout da branch resolvida e não executa as etapas de slug, convenção e prefixo; no modo fork, o fluxo segue como Work novo.
* **RF-4.** Fora do modo contribuição, o Work deve solicitar um slug.
* **RF-5.** Fora do modo contribuição, o Work deve apresentar os prefixos da convenção de branch resolvida para escolha.
* **RF-6.** Se o Starter não fornecer base branch, o Work deve permitir selecionar uma branch remota ou local.
* **RF-7.** O Work deve criar a worktree, seu diretório e `work-state.json`; plugins podem acrescentar outros arquivos e diretórios, que não são estrutura criada ou interpretada pelo núcleo.
* **RF-8.** Após materializar a estrutura e o estado de núcleo, o Work deve publicar `start:finalized`, executar Linkers e Importers inscritos e elegíveis na ordem definida pela arquitetura e indicar ao usuário quais extensões estão sendo executadas.
* **RF-9.** Ao concluir, o comando deve reposicionar o terminal no diretório da worktree recém-criada.
* **RF-10.** No primeiro uso, o Work deve disponibilizar, sem instalação manual ou dependência de ferramenta externa, os componentes e a configuração mínima necessários para iniciar um Work a partir de um repositório local encontrável pela estratégia padrão.
* **RF-23.** No primeiro uso de um repositório fora do modo contribuição, se houver mais de uma convenção habilitada, o usuário deve escolher uma e a escolha deve ser memorizada para esse repositório.
* **RF-24.** No primeiro uso, o Work deve disponibilizar ao menos uma convenção de branch por padrão.
* **RF-36.** Quando nenhuma raiz de workspace estiver configurada, o Work deve solicitar sua definição antes da primeira criação de Work, oferecendo um valor padrão sugerido, persistir a escolha e permitir sua alteração posterior.
* **RF-37.** Fora do modo contribuição, antes de criar uma branch ou worktree, o Work deve garantir que o nome de branch resultante seja válido para Git e não colida com uma branch existente incompatível com a nova criação. Em caso de rejeição, o usuário deve poder corrigir a escolha que originou o nome e tentar novamente.

### Extensões do Work

* **RF-26.** Antes de executar uma operação de Importer ou descoberta de Linker, o Work deve avaliar estaticamente habilitação, forma de ativação, inscrição no evento, filtro de Starter e inputs obrigatórios; componente inelegível não deve ser executado.
* **RF-27.** O Work deve resolver inputs nos namespaces `work`, `meta` e `link` e projetar ao subprocesso somente os inputs declarados pelo componente; inputs opcionais ausentes devem ser omitidos.
* **RF-28.** Para cada execução de Importer, o Work deve fornecer diretório temporário exclusivo, validar todas as colisões antes de alterar o Work e nunca sobrescrever arquivos silenciosamente.
* **RF-29.** Falha automática de Linker ou Importer após a criação do Work deve concluir `work start` com aviso e diagnóstico, sem desfazer o Work. Uma descoberta bem-sucedida sem valor não é erro.
* **RF-30.** Dentro de um Work, `work import` deve oferecer Importers que declaram `manual` e estejam elegíveis, e executar a operação `import` selecionada; um Importer explícito deve pular a seleção sem pular a validação de elegibilidade.
* **RF-31.** Dentro de um Work, `work link` deve oferecer Linkers que declaram `manual`, solicitar um valor não vazio e persistir esse valor sob a chave do Linker; Linker e valor explícitos devem permitir a mesma operação sem TUI.
* **RF-32.** O estado do Work, metadata e links devem ser persistidos em seções semanticamente distintas no mesmo snapshot canônico. Cada chave de link mantém um único valor; publicação pelo Starter, descoberta e associação manual fazem upsert, e a última origem vence.
* **RF-33.** O snapshot `work-state.json` deve ser atualizado atomicamente, versionado por schema e controlado exclusivamente pelo core; plugins interagem com estado apenas por inputs e outputs públicos.
* **RF-34.** O Work deve manter `work.db` como projeção global consultável e ser capaz de reconstruí-la ou reconciliá-la a partir dos snapshots dos Works.
* **RF-35.** Chaves públicas de `meta` e `link` devem seguir Semantic Conventions; chaves privadas devem usar namespace explícito do plugin. A identidade semântica da chave não deve incluir o componente que produziu seu valor.
* **RF-38.** `work status [work]` deve apresentar identidade, estado, branch, localização aplicável e links atualmente persistidos no snapshot canônico. Sem alvo, deve usar o Work associado ao diretório corrente. A operação é somente leitura, não executa extensões e não modifica o Work nem seu acesso recente.
* **RF-39.** O Work deve permitir que plugins forneçam Repository Locators independentemente dos Starters. Um Starter não deve precisar conhecer quais Repository Locators estão instalados ou configurados na máquina.
* **RF-40.** O usuário deve poder manter múltiplos Repository Locators instalados e habilitados e definir explicitamente quais deles participam da Repository Resolution Policy e em qual ordem.
* **RF-41.** A instalação ou habilitação de um plugin não deve inserir silenciosamente seus Repository Locators na Repository Resolution Policy existente. Em fluxo interativo, o Work pode oferecer ao usuário a configuração imediata dos novos Locators.
* **RF-42.** Durante a resolução, o Work deve considerar os Repository Locators da policy em ordem. Um Locator sem resultado permite que a resolução continue para o próximo Locator elegível. Um único resultado válido encerra a resolução. Múltiplos resultados válidos devem ser apresentados ao usuário para escolha explícita.
* **RF-43.** Falha operacional de um Repository Locator é distinta de ausência de resultado. Na v1, falha de um Locator explicitamente selecionado na Repository Resolution Policy deve interromper a resolução com diagnóstico, sem fallback silencioso para o próximo Locator.
* **RF-44.** O Work deve avaliar a elegibilidade de um Repository Locator antes de executá-lo, utilizando apenas informações declaradas estaticamente pelo componente e os identificadores presentes na Repository Reference.
* **RF-45.** O usuário deve poder inspecionar e gerenciar a Repository Resolution Policy por TUI e por comandos não interativos equivalentes, incluindo inclusão, remoção e reordenação de Locators. Deve também poder listar Locators instalados e gerenciar raízes de busca de repositórios. A configuração persistida deve permanecer inspecionável e utilizável por automação.
* **RF-46.** O Work deve permitir configurar raízes de busca de repositórios independentemente da raiz onde Works e suas worktrees são materializados, e projetá-las explicitamente aos Repository Locators que forem executados.
* **RF-47.** Quando um plugin for desabilitado, seus Repository Locators devem permanecer referenciados na Repository Resolution Policy, porém indisponíveis enquanto o plugin estiver desabilitado. A interface deve tornar esse estado visível.
* **RF-48.** A desinstalação de um plugin que forneça Repository Locators referenciados pela Repository Resolution Policy deve reconhecer essas referências e, mediante confirmação explícita, removê-las na mesma alteração consistente; o modo não interativo deve exigir tratamento explícito dessa dependência.
* **RF-49.** A estratégia de localização fornecida por padrão deve conseguir descobrir repositórios Git em raízes de busca configuradas sem depender de serviços externos. Quando mais de um repositório corresponder à referência fornecida, o usuário deve poder selecionar explicitamente o desejado.

### `work resume` e `work archive`

* **RF-11.** `work resume` deve listar trabalhos existentes do mais recentemente acessado ao mais antigo e abrir o selecionado; com alvo explícito, deve abri-lo diretamente.
* **RF-12.** A seleção em `work resume` deve atualizar o acesso recente e reposicionar o terminal na worktree.
* **RF-13.** `work archive` deve listar trabalhos ativos com multi-seleção; com alvos explícitos, deve pular a seleção e ainda exigir a confirmação aplicável.
* **RF-14.** Após confirmação, deve destruir as worktrees selecionadas e mover os demais arquivos para a área de arquivados.
* **RF-15.** O estado persistido deve refletir o arquivamento.

### Gestão de plugins e convenções

* **RF-16.** O usuário deve poder instalar plugin de origem remota ou caminho local como conteúdo fixado; para desenvolvimento, `--link` deve permitir vincular um caminho local sem copiar.
* **RF-17.** O usuário deve poder listar plugins instalados.
* **RF-18.** O usuário deve poder habilitar ou desabilitar plugin sem desinstalá-lo.
* **RF-19.** O usuário deve poder verificar atualizações com `work plugin update --check` e aplicá-las explicitamente a plugins nomeados ou a todos; comandos de Work não verificam atualizações automaticamente.
* **RF-20.** O usuário deve poder desinstalar plugin.
* **RF-21.** Conflito de alias de plugin na instalação deve falhar explicitamente, sem sobrescrita ou renomeação silenciosa.
* **RF-25.** O usuário deve poder inspecionar e trocar a convenção de branch memorizada a partir de qualquer clone do repositório.

### Extensibilidade

* **RF-22.** Todo comportamento além da orquestração de worktrees via Git deve ser delegado a plugins externos, descobertos e instalados pelo usuário.

### Superfície CLI/TUI

* **RF-50.** Em terminal interativo, `work` sem argumentos deve exibir uma entrada estática de marca com orientação para `work --help`, sem abrir um seletor. `work --help` deve apresentar uso, opções e exatamente os comandos públicos disponíveis, agrupados por contexto. `work plugin`, `work repository` e `work convention` devem continuar abrindo os hubs de seus domínios.
* **RF-51.** Em terminal interativo, valores de seleção omitidos devem ser coletados por TUI e valores explícitos devem pular somente a seleção correspondente. Em stdin não interativo, valor obrigatório ausente deve falhar com uso acionável sem tentar abrir TUI.
* **RF-52.** Comandos de leitura devem aceitar `--json` e não alterar estado; mutações devem ter saída e códigos de saída estáveis. `--yes` pode confirmar impactos já determinados, mas não escolher alvos ou valores.
* **RF-53.** Toda coleta e seleção interativa deve ocorrer no buffer corrente do terminal. Ao aceitar uma etapa, o controle ativo deve ser substituído por um recibo compacto com título, marca de sucesso e valor aceito; valores sensíveis devem ser redigidos ou omitidos.
* **RF-54.** Uma falha recuperável atribuível ao campo ativo deve aparecer dentro desse campo e ser substituída pela tentativa seguinte. No máximo um erro atual deve permanecer visível, e tentativas rejeitadas não devem se acumular no histórico do terminal.
* **RF-55.** Controles ativos devem respeitar as dimensões disponíveis do terminal. Mover foco, marcar opções, filtrar ou trocar grupos não deve deslocar colunas nem alterar a altura das linhas que não mudaram.
* **RF-56.** O foco de seletores deve ser identificável por negrito e por um indicador textual de largura reservada; cor pode reforçá-lo, mas não pode ser o único sinal. Seletores simples e múltiplos devem oferecer teclas consistentes de movimento, filtragem quando disponível, confirmação e cancelamento, com ajuda visível.
* **RF-57.** O cancelamento interativo deve deixar um único resultado humano conciso e preservar a categoria e o código de saída programáticos já contratados. Cancelamento não pode executar nem confirmar uma mutação.
* **RF-58.** Falhas não recuperáveis devem ser apresentadas uma única vez, em linguagem do usuário, com próximo passo quando conhecido. Causas técnicas devem permanecer disponíveis para diagnóstico explícito, sem compor a saída humana normal.
* **RF-59.** A marca principal deve escrever `WORK` como arte de terminal e aplicar, quando houver cor verdadeira e contraste adequado, um degradê entre as cores primária `#11A8CD` e secundária `#8B7CF6`. Terminais estreitos ou sem cor devem receber a forma compacta `WORK` sem quebra defeituosa.
* **RF-60.** A apresentação deve usar um tema semântico único: as cores de marca não substituem os significados de sucesso, aviso ou falha; texto de corpo usa a cor normal do terminal; fundos claros, paletas limitadas e modo monocromático recebem alternativas legíveis.
* **RF-61.** Cor deve ser desativada em saída não interativa, quando `NO_COLOR` não estiver vazio ou quando `TERM=dumb`. A saída continua compreensível sem cor e não deve conter sequências de controle nesses casos.
* **RF-62.** Apresentação interativa e diagnósticos humanos devem usar o canal de interface configurado; resultados estáveis de comandos permanecem no stdout. A renovação visual não pode alterar argumentos, flags, tokens de erro ou códigos de saída existentes sem contrato posterior explícito.
* **RF-63.** `work` sem argumentos deve sair com sucesso depois da entrada de marca em terminal interativo. Em uso não interativo, deve preservar a falha de uso existente; `work --help` deve sair com sucesso em ambos os modos.
* **RF-64.** Confirmações que antecedem mutações devem mostrar o impacto antes da aceitação e, quando aceitas, reduzir-se a um recibo compacto antes do resultado estável da operação. Voltar e editar etapas anteriores não faz parte desta entrega.

***

## 9. Requisitos não funcionais

* **RNF-1. Simplicidade radical do núcleo.** O Work permanece pequeno e auditável; lógica de domínio vive em plugins.
* **RNF-2. Extensibilidade sem fricção.** Novo plugin não exige mudança no código do Work.
* **RNF-3. Determinismo.** Preparação e elegibilidade são previsíveis e não dependem de IA.
* **RNF-4. Zero custo de raciocínio de IA.** Nenhuma jornada do CLI requer LLM.
* **RNF-5. Auditabilidade do estado.** O snapshot local canônico preserva Work, metadata, links e artefatos de forma rastreável; o índice global pode ser reconstruído a partir dele.
* **RNF-6. Portabilidade.** O comportamento é consistente nos sistemas operacionais suportados.
* **RNF-7. Confiança mínima necessária.** A origem de plugin é escolhida explicitamente pelo usuário e o núcleo nunca carrega código de plugin no próprio processo.
* **RNF-8. Componibilidade de extensões.** Starters e mecanismos de localização de repositório devem poder evoluir independentemente. A criação de um novo Starter não deve exigir implementação própria das estratégias locais de localização já disponíveis ao usuário.
* **RNF-9. Política local explícita.** A precedência entre mecanismos de localização pertence à configuração do usuário e não deve depender de ordem de instalação, prioridade autodeclarada pelo plugin ou heurística oculta do core.
* **RNF-10. CLI progressiva e consistente.** Toda jornada deve ser descobrível a partir de `work` e `work --help`; comandos cotidianos permanecem rasos, enquanto comandos diretos para automação seguem uma gramática regular, têm saída e códigos de saída estáveis e nunca dependem de TUI.
* **RNF-11. Apresentação acessível e portátil.** Interações devem permanecer compreensíveis sem cor, adaptar-se ao espaço disponível e preservar geometria estável nos sistemas operacionais e terminais suportados.

***

## 10. Experiência do usuário

Em terminal interativo, `work` sem argumentos mostra uma entrada estática: o wordmark `WORK` em arte de terminal, colorido por um degradê entre os acentos primário e secundário quando houver suporte, seguido da tagline e da indicação de `work --help`. A ajuda agrupa somente os comandos realmente disponíveis por contexto de uso e mantém todas as jornadas públicas descobríveis. O comando vazio não abre uma seleção. Em modo não interativo, preserva a falha de uso existente; `work --help` funciona nos dois modos.

Os comandos cotidianos são `work start [source]`, `work resume [work]`, `work archive [work...]`, `work status [work]`, `work import [importer]` e `work link [linker] [value]`. Ausência de alvo usa TUI quando a operação exige escolha; alvo explícito pula essa seleção. Em stdin não interativo, valor obrigatório ausente falha com uso acionável em vez de tentar abrir TUI.

No fluxo de `work start`, o Work resolve e apresenta escolhas necessárias, cria o Work, materializa seu estado canônico a partir da resposta do Starter e executa extensões pós-criação. Enquanto Linkers e Importers são executados, o Work indica qual extensão está em execução; erros automáticos são reportados como avisos com diagnóstico, pois o Work já está disponível.

Todas as escolhas — Starters concorrentes, modo de início, convenção, prefixo, base branch, retomada, arquivamento, Importer e Linker manual — usam controles consistentes, inline e navegáveis por teclado. Enquanto ativos, erros recuperáveis são substituídos no próprio campo e seletores respeitam o viewport sem saltos de geometria; quando aceitos, deixam recibos compactos com os valores escolhidos. Cancelamentos e falhas finais aparecem uma única vez, sem expor cadeias técnicas na saída humana normal. `work status` usa o Work atual quando não recebe um Work explícito; `work import` e `work link` operam sempre sobre o Work atual. Quando esse contexto não puder ser resolvido, falham com mensagem clara.

`work plugin`, `work repository` e `work convention` são hubs TUI. Na API direta, o verbo vem após um caminho de recursos no singular: `plugin list|install|enable|disable|update|uninstall`, `repository policy list|add|remove|move|replace`, `repository locator list`, `repository root list|add|remove|replace` e `convention show|set`. Remover um Locator da policy apenas deixa de usá-lo na estratégia; desabilitar continua sendo uma operação exclusiva do plugin. A superfície completa e suas semânticas transversais são governadas pela ADR-0019; a separação entre jornadas e apresentação interativa é governada pela ADR-0020.

Comandos de leitura aceitam `--json` e não alteram acesso recente, configuração, checkout ou proveniência. `--yes` confirma impactos já determinados, mas nunca escolhe alvo, modo ou valor. Não existem aliases oficiais de comandos ou recursos.

***

## 11. Métricas de sucesso

* Tempo entre `work start <arg>` e ambiente pronto, incluindo extensões pós-criação.
* Número de plugins criados pela comunidade.
* Redução de artefatos de IA/spec commitados acidentalmente.
* Uso recorrente de `work resume`.
* Taxa de extensões que se tornam elegíveis e concluem sem intervenção manual.
* Taxa de conclusão das jornadas interativas sem resíduos de tentativas rejeitadas, erros antigos ou seletores expandidos no histórico do terminal.
* Tempo para uma pessoa encontrar, pela entrada de marca e pela ajuda, o comando adequado a uma jornada pública.

***

## 12. Riscos e mitigações

| Risco | Mitigação |
| --- | --- |
| Plugin malicioso ou defeituoso | Origem explícita, processos externos e nenhuma permissão adicional implícita. |
| Ambiguidade de Starter | Escolha explícita a cada colisão, sem prioridade oculta. |
| Importação destrutiva | Staging e detecção de todas as colisões antes da incorporação. |
| Extensão sem dados suficientes | Elegibilidade estática; o componente não é executado apenas para descobrir a ausência de input. |
| Perda de contexto no arquivamento | Arquivamento preserva metadata, links e artefatos; somente a worktree é destruída. |
| Instalação de plugin altera inesperadamente como repositórios são encontrados | Locators novos não entram automaticamente na policy existente. |
| Dois mecanismos localizam repositórios diferentes para a mesma referência | A policy é ordenada e usa short-circuit; não há arbitragem implícita entre resultados de Locators distintos. |
| Locator configurado fica indisponível | O estado é exposto explicitamente; falha operacional não é tratada como ausência de match. |
| Scanner padrão encontra worktrees criadas pelo próprio Work | Raízes de busca de repositórios são configuradas separadamente da raiz de materialização dos Works. |

***

## 13. Roadmap / fora do escopo da v1

O sequenciamento da v1 em fatias verticais, com demonstração e critério de saída por entrega, está em [`docs/roadmap.md`](roadmap.md).

**Fora do escopo da v1:** plugins de integração além do conjunto mínimo de referência; múltiplas convenções simultâneas por repositório; sincronização entre máquinas; interface gráfica; sandbox de segurança para plugins.

**Candidatos futuros:** telemetria local opcional; integrações oficiais de forge; componente executável para detectar automaticamente convenção de branch; novas operações e eventos de lifecycle definidos pelo núcleo.
