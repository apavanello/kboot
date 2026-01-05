# kboot (Português)

**kboot** é uma ferramenta CLI de DevOps projetada para simplificar o gerenciamento de múltiplos clusters Amazon EKS espalhados por diferentes contas AWS. Ela automatiza a autenticação via AWS SSO, gera kubeconfigs com contexto para clusters selecionados em paralelo e inicia o `k9s` (ou um shell) com todos os clusters acessíveis imediatamente.

## Funcionalidades

- **Dashboard TUI Unificado** (v2.3.0): Gerencie clusters e credenciais AWS de uma única interface de terminal intuitiva
- **Autenticação Inteligente**: Verifica se sua sessão AWS SSO é válida. Se expirada, executa `aws sso login` automaticamente
- **Sincronização Paralela**: Busca detalhes dos clusters e gera kubeconfigs para múltiplos clusters simultaneamente
- **Aliasing de Contexto**: Mapeia ARNs complexos da AWS para apelidos curtos e amigáveis (ex: `prod`, `staging`)
- **Zero Poluição**: **Não** modifica seu `~/.kube/config` principal. Gera configs temporários apenas para a sessão
- **Multi-plataforma**: Funciona em Windows, Linux e macOS

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

## Uso

### Iniciar k9s com todos os clusters
```bash
./kboot
```
Sincroniza todos os clusters configurados e abre o k9s.

### Dashboard de Configuração (TUI)
```bash
./kboot config
```
Abre o dashboard unificado de gerenciamento onde você pode:

**Gerenciar Clusters:**
- `a` - Adicionar novo cluster
- `e` / `Enter` - Editar cluster selecionado
- `c` - Duplicar cluster selecionado
- `d` - Deletar cluster selecionado

**Gerenciar Credenciais AWS:**
- Credenciais estáticas (`~/.aws/credentials`)
- Perfis SSO (`~/.aws/config`)

**Navegação:**
- `Tab` / `Shift+Tab` - Navegar campos do formulário
- `Esc` - Voltar / Cancelar
- `q` - Sair

## Configuração

Os clusters são armazenados em `~/.kboot.yaml`:

```yaml
sso_session: "minha-sessao-sso"
clusters:
  - alias: "prod"
    profile: "aws-prod"
    region: "us-east-1"
    name: "eks-cluster-production"
  - alias: "staging"
    profile: "aws-staging"
    region: "us-east-1"
    name: "eks-cluster-staging"
```

Você também pode gerenciar este arquivo através da TUI com `kboot config`.

## Licença

MIT
