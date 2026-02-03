# Features do Projeto - kboot

## 1. Login SSO Paralelo
**Descrição:** O sistema deve realizar a autenticação AWS SSO para múltiplos perfis simultaneamente, em vez de sequencialmente.
**Objetivo:** Reduzir drasticamente o tempo de inicialização quando se tem muitas contas AWS configuradas.
**Comportamento:**
- Identificar todos os perfis sso únicos necessários para os clusters configurados.
- Disparar processos de validação/login em goroutines separadas.
- Aguardar a conclusão de todos (com timeout).

## 2. Tolerância a Falhas (Graceful Degradation)
**Descrição:** O processo de inicialização não deve ser abortado se uma ou mais contas AWS falharem na autenticação.
**Objetivo:** Permitir o acesso aos ambientes que estão saudáveis mesmo que uma conta específica esteja indisponível (ex: conta suspensa, erro de rede pontual).
**Comportamento:**
- Capturar erros individuais durante o login paralelo.
- Registrar o erro no console (stderr) mas continuar o processo para as contas bem-sucedidas.
- O kubeconfig final deve conter apenas os clusters das contas validadas com sucesso.

## 3. Modo "Apenas Configuração" (Headless)
**Descrição:** Implementar uma flag (ex: `--no-k9s` ou `--dry-run`) para gerar o arquivo kubeconfig e encerrar a execução, sem abrir a interface do k9s.
**Objetivo:** Permitir o uso do kboot como um gerador de credenciais para outras ferramentas (Lens, kubectl puro, scripts de automação).
**Comportamento:**
- Se a flag for passada, o programa gera o arquivo, imprime o caminho do arquivo gerado e encerra com código 0.

## 4. Feedback Visual de Progresso
**Descrição:** Melhorar o output do CLI durante a inicialização para mostrar o status de cada conta em tempo real (ou tabelado ao final).
**Objetivo:** Dar visibilidade ao usuário sobre qual conta específica falhou, já que agora o erro não trava o app.
