# ADR-0001: Manifesto de Plugin (`plugin.json`) e Pacotes Multi-Componente

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs-v2/prd.md` — RF-22, RNF-1, RNF-2, RNF-7
**Governa:** `docs-v2/add/add-0001-work-system-architecture.md`, Seção 4
**Supersede:** `temp/estrategia-plugins.md`, Seção 0 (premissa) e Seção 3 (3.2)
**Insumo:** `temp/analise-prd.md`, Seções 1.2, 1.5 e 2

## Contexto

`analise-prd.md` identificou o gap estrutural central do rascunho v1 do PRD: `work.start.json` misturava *o plugin descrevendo a si mesmo* com *o usuário configurando manualmente o plugin* — pattern, capabilities e hooks eram digitados à mão pelo usuário, sem nenhuma garantia de que correspondiam ao que o plugin realmente fazia (Seção 1.2). Além disso, o identificador de um plugin estava acoplado ao nome do arquivo/linguagem de implementação, contradizendo a promessa de que "plugins podem ser escritos em qualquer linguagem" (Seção 1.5, formalizada em ADR-0006).

## Decisão

* Todo plugin carrega um manifesto próprio, `plugin.json`, na raiz do seu pacote — não editado pelo usuário, apenas lido pelo Work no momento da instalação.
* Um manifesto descreve um **pacote**, que pode declarar múltiplos componentes (cada um cumprindo um papel — Starter, Importer, Linker, descoberta de Branch Strategy). Um pacote é instalado/habilitado/desinstalado atomicamente como uma unidade — resolve o caso de uma integração que naturalmente acopla mais de um papel (ex: um plugin que reconhece um link de PR e, no mesmo pacote, importa metadados desse PR).
* A identidade de um componente é o nome lógico declarado no manifesto, nunca o nome do arquivo de implementação — desacopla identificador de linguagem.
* O nome de um pacote no manifesto é uma **proposta de alias local**, não uma garantia de unicidade global — dois autores diferentes podem propor o mesmo nome sem coordenação entre si. A unicidade de fato é aplicada no momento da instalação, contra o registro local do usuário (ver ADR-0002), não pelo manifesto em si. Quando dois componentes instalados colidem por nome, a referência entre eles nos comandos é qualificada por pacote — o esquema exato de referência está em `add-0001` §4.
* Pattern, capabilities e hooks são declarados pelo autor do plugin dentro do `plugin.json`. O usuário nunca os digita à mão; o registro é gerado automaticamente no momento da instalação (ADR-0002).
* Capabilities são validadas, não só confiadas: na instalação/atualização, o Work invoca cada componente com uma ação de auto-descrição sobre o mesmo contrato de execução já usado em runtime (ADR-0000); a resposta do próprio processo é a fonte de verdade sobre capabilities/versão.
* O schema do manifesto v1 é deliberadamente mínimo; os schemas de payload por papel de componente são definidos incrementalmente, à medida que cada papel é implementado — não travados de antemão. Schema completo em `add-0001` §4.

## Alternativas consideradas

* **1 repositório = 1 capability** (granularidade estilo `gh extension`) — rejeitada: integrações reais são naturalmente acopladas (reconhecer um PR e importar seus metadados só fazem sentido juntos); forçar dois repositórios versionados/lançados de forma independente para algo que só faz sentido junto é fricção artificial tanto para autor quanto para usuário.
* **Manter o modelo v1** (campos inferidos de nome de arquivo/config central) — rejeitada: é a causa raiz do gap central identificado em `analise-prd.md`.
* **Referência de componente prefixada por papel** (ex: `pacote@starter:nome`) — rejeitada: o papel de um componente é metadado estável, não algo que precise viajar na string de referência; embuti-lo é redundante na esmagadora maioria dos casos e adiciona um separador novo ao vocabulário do CLI sem necessidade.
* **Nome canônico do pacote sempre `owner/repo` concatenado** (modelo `publisher.extension` do VS Code) — rejeitada para v1: esse modelo depende de um registro central que arbitra unicidade globalmente, que o Work não tem no v1. Sem ele, forçar o nome completo em todo comando é só verbosidade sem o ganho de unicidade global que o modelo original compra.

## Consequências

**Positivas:** instalação/desinstalação atômica de componentes acoplados; identidade desacoplada da linguagem de implementação; conteúdo do manifesto autoauditável e validado na instalação.

**Negativas / trade-offs:** um pacote malprojetado pode virar um "monólito" misturando responsabilidades demais — mitigado como convenção/boa prática de ecossistema (documentação), não como restrição do mecanismo.
