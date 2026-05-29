# GoPhish — Vercel Edition

Adaptação completa do GoPhish para rodar na Vercel com PostgreSQL.

## Arquitetura

| Componente | Solução |
|---|---|
| Backend | Go Serverless Functions (`/api/`) |
| Banco de dados | Vercel Postgres (PostgreSQL) |
| Autenticação | JWT (cookie httpOnly + localStorage) |
| Envio de emails | Vercel Cron Job (a cada minuto) + SMTP próprio |
| Frontend | HTML estático em `/public/` |

## Deploy

### 1. Pré-requisitos
- Conta na [Vercel](https://vercel.com)
- Repositório Git (GitHub, GitLab ou Bitbucket)

### 2. Criar banco de dados
No painel da Vercel:
1. **Storage → Create Database → Postgres**
2. Dê um nome e crie
3. Vá em **Settings → Environment Variables** e copie `DATABASE_URL`

### 3. Configurar variáveis de ambiente
No painel da Vercel, em **Settings → Environment Variables**, adicione:

```
DATABASE_URL   = (preenchido automaticamente pelo Vercel Postgres)
JWT_SECRET     = (string aleatória longa, ex: openssl rand -hex 32)
CRON_SECRET    = (string aleatória para proteger o endpoint de cron)
```

### 4. Deploy
```bash
# Instalar Vercel CLI
npm i -g vercel

# Na pasta do projeto
cd gophish-vercel
vercel --prod
```

Ou conecte o repositório no painel da Vercel para deploy automático.

### 5. Criar usuário admin (primeiro acesso)
Após o deploy, acesse uma vez:
```bash
curl -X POST https://SEU-DOMINIO.vercel.app/api/setup \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"SuaSenhaForte"}'
```

Esse endpoint só funciona enquanto não houver nenhum usuário no banco.

### 6. Acessar
Abra `https://SEU-DOMINIO.vercel.app` e faça login.

## Estrutura do projeto

```
gophish-vercel/
├── api/                    # Serverless Functions (Go)
│   ├── auth/login.go       # POST /api/auth/login
│   ├── campaigns/index.go  # GET/POST /api/campaigns/ + sub-rotas
│   ├── groups/index.go     # GET/POST /api/groups/
│   ├── templates/index.go  # GET/POST /api/templates/
│   ├── pages/index.go      # GET/POST /api/pages/
│   ├── smtp/index.go       # GET/POST /api/smtp/
│   ├── users/index.go      # GET/PUT /api/users/
│   ├── track/index.go      # Tracker de phishing (?rid=XXX)
│   ├── cron/send-emails.go # Chamado pelo Vercel Cron (1x/min)
│   ├── import/index.go     # POST /api/import/site
│   └── setup/index.go      # POST /api/setup (primeiro uso)
├── lib/
│   ├── models/models.go    # Structs GORM + lógica de negócio
│   ├── middleware/auth.go  # JWT auth
│   └── mailer/mailer.go    # Envio de email via SMTP
├── public/                 # Frontend estático
│   ├── login.html
│   ├── campaigns.html
│   ├── campaign_results.html
│   ├── js/
│   │   ├── api.js          # Wrapper de API com JWT
│   │   └── dist/app/       # JS compilado
│   └── css/
├── vercel.json             # Configuração de rotas + cron
└── go.mod
```

## Funcionalidades

- ✅ Campanhas de phishing (email)
- ✅ Links WhatsApp rastreáveis
- ✅ Dashboard com gráficos separados (Email / WhatsApp)
- ✅ Rastreamento: cliques, abertura de email, submissão de dados
- ✅ Grupos de alvos
- ✅ Templates de email
- ✅ Landing pages
- ✅ Perfis SMTP
- ✅ Export CSV (email + WhatsApp)
- ✅ Envio agendado de emails via Cron

## Diferenças em relação ao GoPhish original

| Feature | GoPhish Original | Vercel Edition |
|---|---|---|
| Banco | SQLite | PostgreSQL |
| Auth | Sessão em memória | JWT |
| Email sending | Worker contínuo | Vercel Cron (1x/min) |
| Deploy | Servidor VPS | Serverless |
| Custo | VPS (~$6/mês) | Vercel Hobby (grátis) |
