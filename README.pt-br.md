# kboot (Português)

**kboot** é uma ferramenta CLI de DevOps projetada para simplificar o gerenciamento de múltiplos clusters Amazon EKS espalhados por diferentes contas AWS. Ela automatiza a autenticação via AWS SSO, gera kubeconfigs com contexto para clusters selecionados em paralelo e inicia o `k9s` (ou um shell) com todos os clusters acessíveis imediatamente.

## Funcionalidades

- **Autenticação Inteligente**: Verifica se sua sessão AWS SSO é válida em `~/.aws/sso/cache`. Se expirada, executa `aws sso login` automaticamente.
- **Sincronização Paralela**: Busca detalhes dos clusters e gera kubeconfigs para múltiplos clusters simultaneamente usando Goroutines.
- **Aliasing de Contexto**: Mapeia ARNs complexos da AWS para apelidos curtos e amigáveis (ex: `prod`, `staging`) para seus contextos Kubernetes.
- **Zero Poluição**: **Não** modifica seu `~/.kube/config` principal. Gera arquivos temporários e define a variável de ambiente `KUBECONFIG` apenas para a sessão atual.
- **Multi-plataforma**: Funciona em Windows, Linux e macOS.

## Instalação

### Pré-requisitos
- Go 1.21+
- AWS CLI v2
- `k9s` (recomendado) ou `kubectl`

### Compilar do Código Fonte
```bash
git clone https://github.com/apavanello/kboot
cd kboot
go build -o kboot .
```

## Configuração

Crie um arquivo de configuração em `~/.kboot.yaml`:

```yaml
sso_session: "minha-sessao-sso" # Deve coincidir com o nome da sessão no ~/.aws/config
clusters:
  - alias: "prod"         # Nome curto para o contexto (ex: exibido no k9s)
    profile: "aws-prod"   # Profile AWS do ~/.aws/config
    region: "us-east-1"
    name: "eks-cluster-production" # Nome real do cluster EKS
  - alias: "staging"
    profile: "aws-staging"
    region: "us-east-1"
    name: "eks-cluster-staging"
```

## Uso

Simplesmente execute o binário:

```bash
./kboot
```

A ferramenta irá:
1. Garantir que você está logado no AWS SSO.
2. Gerar kubeconfigs para `prod` e `staging` em um diretório temporário.
3. Iniciar o `k9s` com acesso a ambos os clusters.

## Licença

MIT
