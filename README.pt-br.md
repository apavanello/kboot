# kboot

> **Conecte-se a todos os seus clusters EKS de uma vez.**

**kboot** é uma ferramenta CLI de DevOps projetada para simplificar o gerenciamento de múltiplos clusters Amazon EKS em diferentes contas AWS. Automatiza a autenticação via AWS SSO, gera kubeconfigs com contexto para os clusters selecionados em paralelo, e inicia o [`k9s`](https://k9scli.io/) com todos os clusters imediatamente acessíveis — sem poluir seu `~/.kube/config`.

## Índice

- [Funcionalidades](#funcionalidades)
- [Como Funciona](#como-funciona)
- [Arquitetura](#arquitetura)
- [Pré-requisitos](#pré-requisitos)
- [Instalação](#instalação)
- [Uso](#uso)
- [Configuração](#configuração)
- [Autenticação](#autenticação)
- [Comandos CLI](#comandos-cli)
- [Infraestrutura de Testes](#infraestrutura-de-testes)
- [Desenvolvimento](#desenvolvimento)
- [Estrutura do Projeto](#estrutura-do-projeto)
- [Segurança](#segurança)
- [Licença](#licença)

## Funcionalidades

- **Dashboard TUI Unificado** — Gerencie clusters, credenciais estáticas e perfis SSO em uma única interface de terminal construída com [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Autenticação via AWS SDK Puro** — Login SSO e geração de tokens EKS usam exclusivamente AWS SDK Go v2. **Sem dependência de AWS CLI.**
- **Sincronização Paralela** — Autentica e busca metadados de múltiplos clusters simultaneamente com pool de workers configurável (padrão: 5 workers)
- **Aliasing de Contexto** — Mapeia ARNs complexos (`arn:aws:eks:us-east-1:123456789012:cluster/prod`) para apelidos curtos e amigáveis (`prod`, `staging`, `dev`)
- **Zero Poluição** — **Não** modifica seu `~/.kube/config`. Usa arquivos kubeconfig temporários em `/tmp/kboot/` que são limpos após a sessão
- **Modo Headless** — Gera kubeconfigs e sai sem lançar o k9s, ideal para pipelines CI/CD e scripts
- **Multi-plataforma** — Funciona em Windows, Linux e macOS
- **Ambiente de Testes LocalStack** — Infraestrutura completa com Terraform + kind para testar sem tocar em recursos AWS reais

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

## Arquitetura

```
kboot/
├── cmd/kboot/main.go          # Entrada CLI, parsing de flags, launch do k9s
├── internal/
│   ├── app/
│   │   └── orchestrator.go    # Worker pool, processamento paralelo
│   ├── aws/
│   │   ├── client.go          # Cliente AWS SDK, geração de token EKS (v4 presign)
│   │   └── sso.go             # Fluxo de autorização de dispositivo SSO OIDC
│   ├── config/
│   │   └── config.go          # Parsing e validação de ~/.kboot.yaml
│   ├── kube/
│   │   └── generator.go       # Geração de YAML kubeconfig
│   └── ui/
│       ├── dashboard.go       # Tela de carregamento com barras de progresso
│       └── manager.go         # Gerenciamento TUI de clusters/credenciais
└── infra/                     # Ambiente de teste Terraform + kind
```

## Pré-requisitos

| Dependência | Obrigatório | Finalidade |
|---|---|---|
| **Go 1.21+** | Apenas build | Compilar a partir do código fonte |
| **k9s** | Runtime (recomendado) | UI de terminal para Kubernetes |
| **AWS SDK Go v2** | Runtime (vendored) | Autenticação, API EKS — sem CLI externa |

> **Nota:** kboot **não** requer a AWS CLI. Todas as interações com AWS usam o AWS SDK Go v2 diretamente, incluindo login SSO via OAuth 2.0 Device Authorization Grant flow.

## Instalação

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

Abre a tela de carregamento TUI, autentica em todos os clusters configurados em paralelo, gera kubeconfigs temporários e lança o k9s com todos os clusters acessíveis via troca de contexto.

### Modo Headless (Scripting / CI/CD)

```bash
./kboot --headless
```

Gera kubeconfigs e imprime o caminho `KUBECONFIG` no stdout. Não lança o k9s. Útil para pipelines:

```bash
export KUBECONFIG=$(./kboot --headless)
kubectl get nodes --all-contexts
```

### Dashboard de Configuração

```bash
./kboot config
```

Abre o gerenciador TUI com três seções:

| Menu | Descrição |
|------|-----------|
| **Gerenciar Clusters** | Adicionar, editar, duplicar ou deletar definições de clusters EKS |
| **Credenciais Estáticas** | Gerenciar perfis em `~/.aws/credentials` diretamente |
| **Perfis SSO** | Gerenciar perfis de sessão SSO em `~/.aws/config` |

### Teclas de Atalho (Gerenciador TUI)

| Tecla | Ação |
|-------|------|
| `a` | Adicionar novo item |
| `e` / `Enter` | Editar item selecionado |
| `c` | Duplicar item selecionado |
| `d` | Deletar item selecionado |
| `Tab` | Próximo campo |
| `Shift+Tab` | Campo anterior |
| `Esc` | Voltar / Cancelar |
| `q` | Sair |

## Configuração

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
| `profile` | Sim | Nome do perfil de credenciais AWS (de `~/.aws/credentials` ou `~/.aws/config`) |
| `optional` | Não | Se `true`, o cluster pode ser ignorado no launch sem erro |

## Autenticação

kboot suporta dois métodos de autenticação, ambos tratados inteiramente pelo AWS SDK Go v2:

### 1. Autenticação SSO (Recomendado)

Quando você configura um perfil SSO em `~/.aws/config`:

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
1. Verificar se existe um token SSO válido em `~/.aws/sso/cache/`
2. Se não, iniciar o fluxo **OAuth 2.0 Device Authorization Grant**
3. Abrir o navegador na página de login SSO da AWS
4. Aguardar aprovação do token via polling
5. Armazenar o token em cache para uso futuro

### 2. Credenciais Estáticas

Configure chaves de acesso em `~/.aws/credentials`:

```ini
[aws-prod]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### Geração de Token EKS

Para autenticação no kubeconfig, o kboot gera tokens STS `GetCallerIdentity` pré-assinados usando AWS Signature V4. O header `x-k8s-aws-id` é assinado como parte da requisição canônica, produzindo um token `k8s-aws-v1.*` válido que o servidor API do EKS aceita. Isso substitui a chamada CLI tradicional `aws eks get-token`.

## Comandos CLI

### `kboot` — Launch

Inicia todos os clusters configurados e lança o k9s.

```bash
./kboot                    # Modo interativo com seleção de clusters
./kboot --headless         # Gera kubeconfigs e sai
```

### `kboot config` — Gerenciar

Abre o gerenciador de configuração TUI.

```bash
./kboot config
```

### `kboot token` — Gerar Token de Auth EKS

Comando interno usado como plugin exec do kubeconfig. Gera um token STS pré-assinado para autenticação EKS.

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
```

O ambiente de teste provisiona:
- **LocalStack** — Mock de AWS EKS, IAM, STS, EC2, S3, CloudWatch
- **kind cluster staging** — Kubernetes real na porta 6443
- **kind cluster production** — Kubernetes real na porta 6444
- **Terraform** — Definições de clusters EKS no LocalStack

Veja [`infra/`](infra/) para todos os arquivos de configuração.

## Desenvolvimento

### Targets do Makefile

```bash
make build          # Compila para ./bin/kboot
make run            # Compila e lança
make install        # Instala em $GOPATH/bin
make clean          # Remove artefatos de build

make fmt            # Formata código com gofmt
make vet            # Executa go vet
make lint           # Executa golangci-lint
make check          # fmt + vet + lint
make tidy           # Limpa dependências do go.mod

make test           # Executa testes com race detection
make test-verbose   # Executa testes (output verboso)
make test-coverage  # Executa testes com relatório de cobertura

make infra          # Configura ambiente de teste LocalStack + kind
make infra-cleanup  # Destrói ambiente de teste
make docker-up      # Inicia container LocalStack
make docker-down    # Para container LocalStack

make help           # Mostra todos os targets disponíveis
```

### Estrutura do Projeto

```
kboot/
├── cmd/kboot/
│   └── main.go              # Entrada CLI
├── internal/
│   ├── app/
│   │   └── orchestrator.go  # Worker pool paralelo
│   ├── aws/
│   │   ├── client.go        # Cliente AWS SDK + geração de token EKS
│   │   └── sso.go           # Autorização de dispositivo SSO OIDC
│   ├── config/
│   │   └── config.go        # Carregamento de configuração
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
├── Makefile
├── go.mod
├── .kboot.yaml.example
├── README.md
└── README.pt-br.md
```

## Segurança

- **Sem armazenamento de credenciais** — O kboot nunca armazena chaves de acesso ou segredos AWS. Todo o gerenciamento de credenciais é delegado ao AWS SDK e aos arquivos padrão `~/.aws/`.
- **Kubeconfigs temporários** — Os arquivos kubeconfig gerados são armazenados em `/tmp/kboot/` e limpos após a sessão. Seu `~/.kube/config` nunca é modificado.
- **Cache de tokens SSO** — Tokens SSO são armazenados em `~/.aws/sso/cache/` com permissões adequadas (0600) e rastreamento de expiração, seguindo o formato padrão do AWS SDK.
- **Tokens pré-assinados** — Tokens de autenticação EKS são gerados via requisições STS pré-assinadas com expiração de 60 segundos, minimizando a janela para ataques de replay.
- **Sem injeção de shell** — Todas as interações com AWS usam o SDK diretamente. Sem chamadas de subprocesso para a CLI `aws`, eliminando vetores de injeção de shell.

## Licença

MIT
