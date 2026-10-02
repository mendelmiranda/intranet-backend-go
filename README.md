# Sistema Corporativo Go

Conversão do projeto multi-service para Go, usando Fiber v3 sobre `fasthttp`.

## Arquitetura

O repositório usa um Go Workspace (`go.work`) com cinco módulos independentes:

- `shared-common`: tipos compartilhados;
- `usuarios-service`: API na porta `8081`;
- `ferias-service`: API na porta `8082`;
- `servidor-service`: API na porta `8083`;
- `contracheque-service`: API na porta `8084` (módulo de contracheque do S3i).

Cada serviço produz um binário próprio e pode ser iniciado, parado e publicado separadamente.

## Requisitos

- Go 1.25 ou superior;
- Bash para usar `servicos.sh`;
- `curl`, opcionalmente, para a verificação de saúde;
- Docker e Docker Compose, opcionalmente.

## Executar localmente

Conceda permissão de execução ao script:

```bash
chmod +x servicos.sh
```

Compile e inicie todos os serviços:

```bash
./servicos.sh iniciar
```

Veja o estado dos processos:

```bash
./servicos.sh status
```

Pare apenas o serviço de servidores:

```bash
./servicos.sh parar servidor
```

Outros exemplos:

```bash
./servicos.sh iniciar ferias
./servicos.sh reiniciar usuarios
./servicos.sh parar tudo
./servicos.sh logs servidor
./servicos.sh testar
./servicos.sh benchmark
```

Os PIDs, binários e logs são mantidos em `.runtime/`.

## Contracheque (`contracheque-service`)

Reimplementa em Go o módulo `contracheque` do backend Spring Boot (S3i), mantendo o contrato
consumido pelo frontend Next.js. Rotas (as de `/api/contra-cheque/**` exigem JWT com `ROLE_DASHBOARD`):

| Método | Rota | Descrição |
|---|---|---|
| GET | `/api/contra-cheque/intranet/mes/{mes}/ano/{ano}/matricula/{matricula}` | `{"first": cabeçalho, "second": linhas}` |
| GET | `/api/contra-cheque/meses-ecidade/{ano}/{matricula}` | meses disponíveis (cache) |
| GET | `/api/contra-cheque/meses/{ano}` | meses disponíveis do ano (cache) |
| GET | `/api/contra-cheque/{mes}/{ano}/{matricula}` | rubricas do mês |
| POST | `/api/contra-cheque` | rubricas (corpo: `mes`, `ano`, `matriculaSistema`) |
| POST | `/api/contra-cheque/decimo` | rubricas do 13º; vazio responde 400 |
| POST | `/api/contra-cheque/gerar` | PDF com QR Code (corpo: `mes`, `ano`, `matricula`) |
| GET | `/api/contra-cheque/informacao` | bean `ContraCheque` padrão |
| GET | `/verifica-contracheque/{codigo}` | verificação pública do código/QR Code |

Configuração por variáveis de ambiente (use um `.env` na raiz, já ignorado pelo Git; `servicos.sh`,
`air` e o Docker Compose o carregam):

```bash
JWT_SIGNING_KEY=<mesma chave do Spring (Constants.SIGNING_KEY), em Base64 como o jjwt 0.9 espera>
MSSQL_DSN='sqlserver://USUARIO:SENHA@HOST:1597?database=GP0001_TCEAP'
MYSQL_DSN='USUARIO:SENHA@tcp(HOST:3306)/internet_novo?parseTime=true&loc=UTC&clientFoundRows=true'
# opcionais
CONTRACHEQUE_VERIFICA_URL=http://10.10.3.5:3000/resposta-contra-cheque
CORS_ALLOWED_ORIGINS=http://localhost:3000
CACHE_TTL_MINUTES=5
TIMEZONE=America/Belem
```

## Atalhos com Make

```bash
make build
make test
make benchmark
make start
make status
make stop
```

## Servidor (`servidor-service`)

Consulta o cadastro completo da view `dbo.devops_servidor` (SQL Server da folha, `MSSQL_DSN`).

| Método | Rota | Descrição |
|---|---|---|
| GET | `/api/servidores/detalhe?cpf={cpf}` | detalhe pelo CPF (com ou sem pontuação) |
| GET | `/api/servidores/detalhe?nome={nome}` | detalhe por trecho do nome, sem acento |
| GET | `/api/servidores/detalhe?matricula={matricula}` | detalhe pela matrícula |
| GET | `/api/servidores/detalhe?q={termo}` | o termo é CPF (11 dígitos), matrícula (número) ou nome |

`cpf`, `nome` e `matricula` podem ser combinados. A resposta traz `total` e `servidores` com todas as colunas da view. Sem resultado, a API responde 404.

## Endpoints

```text
GET http://localhost:8081/api/usuarios/status
GET http://localhost:8082/api/ferias/status
GET http://localhost:8083/api/servidores/status
GET http://localhost:8084/api/contracheque/status
```

Todos os serviços também expõem:

```text
GET /healthz
GET /actuator/health
```

## Docker Compose

Iniciar todos:

```bash
docker compose up -d --build
```

Parar apenas um:

```bash
docker compose stop servidor-service
```

Ver o estado:

```bash
docker compose ps
```

Remover os containers:

```bash
docker compose down
```

## Testes de desempenho

O comando abaixo executa benchmarks internos dos handlers:

```bash
./servicos.sh benchmark
```

Para testar a API pela rede, instale uma ferramenta como `oha` e execute:

```bash
oha -z 30s -c 100 http://localhost:8083/api/servidores/status
```

Compare pelo menos:

- requisições por segundo;
- latências p50, p95 e p99;
- percentual de erros;
- consumo de CPU e memória;
- comportamento com conexões simultâneas.

Faça testes de carga somente em ambiente autorizado e isolado da produção do TCE.

## Decisões voltadas a desempenho

- Fiber v3 e `fasthttp`;
- handlers mínimos, sem middleware global de logging;
- binários sem CGO;
- flags `-trimpath` e `-ldflags="-s -w"`;
- imagem Docker final `scratch`;
- limites de corpo e timeouts configurados;
- encerramento gracioso com prazo de 10 segundos.

Fiber prioriza desempenho e não usa `net/http` como mecanismo principal. Antes de adotar em produção, valide a compatibilidade das bibliotecas, middlewares, proxy reverso, observabilidade e requisitos institucionais necessários no TCE.
