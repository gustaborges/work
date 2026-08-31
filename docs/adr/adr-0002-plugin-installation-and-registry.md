# ADR-0002: Instalação de Plugins, Registro Gerado e Separação de Config/Estado

**Status:** Aceito
**Data:** 2026-08-23
**Contexto de produto:** `docs/prd.md` — RF-16, RF-17, RF-20, RF-21, RNF-2, RNF-5
**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seção 5

## Contexto

Sem um mecanismo de instalação definido, o fluxo implícito exigiria copiar arquivos manualmente e editar configuração central à mão — o oposto de RNF-2. Falta também uma separação entre config editada por humano e estado gerenciado pela ferramenta (RNF-5).

## Decisão

Dois tipos de fonte de instalação, alimentando o mesmo registro:

* Instalar a partir de uma origem remota — clona o pacote, fixando uma referência de conteúdo no momento da instalação.
* Instalar a partir de um caminho local (link, sem copiar) — existe para desenvolvimento ativo de um plugin.

**Nome local e colisão de alias.** O nome usado no registro funciona como o alias que o usuário digita nos demais comandos de gestão de plugin (RF-17 a RF-21). Por padrão, a instalação usa o `name` declarado no manifesto do pacote (ADR-0012) como esse alias. Como esse campo é escolhido pelo autor do pacote sem nenhuma coordenação entre autores, dois pacotes de fontes diferentes podem propor o mesmo `name` — a garantia de unicidade não vem do manifesto, vem do registro local do usuário:

* Se o alias resultante já existe no registro apontando para uma origem diferente, a instalação **falha** com uma mensagem explícita apontando o conflito (RF-21), em vez de sobrescrever ou auto-sufixar silenciosamente — auto-sufixo silencioso violaria RNF-3, já que o mesmo alias passaria a apontar para fontes diferentes em máquinas diferentes dependendo da ordem de instalação.
* O usuário resolve escolhendo explicitamente um alias alternativo para aquele pacote. Reinstalar a mesma origem sob o mesmo alias já existente é idempotente, não é conflito.
* Essa unicidade é sempre **local** ao registro do usuário, nunca global — é a mesma garantia que já existe para clonar dois repositórios git: não há como ter dois diretórios com o mesmo nome sem renomear um.

**Referência a componente individual.** Comandos que apontam para um componente específico aceitam o nome nu do componente quando ele é único entre os componentes habilitados; em colisão de nome entre pacotes diferentes, a referência é qualificada por pacote (esquema exato em `add-0001` §4).

A instalação separa, no layout de diretórios, config (editado por humano) de estado (gerado pela ferramenta) — layout completo em `add-0001` §5. O registro em si vive inteiramente no estado gerado, nunca editado à mão. Listar plugins instalados (RF-17) e desinstalar (RF-20) são comandos diretos de primeira classe, usáveis em CI/dotfiles sem depender da TUI de gestão descrita no ADD. Conforme ADR-0015, a policy de Repository Locators é configuração declarativa do usuário: instalação e habilitação não a alteram; ao desinstalar um plugin cujos Locators estejam nela, o Work exige tratamento explícito e remove as referências confirmadas na mesma alteração consistente.

## Alternativas consideradas

* **Config central editada à mão** — rejeitada: é a causa direta do gap descrito no Contexto e uma superfície de erro (usuário pode digitar errado um campo sem que nada valide contra o que o plugin de fato faz).
* **Auto-sufixar alias em colisão** (ex.: instalar automaticamente como `plugin-2`) — rejeitada: o sufixo dependeria da ordem de instalação, tornando o alias não-determinístico entre máquinas/execuções diferentes (viola RNF-3); pedir uma escolha explícita é mais verboso mas reprodutível.
* **Nome canônico sempre derivado da URL de origem** (em vez de alias curto) — rejeitada como padrão: resolve unicidade por construção, mas é verboso para o uso diário; fica disponível como sugestão natural quando uma escolha explícita é necessária, não como comportamento padrão.

## Consequências

**Positivas:** instalação é uma ação de primeira classe, auditável e scriptável; fronteira clara entre config e estado.

**Negativas / trade-offs:** mais um subsistema (registro + convenção de diretório) para construir e manter sincronizado — mitigado por ter uma única fonte de verdade (nunca estado duplicado entre o registro e os arquivos em disco).
