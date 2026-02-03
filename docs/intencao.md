# Intenção do Projeto - kboot

## Visão Geral
**kboot** é uma ferramenta CLI (Go) para simplificar e acelerar o acesso a múltiplos clusters Amazon EKS em diferentes contas AWS utilizando AWS SSO.

## Problemas Identificados (Dores)
1. **Performance e Bloqueios:** O processo atual de login não é paralelo.
2. **Resiliência:** Se uma conta AWS estiver inativa ou falhar, todo o processo é interrompido, impedindo o acesso aos clusters das contas saudáveis.
3. **Acoplamento Excessivo:** A ferramenta atualmente força a abertura do `k9s` após a geração do kubeconfig, impedindo o uso de outras ferramentas (kubectl puro, Lens, etc.) ou apenas a geração do arquivo de configuração.

## Objetivos da Evolução
- **Paralelismo:** Implementar execução concorrente para validação de sessões e geração de configurações.
- **Falha Parcial (Graceful Degradation):** Permitir que o processo continue e gere o kubeconfig para as contas que funcionaram, apenas logando o erro das que falharam.
- **Flexibilidade de Execução:** Tornar a abertura do `k9s` opcional ou configurável, permitindo que o usuário escolha apenas gerar o arquivo de configuração ou executar outra ferramenta.

## Público-Alvo
- Uso pessoal e focado em alta produtividade DevOps.

## Stack Tecnológico
- Linguagem: Go (100%)
