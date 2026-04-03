# kboot

> **Conecte-se a todos os seus clusters EKS de uma vez.**

[![Latest Release](https://img.shields.io/github/v/release/apavanello/kboot?sort=semver&color=blue)](https://github.com/apavanello/kboot/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/apavanello/kboot)](https://go.dev)
[![License](https://img.shields.io/github/license/apavanello/kboot)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/apavanello/kboot)](https://goreportcard.com/report/github.com/apavanello/kboot)
[![Release](https://github.com/apavanello/kboot/actions/workflows/release.yml/badge.svg)](https://github.com/apavanello/kboot/actions/workflows/release.yml)

**kboot** é uma ferramenta CLI de DevOps projetada para simplificar o gerenciamento de múltiplos clusters Amazon EKS em diferentes contas AWS. Automatiza a autenticação via AWS SSO, gera kubeconfigs com contexto para os clusters selecionados em paralelo, e inicia o [`k9s`](https://k9scli.io/) com todos os clusters imediatamente acessíveis — sem poluir seu `~/.kube/config`.

## Índice

- [Funcionalidades](#funcionalidades)
- [Como Funciona](#como-funciona)
- [Pré-requisitos](#pré-requisitos)
- [Instalação](#instalação)
- [Uso](#uso)
- [Configuração](#configuração)
- [Autenticação](#autenticação)
- [Comandos CLI](#comandos-cli)
- [Infraestrutura de Testes](#infraestrutura-de-testes)
- [Testes](#testes)
- [Desenvolvimento](#desenvolvimento)
- [Estrutura do Projeto](#estrutura-do-projeto)
- [Segurança](#segurança)
- [Licença](#licença)

## Funcionalidades

- **Dashboard TUI Unificado** — Gerencie clusters, credenciais estáticas e perfis SSO em uma única interface de terminal construída com [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Autenticação via AWS SDK Puro** — Login SSO e geração de tokens EKS usam exclusivamente AWS SDK Go v2. **Sem dependência de AWS CLI.**
- **Credenciais AWS Isoladas** — Por padrão, kboot usa `~/.kboot/aws/` para credenciais, config e cache SSO. Seu `~/.aws/` do sistema nunca é tocado. Defina `use_system_aws: true` para usar os arquivos AWS do sistema.
- **Sincronização Paralela** — Autentica e busca metadados de múltiplos clusters simultaneamente com pool de workers configurável (padrão: 5 workers)
- **Aliasing de Contexto** — Mapeia ARNs complexos (`arn:aws:eks:us-east-1:123456789012:cluster/prod`) para apelidos curtos e amigáveis (`prod`, `staging`, `dev`)
- **Zero Poluição** — **Não** modifica seu `~/.kube/config`. Usa arquivos kubeconfig temporários em `/tmp/kboot/` que são limpos após a sessão
- **Modo Não-Interativo** — Target um cluster específico com `--cluster <alias>` para scripting e automação
- **Modo Headless** — Gera kubeconfigs e sai sem lançar o k9s, ideal para pipelines CI/CD
- **Multi-plataforma** — Funciona em Windows, Linux e macOS
- **Ambiente de Testes LocalStack** — Infraestrutura completa com Terraform + kind para testar sem tocar em recursos AWS reais
- **Instalador YOLO** — Setup de todas as dependências e infraestrutura com um comando

## Como Funciona

```
┌─────────────────────────────────────────────────────────┐
│  kboot                                                  │
│                                                         │
│  1. Carrega ~/.kboot.yaml → lista de clusters           │
│  2. Para cada cluster (paralelo, 5 workers):            │
│     a. Cria cliente AWS SDK (profile + region)          │
│     b. Verifica identidade (STS GetCallerIdentity)      │
│     c. Se auth falha → login SSO (device auth flow)     │
│     d. Descreve cluster (EKS API) → endpoint + CA       │
│  3. Gera kubeconfig por cluster (arquivos temp)         │
│  4. Lança k9s com KUBECONFIG=temp1:temp2:...            │
│  5. Exec substitui processo kboot pelo k9s              │
└─────────────────────────────────────────────────────────┘
```

## Pré-requisitos

| Dependência | Obrigatório | Finalidade |
|---|---|---|
| **Go 1.21+** | Apenas build | Compilar a partir do código fonte |
| **k9s** | Runtime (recomendado) | UI de terminal para Kubernetes |
| **AWS SDK Go v2** | Runtime (vendored) | Autenticação, API EKS — sem CLI externa |

> **Nota:** kboot **não** requer a AWS CLI. Todas as interações com AWS usam o AWS SDK Go v2 diretamente, incluindo login SSO via OAuth 2.0 Device Authorization Grant flow.

## Instalação

### Instalador YOLO (Recomendado)

Instala todas as dependências (Go, Docker, kubectl, kind, Terraform, k9s), compila o kboot, configura LocalStack + clusters kind, e prepara tudo automaticamente:

```bash
make install-yolo
```

### Compilar a partir do Código Fonte

```bash
git clone https://github.com/apavanello/kboot
cd kboot
go build -o kboot ./cmd/kboot/
```

### Usando Make

```bash
make build        # Compila para ./bin/kboot
make install      # Instala em $GOPATH/bin
make run          # Compila e lança imediatamente
```

### Download Direto

Baixe o binário mais recente em [Releases](https://github.com/apavanello/kboot/releases) e coloque no seu `PATH`.

## Uso

### Iniciar k9s com Todos os Clusters

```bash
./kboot
```

Abre a tela de carregamento TUI, autentica em todos os clusters configurados em paralelo, gera kubeconfigs temporários e lança o k9s.

### Target um Cluster Específico

```bash
./kboot --cluster staging
```

Pula a seleção TUI e inicia apenas o cluster especificado. Implica `--non-interactive`.

### Modo Headless (Scripting / CI/CD)

```bash
./kboot --headless
```

Gera kubeconfigs e imprime o caminho `KUBECONFIG` no stdout. Não lança o k9s:

```bash
export KUBECONFIG=$(./kboot --headless)
kubectl get nodes --all-contexts
```

### Modo Não-Interativo

```bash
./kboot --non-interactive
```

Processa todos os clusters configurados sem prompts TUI. Mostra os caminhos dos kubeconfigs ao final.

## Configuração

### Configuração de Clusters

Os clusters são armazenados em `~/.kboot.yaml`:

```yaml
clusters:
  - alias: "prod"
    name: "eks-cluster-production"
    region: "us-east-1"
    profile: "aws-prod"
    optional: false

  - alias: "staging"
    name: "eks-cluster-staging"
    region: "us-east-1"
    profile: "aws-staging"
    optional: true
```

| Campo | Obrigatório | Descrição |
|---|---|---|
| `alias` | Sim | Nome curto e amigável exibido no TUI e usado como contexto k9s |
| `name` | Sim | Nome real do cluster EKS conforme registrado na AWS |
| `region` | Sim | Região AWS onde o cluster está implantado |
| `profile` | Sim | Nome do perfil de credenciais AWS |
| `optional` | Não | Se `true`, o cluster pode ser ignorado no launch sem erro |

### Credenciais AWS Isoladas

Por padrão, o kboot usa seu próprio diretório AWS isolado em `~/.kboot/aws/`:

```
~/.kboot/
├── aws/
│   ├── credentials     # Chaves de acesso AWS
│   ├── config          # Config AWS com overrides de endpoint
│   └── sso/cache/      # Cache de tokens SSO
└── .kboot.yaml         # Definições de clusters
```

Isso mantém o kboot completamente isolado da configuração `~/.aws/` do seu sistema.

Para usar os arquivos AWS do sistema, adicione ao `~/.kboot.yaml`:

```yaml
use_system_aws: true
```

Ou especifique caminhos customizados:

```yaml
aws_credentials_file: /caminho/para/credentials
aws_config_file: /caminho/para/config
aws_sso_cache_dir: /caminho/para/sso/cache
```

### Configuração via CLI

Adicione clusters diretamente pela linha de comando:

```bash
kboot config add --alias prod --name meu-cluster --region us-east-1 --profile aws-prod
kboot config list
kboot config              # Abre o gerenciador TUI
```

## Autenticação

O kboot suporta dois métodos de autenticação, ambos tratados inteiramente pelo AWS SDK Go v2:

### 1. Autenticação SSO (Recomendado)

Quando você configura um perfil SSO na sua config AWS:

```ini
[profile meu-sso-profile]
sso_session = meu-sso
sso_account_id = 123456789012
sso_role_name = EKSAdmin
region = us-east-1

[sso-session meu-sso]
sso_start_url = https://minha-empresa.awsapps.com/start
sso_region = us-east-1
```

O kboot irá:
1. Verificar se existe um token SSO válido no cache
2. Se não, iniciar o fluxo **OAuth 2.0 Device Authorization Grant**
3. Abrir o navegador na página de login SSO da AWS
4. Aguardar aprovação do token via polling
5. Armazenar o token em cache para uso futuro

### 2. Credenciais Estáticas

Configure chaves de acesso no seu arquivo de credenciais:

```ini
[aws-prod]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### Geração de Token EKS

Para autenticação no kubeconfig, o kboot gera tokens STS `GetCallerIdentity` pré-assinados usando AWS Signature V4. O header `x-k8s-aws-id` é assinado como parte da requisição canônica, produzindo um token `k8s-aws-v1.*` válido que o servidor API do EKS aceita. Isso substitui a chamada CLI tradicional `aws eks get-token`.

## Comandos CLI

### `kboot` — Launch

```bash
./kboot                           # Modo interativo com seleção de clusters
./kboot --cluster staging         # Target single cluster
./kboot --non-interactive         # Processa todos os clusters sem TUI
./kboot --headless                # Gera kubeconfigs e imprime path
```

### `kboot config` — Gerenciar

```bash
kboot config                      # Abre gerenciador TUI
kboot config list                 # Lista todos os clusters configurados
kboot config add --alias X --name Y --region Z --profile P
```

### `kboot token` — Gerar Token de Auth EKS

Comando interno usado como plugin exec do kubeconfig:

```bash
./kboot token --cluster-name meu-cluster --region us-east-1 --profile meu-profile
```

Saída:
```json
{
  "kind": "ExecCredential",
  "apiVersion": "client.authentication.k8s.io/v1beta1",
  "status": {
    "token": "k8s-aws-v1.aHR0cHM6Ly9zdHMudXMtZWFzdC0xLmFtYXpvbmF3cy5jb20v..."
  }
}
```

## Infraestrutura de Testes

O kboot inclui um ambiente de testes completo usando **LocalStack** (AWS mock) e **kind** (Kubernetes real):

```bash
make infra           # Inicia LocalStack + cria 2 clusters kind
make infra-status    # Mostra status atual da infraestrutura
make infra-cleanup   # Destrói tudo
make docker-up       # Inicia container LocalStack
make docker-down     # Para container LocalStack
```

O ambiente de teste provisiona:
- **LocalStack** — Mock de AWS EKS, IAM, STS, EC2, S3, CloudWatch
- **kind cluster staging** — Kubernetes real na porta 6443
- **kind cluster production** — Kubernetes real na porta 6444
- **Terraform** — Definições de clusters EKS no LocalStack

Veja [`infra/`](infra/) para todos os arquivos de configuração.

## Testes

```bash
make test              # Testes unitários Go com race detection
make test-verbose      # Testes unitários com output verboso
make test-coverage     # Testes unitários com relatório de cobertura
make test-e2e          # Testes unitários CLI (config CRUD, flags, token)
make test-integration  # Suite completa de integração (11 fases)
```

### Fases do Teste de Integração

| Fase | O que Testa |
|---|---|
| 1. Build | Binário compila e é executável |
| 2. Pré-requisitos | Docker, kind, Terraform, kubectl disponíveis |
| 3. LocalStack | Container rodando, serviço EKS saudável |
| 4. Terraform EKS | Clusters mock criados no LocalStack |
| 5. kind Clusters | Clusters Kubernetes reais acessíveis |
| 6. AWS Isolado | Credenciais e config em `~/.kboot/aws/` |
| 7. kboot Config | Clusters adicionados e listados via CLI |
| 8. Não-Interativo | Flags `--cluster` e `--non-interactive` |
| 9. Geração de Token | Token `k8s-aws-v1.*` com headers assinados |
| 10. kubectl | Conectividade de nodes e pods em ambos os clusters |
| 11. Multi-contexto | Kubeconfig combinado com ambos os contextos |

## Desenvolvimento

### Targets do Makefile

```bash
make build           # Compila para ./bin/kboot
make run             # Compila e lança
make install         # Instala em $GOPATH/bin
make clean           # Remove artefatos de build

make fmt             # Formata código com gofmt
make vet             # Executa go vet
make lint            # Executa golangci-lint
make check           # fmt + vet + lint
make tidy            # Limpa dependências do go.mod

make test            # Executa testes com race detection
make test-verbose    # Executa testes (output verboso)
make test-coverage   # Executa testes com relatório de cobertura
make test-e2e        # Testes unitários CLI
make test-integration # Suite completa de integração

make install-yolo    # Instalação automática de todas as dependências + infra

make infra           # Configura ambiente de teste LocalStack + kind
make infra-cleanup   # Destrói ambiente de teste
make infra-status    # Mostra status atual da infra
make infra-destroy   # Destruição completa (Terraform + kind)
make docker-up       # Inicia container LocalStack
make docker-down     # Para container LocalStack

make help            # Mostra todos os targets disponíveis
```

### Estrutura do Projeto

```
kboot/
├── cmd/kboot/
│   └── main.go              # Entrada CLI, parsing de flags
├── internal/
│   ├── app/
│   │   └── orchestrator.go  # Worker pool paralelo
│   ├── aws/
│   │   ├── client.go        # Cliente AWS SDK + geração de token EKS
│   │   └── sso.go           # Autorização de dispositivo SSO OIDC
│   ├── config/
│   │   └── config.go        # Carregamento de config + paths AWS isolados
│   ├── kube/
│   │   └── generator.go     # Geração de kubeconfig
│   └── ui/
│       ├── dashboard.go     # Tela de carregamento
│       └── manager.go       # Gerenciador TUI
├── infra/
│   ├── main.tf              # Terraform: VPC, IAM, 2 clusters EKS
│   ├── variables.tf         # Variáveis Terraform
│   ├── outputs.tf           # Outputs Terraform
│   ├── bootstrap.sh         # Script de setup
│   ├── docker-compose.yml   # Container LocalStack
│   ├── kind-staging.yaml    # Config cluster kind staging
│   └── kind-production.yaml # Config cluster kind production
├── scripts/
│   ├── install.sh           # Instalador YOLO
│   ├── test-e2e.sh          # Testes unitários CLI
│   └── test-integration.sh  # Suite completa de integração
├── Makefile
├── go.mod
├── .kboot.yaml.example
├── README.md
└── README.pt-br.md
```

## Segurança

- **Sem armazenamento de credenciais** — O kboot nunca armazena chaves de acesso ou segredos AWS. Todo o gerenciamento de credenciais é delegado ao AWS SDK e aos arquivos padrão AWS.
- **Isolado por padrão** — O kboot usa `~/.kboot/aws/` para todos os arquivos AWS, deixando seu `~/.aws/` do sistema intocado.
- **Kubeconfigs temporários** — Os arquivos kubeconfig gerados são armazenados em `/tmp/kboot/` e limpos após a sessão. Seu `~/.kube/config` nunca é modificado.
- **Cache de tokens SSO** — Tokens SSO são armazenados com permissões adequadas (0600) e rastreamento de expiração, seguindo o formato padrão do AWS SDK.
- **Tokens pré-assinados** — Tokens de autenticação EKS são gerados via requisições STS pré-assinadas com expiração de 60 segundos, minimizando a janela para ataques de replay.
- **Sem injeção de shell** — Todas as interações com AWS usam o SDK diretamente. Sem chamadas de subprocesso para a CLI `aws`, eliminando vetores de injeção de shell.

## Licença

MIT
