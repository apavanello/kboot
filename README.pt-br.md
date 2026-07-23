# kboot

<p align="center">
  <img src="./assets/readme/hero.svg" width="100%" alt="kboot — Conecte-se a todos os seus clusters EKS de uma vez">
</p>

[![Latest Release](https://img.shields.io/github/v/release/apavanello/kboot?sort=semver&color=blue)](https://github.com/apavanello/kboot/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/apavanello/kboot)](https://go.dev)
[![License](https://img.shields.io/github/license/apavanello/kboot)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/apavanello/kboot)](https://goreportcard.com/report/github.com/apavanello/kboot)
[![Release](https://github.com/apavanello/kboot/actions/workflows/release.yml/badge.svg)](https://github.com/apavanello/kboot/actions/workflows/release.yml)

## O que é kboot?

**kboot** é uma ferramenta CLI de DevOps que simplifica o gerenciamento de múltiplos clusters Amazon EKS em diferentes contas AWS. Automatiza a autenticação via AWS SSO, gera kubeconfigs com contexto para os clusters selecionados em paralelo, e inicia o [`k9s`](https://k9scli.io/) com todos os clusters imediatamente acessíveis — sem poluir seu `~/.kube/config`.

**Sem dependência de AWS CLI.** Todas as interações com AWS usam o AWS SDK Go v2 diretamente.

## Instalação

Um comando — baixa o binário da última release e instala em `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/apavanello/kboot/main/scripts/install.sh | bash
```

Ou compile a partir do código fonte:

```bash
git clone https://github.com/apavanello/kboot
cd kboot
make install
```

<details>
<summary>Todas as opções de instalação</summary>

### Setup Completo (YOLO)

Instala todas as dependências (Go, Docker, kubectl, kind, Terraform, k9s), compila o kboot, configura LocalStack + kind clusters:

```bash
curl -fsSL https://raw.githubusercontent.com/apavanello/kboot/main/scripts/install.sh | bash -s full
```

### Comandos do Instalador

| Comando | O que faz |
|---|---|
| `curl ... \| bash` | Baixa o binário da última release |
| `curl ... \| bash -s update` | Atualiza para a última release |
| `curl ... \| bash -s full` | Instala todas as dependências + kboot + infra de teste |
| `curl ... \| bash -s help` | Mostra todas as opções |

### Usando Make

```bash
make build        # Compila para ./bin/kboot
make install      # Instala em ~/.local/bin
make run          # Compila e lança imediatamente
```

### Download Direto

Baixe de [Releases](https://github.com/apavanello/kboot/releases) e coloque no seu `PATH`.

</details>

## Uso

### Iniciar k9s com todos os clusters

```bash
kboot
```

Abre a tela de carregamento TUI, autentica em todos os clusters configurados em paralelo, gera kubeconfigs temporários e lança o k9s.

### Target um cluster específico

```bash
kboot --cluster staging
```

### Modo headless (CI/CD)

```bash
export KUBECONFIG=$(kboot --headless)
kubectl get nodes --all-contexts
```

### Modo não-interativo

```bash
kboot --non-interactive
```

## Como funciona

<p align="center">
  <img src="./assets/readme/workflow.svg" width="100%" alt="Diagrama do fluxo de trabalho do kboot">
</p>

1. Carrega `~/.kboot.yaml` → lista de clusters
2. Para cada cluster (paralelo, 5 workers):
   - Cria cliente AWS SDK (profile + region)
   - Verifica identidade (STS GetCallerIdentity)
   - Se auth falha → login SSO (device auth flow)
   - Descreve cluster (EKS API) → endpoint + CA
3. Gera kubeconfig por cluster (arquivos temp)
4. Lança k9s com `KUBECONFIG=temp1:temp2:...`
5. Exec substitui processo kboot pelo k9s

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
| `alias` | Sim | Nome curto exibido no TUI e usado como contexto k9s |
| `name` | Sim | Nome real do cluster EKS na AWS |
| `region` | Sim | Região AWS |
| `profile` | Sim | Nome do perfil de credenciais AWS |
| `optional` | Não | Se `true`, pode ser ignorado sem erro |

<details>
<summary>Credenciais AWS isoladas (padrão)</summary>

kboot usa seu próprio diretório AWS isolado em `~/.kboot/aws/`:

```
~/.kboot/
├── aws/
│   ├── credentials     # Chaves de acesso AWS
│   ├── config          # Config AWS com overrides
│   └── sso/cache/      # Cache de tokens SSO
└── .kboot.yaml         # Definições de clusters
```

Seu `~/.aws/` do sistema nunca é tocado.

Para usar os arquivos AWS do sistema:

```yaml
use_system_aws: true
```

Ou especifique caminhos customizados:

```yaml
aws_credentials_file: /caminho/para/credentials
aws_config_file: /caminho/para/config
aws_sso_cache_dir: /caminho/para/sso/cache
```

</details>

<details>
<summary>Configuração via CLI</summary>

```bash
kboot config add --alias prod --name meu-cluster --region us-east-1 --profile aws-prod
kboot config list
kboot config              # Abre o gerenciador TUI
```

</details>

## Segurança

<p align="center">
  <img src="./assets/readme/security.svg" width="100%" alt="Diagrama de isolamento de segurança do kboot">
</p>

| Recurso | Descrição |
|---|---|
| **Sem armazenamento de credenciais** | Gerenciamento delegado ao AWS SDK |
| **Isolado por padrão** | Usa `~/.kboot/aws/`, nunca toca em `~/.aws/` |
| **Kubeconfigs temporários** | Armazenados em `/tmp/kboot/`, limpos após sessão |
| **Sem injeção de shell** | Todas as interações com AWS usam SDK diretamente |
| **Tokens pré-assinados** | Expiração de 60 segundos minimiza janela de replay |

## Autenticação

### Autenticação SSO (Recomendado)

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
2. Se não, iniciar o fluxo OAuth 2.0 Device Authorization Grant
3. Abrir o navegador na página de login SSO da AWS
4. Aguardar aprovação do token via polling
5. Armazenar o token em cache para uso futuro

### Credenciais Estáticas

```ini
[aws-prod]
aws_access_key_id = AKIAIOSFODNN7EXAMPLE
aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

## Comandos CLI

```bash
kboot                           # Modo interativo com seleção de clusters
kboot --cluster staging         # Target single cluster
kboot --non-interactive         # Processa todos os clusters sem TUI
kboot --headless                # Gera kubeconfigs e imprime path

kboot config                    # Abre gerenciador TUI
kboot config list               # Lista todos os clusters configurados
kboot config add --alias X --name Y --region Z --profile P
```

## Testes

kboot inclui um ambiente de testes completo usando LocalStack e kind:

```bash
make test              # Testes unitários com race detection
make test-e2e          # Testes unitários CLI
make test-integration  # Suite completa de integração (11 fases)

make infra             # Inicia LocalStack + kind clusters
make infra-cleanup     # Destrói tudo
```

<details>
<summary>Fases do teste de integração</summary>

| Fase | O que Testa |
|---|---|
| 1. Build | Binário compila e é executável |
| 2. Pré-requisitos | Docker, kind, Terraform, kubectl disponíveis |
| 3. LocalStack | Container rodando, serviço EKS saudável |
| 4. Terraform EKS | Clusters mock criados |
| 5. kind Clusters | Clusters Kubernetes reais acessíveis |
| 6. AWS Isolado | Credenciais e config em `~/.kboot/aws/` |
| 7. kboot Config | Clusters adicionados e listados via CLI |
| 8. Não-Interativo | Flags `--cluster` e `--non-interactive` |
| 9. Geração de Token | Token `k8s-aws-v1.*` com headers assinados |
| 10. kubectl | Conectividade de nodes e pods |
| 11. Multi-contexto | Kubeconfig combinado com ambos os contextos |

</details>

## Desenvolvimento

### Estrutura do Projeto

```
kboot/
├── cmd/kboot/main.go           # Entrada CLI
├── internal/
│   ├── app/orchestrator.go     # Worker pool paralelo
│   ├── aws/client.go           # Cliente AWS SDK + token EKS
│   ├── aws/sso.go              # Autorização SSO OIDC
│   ├── config/config.go        # Carregamento de config
│   ├── kube/generator.go       # Geração de kubeconfig
│   └── ui/                     # TUI Bubble Tea
├── infra/                      # Setup LocalStack + kind
├── scripts/                    # Scripts de instalação + teste
└── Makefile
```

### Targets do Makefile

```bash
make build           # Compila
make run             # Compila e lança
make install         # Instala em ~/.local/bin
make fmt             # Formata código
make lint            # Executa golangci-lint
make test            # Executa testes com race detection
make infra           # Configura ambiente de teste
```

## Licença

Apache-2.0
