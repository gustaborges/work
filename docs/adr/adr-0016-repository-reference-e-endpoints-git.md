# ADR-0016: Modelo de Repository Reference e Semântica de Endpoints Git

**Status:** Aceito

**Data:** 2026-08-31

**Contexto de produto:** `docs/prd.md` — RF-2, RF-42, RF-44 e RF-49

**Governa:** `docs/add/add-0001-work-system-architecture.md`, Seções 4.1 e 7.1

## Contexto

Após interpretar um argumento, um Starter pode conhecer um caminho local, endpoints Git, um nome ou apenas um texto de busca. Esses dados têm semânticas distintas. Em especial, uma URL Git não é uma identidade canônica universal: protocolos, usuários, hosts e paths não admitem equivalências genéricas seguras.

## Decisão

A `Repository Reference` v1 é um objeto transitório com campos independentes e opcionais:

```jsonc
{
  "repository": {
    "path": "/home/user/src/payments",
    "git_fetch_urls": ["https://github.com/acme/payments.git", "git@github.com:acme/payments.git"],
    "name": "payments",
    "query": "pay"
  }
}
```

`path` declara localização já resolvida e é validado diretamente pelo core. `git_fetch_urls` são endpoints conhecidos para fetch, não identidade canônica. `name` é uma propriedade conhecida, porém potencialmente ambígua. `query` é texto opaco para mecanismos locais; não deve ser promovido automaticamente a `name`.

Locators declaram em `accepts` quais dos campos `git_fetch_urls`, `name` e `query` consomem; `path` nunca aparece ali. Um Locator que consome URLs compara fetch URLs em todos os remotes locais, não apenas `origin`, e usa operações nativas do Git em vez de reinterpretar `.git/config` ou regras de rewrite.

O Work não remove protocolos, usuários ou sufixos `.git`, não infere equivalência SSH/HTTPS e não usa URLs de push. A v1 não introduz identifiers tipados de provider; eles podem ser considerados quando existir consumo concreto além dos campos atuais.

## Consequências

Componentes de forge podem publicar conhecimento rico sem forçar dependência de provider nos Locators; clones de forks podem corresponder por `upstream`; aliases permanecem locais. Em troca, endpoints equivalentes não publicados pelo produtor podem não corresponder e não há identidade universal de repositório na v1.

## Alternativas rejeitadas

* `remote_url` singular ou URL Git canônica: não representam todos os endpoints nem possuem normalização universal segura.
* Usar apenas `origin` ou URLs de push: omite clones legítimos e descreve destino operacional, não a fonte Git.
* Tratar `name` como identificador global ou `query` como `name`: ambos perdem semântica e introduzem falsos matches.
* Identifiers tipados na v1: ampliariam a superfície pública antes de haver necessidade comprovada.
