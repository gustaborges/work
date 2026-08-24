# ADR-0001: Manifesto de Plugin (`plugin.json`) e Pacotes Multi-Componente

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs/prd.md` — RF-22, RNF-1, RNF-2, RNF-7
**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seção 4

## Contexto

Um modelo em que `work.start.json` mistura *o plugin descrevendo a si mesmo* com *o usuário configurando manualmente o plugin* — pattern, capabilities e hooks digitados à mão pelo usuário, sem nenhuma garantia de que correspondem ao que o plugin realmente faz — é a causa raiz de um gap estrutural: nada valida a config contra o comportamento real do plugin. Além disso, um identificador de plugin acoplado ao nome do arquivo/linguagem de implementação contradiz a promessa de que "plugins podem ser escritos em qualquer linguagem" (RNF-2, formalizada em ADR-0006).

## Decisão

* Todo plugin carrega um manifesto próprio, `plugin.json`, na raiz do seu pacote — não editado pelo usuário, apenas lido pelo Work no momento da instalação.
* Um manifesto descreve um **pacote**, que pode declarar múltiplos componentes executáveis (cada um cumprindo um papel — Starter, Importer, Linker) e/ou múltiplas convenções de branch, uma contribuição puramente declarativa que não cumpre papel executável algum (ADR-0010) e por isso vive em um array próprio do manifesto, separado dos componentes. Um pacote é instalado/habilitado/desinstalado atomicamente como uma unidade — resolve o caso de uma integração que naturalmente acopla mais de uma contribuição (ex: um plugin que reconhece um link de PR e, no mesmo pacote, importa metadados desse PR).
* A identidade de um componente é o nome lógico declarado no manifesto, nunca o nome do arquivo de implementação — desacopla identificador de linguagem.
* O nome de um pacote no manifesto é uma **proposta de alias local**, não uma garantia de unicidade global — dois autores diferentes podem propor o mesmo nome sem coordenação entre si. A unicidade de fato é aplicada no momento da instalação, contra o registro local do usuário (ver ADR-0002), não pelo manifesto em si. Quando dois componentes instalados colidem por nome, a referência entre eles nos comandos é qualificada por pacote — o esquema exato de referência está em `add-0001` §4.
* `pattern`, `capabilities` e `hooks` são declarados pelo autor do plugin dentro do `plugin.json` e confiados como declarados — o usuário nunca os digita à mão (o registro é gerado automaticamente no momento da instalação, ADR-0002), e o Work nunca invoca um componente antes do seu primeiro uso real só para confirmar o que ele declara. Isso já é suficiente para fechar o gap descrito no Contexto: ali, quem descolava a config do comportamento real era o usuário digitando à mão; aqui, quem escreve o manifesto é quem escreve a implementação, dentro do mesmo pacote versionado junto. O risco residual de uma capability mal declarada é aceito e se manifesta como um erro comum de execução na primeira vez que ela é de fato exercida (ex: o Work espera o campo `base_branch` de um componente que declarou `provide-base-branch` e ele não vem), nunca como uma falha silenciosa.
* O schema do manifesto v1 é deliberadamente mínimo; os schemas de payload por papel de componente são definidos incrementalmente, à medida que cada papel é implementado — não travados de antemão. Schema completo em `add-0001` §4.

## Alternativas consideradas

* **1 repositório = 1 capability** (granularidade estilo `gh extension`) — rejeitada: integrações reais são naturalmente acopladas (reconhecer um PR e importar seus metadados só fazem sentido juntos); forçar dois repositórios versionados/lançados de forma independente para algo que só faz sentido junto é fricção artificial tanto para autor quanto para usuário.
* **Campos inferidos de nome de arquivo/config central editada à mão** — rejeitada: é a causa raiz do gap estrutural descrito acima, já que nada valida a config contra o que o plugin de fato faz.
* **Referência de componente prefixada por papel** (ex: `pacote@starter:nome`) — rejeitada: o papel de um componente é metadado estável, não algo que precise viajar na string de referência; embuti-lo é redundante na esmagadora maioria dos casos e adiciona um separador novo ao vocabulário do CLI sem necessidade.
* **Nome canônico do pacote sempre `owner/repo` concatenado** (modelo `publisher.extension` do VS Code) — rejeitada: esse modelo depende de um registro central que arbitra unicidade globalmente, que o Work não tem. Sem ele, forçar o nome completo em todo comando é só verbosidade sem o ganho de unicidade global que o modelo original compra.
* **Handshake de auto-descrição**: invocar cada componente com uma ação dedicada no `install`/`update` para que ele confirme em runtime suas próprias `capabilities` (e reporte sua própria versão), tratando essa resposta como fonte de verdade em vez do manifesto. Rejeitada por três motivos: (1) nenhum precedente de mercado confirma capability invocando a lógica de domínio real do componente com dado sintético — quando um ecossistema quer autodescrição verificada (LSP `initialize`, `GetProviderSchema` do Terraform), ele cria um verbo de protocolo dedicado e contratualmente puro, nunca uma variação da chamada normal; sem essa pureza garantida por contrato, "descrever" um Importer ou Linker corre o risco de disparar o efeito colateral real que o componente existe para causar (chamada de rede, escrita em disco); (2) versão é um fato do **pacote**, não de um componente/script individual — a ADR-0005 já resolve "o que mudou" lendo o campo `version` do manifesto direto do object database, sem instalar nem invocar processo algum; pedir que cada componente também soubesse relatar sua própria versão duplicaria esse dado no lugar errado, reabrindo exatamente o risco de duas fontes de verdade divergentes que esta ADR existe para fechar; (3) o gap que restaria fechar — manifesto levemente incorreto — já é pequeno o suficiente para ser aceito como risco residual, dado que instalar um plugin já é uma decisão de confiança explícita do usuário sobre uma origem que ele mesmo escolheu (RNF-7).

## Consequências

**Positivas:** instalação/desinstalação atômica de componentes acoplados; identidade desacoplada da linguagem de implementação; conteúdo do manifesto autoauditável, escrito por quem implementa, nunca por quem só consome.

**Negativas / trade-offs:** um pacote malprojetado pode virar um "monólito" misturando responsabilidades demais — mitigado como convenção/boa prática de ecossistema (documentação), não como restrição do mecanismo.
