# kboot (Português)

**kboot** é uma ferramenta CLI de DevOps para simplificar o gerenciamento de múltiplos clusters Amazon EKS em diferentes contas AWS. Automatiza a autenticação via AWS SSO, gera kubeconfigs em paralelo e inicia o `k9s` com todos os clusters acessíveis.

## Funcionalidades

- **Dashboard TUI Unificado** (v2.3): Gerencie clusters e credenciais AWS de uma única interface
- **Autenticação Inteligente**: Valida sessões SSO automaticamente usando AWS SDK Go v2 (sem necessidade de AWS CLI)
- **Sincronização Paralela**: Gera kubeconfigs para múltiplos clusters simultaneamente
- **Aliasing de Contexto**: Mapeia ARNs complexos para apelidos curtos (ex: `prod`, `staging`)
- **Zero Poluição**: **Não** modifica seu `~/.kube/config`. Usa configs temporários
- **Multi-plataforma**: Windows, Linux e macOS

## Instalação

### Pré-requisitos
- Go 1.21+
- `k9s` (recomendado)

### Compilar
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

### Dashboard de Configuração
```bash
./kboot config
```

Abre a TUI unificada com três opções:

| Menu | Descrição |
|------|-----------|
| **Gerenciar Clusters** | Adicionar, editar, duplicar ou deletar clusters EKS |
| **Credenciais Estáticas** | Gerenciar perfis em `~/.aws/credentials` |
| **Perfis SSO** | Gerenciar perfis SSO em `~/.aws/config` |

### Teclas de Atalho

| Tecla | Ação |
|-------|------|
| `a` | Adicionar novo |
| `e` / `Enter` | Editar selecionado |
| `c` | Duplicar selecionado |
| `d` | Deletar selecionado |
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
  - alias: "staging"
    name: "eks-cluster-staging"
    region: "us-east-1"
    profile: "aws-staging"
```

> **Nota:** Credenciais AWS e perfis SSO são gerenciados separadamente em `~/.aws/credentials` e `~/.aws/config`. Use `kboot config` para configurar tudo pela TUI.

## Licença

MIT
