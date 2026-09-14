# Production-Ready Serverless Instagram Downloader Telegram Bot (Go)

A **production-ready, ultra-fast, fully serverless** Instagram Downloader Telegram Bot built with Go (Golang), designed for zero-config deployment on **Vercel Serverless Functions**.

---

## ⚡ Highlights & Features

- **100% Serverless & Ephemeral**: Zero permanent storage, no daemons, no background loops, no Docker, no VPS required.
- **Telegram Bot API Only**: Clean webhook integration. **NO MTProto**, no user session, no account ban risks.
- **Multi-Resolver Fallback Architecture**:
  - `igexport` (https://igexport.com/api/ig-photo/?url=...)
  - `fastdl` (https://fastdl.app/msec + https://api-wh.fastdl.app/api/convert)
  - `sssinstagram` (https://sssinstagram.com/msec + https://api-wh.sssinstagram.com/api/convert)
  - `snapsave` (https://snapsave.app/action.php?lang=en)
  - `saveclip` (https://v3.saveclip.app/api/ajaxSearch)
  - `savefromins` (https://api.savefromins.com/api/contentsite_api/media/parse)
  - **Direct Source Fallback**: If all providers fail, delivers direct Instagram source buttons so users are never left stranded.
- **12 Supported Languages (Dynamic UI & Buttons)**:
  - 🇺🇸 English, 🇮🇳 हिन्दी (Hindi), 🇮🇳 தமிழ் (Tamil), 🇮🇳 తెలుగు (Telugu)
  - 🇸🇦 العربية (Arabic), 🇵🇰 اردو (Urdu), 🇧🇩 বাংলা (Bangla), 🇨🇳 中文 (Chinese)
  - 🇷🇺 Русский (Russian), 🇰🇷 한국어 (Korean), 🇯🇵 日本語 (Japanese), 🇹🇷 Türkçe (Turkish)
- **Inline Keyboard & Dynamic Callbacks**:
  - `/start` greeting with interactive inline buttons (`Help`, `About`, `Language`).
  - Interactive multi-language picker with immediate session switching.
- **Free Temporary File Storage Adapter**:
  - Integrated with `tmpfiles.org` (100% free, no API key required, auto-expiring).
  - Pluggable interface (`internal/storage/storage.go`).
- **Auto-Detection Webhook**:
  - Deploy to Vercel, visit `https://your-domain.vercel.app/api/telegram?setup=true` in your browser, and the bot automatically registers its webhook with Telegram!

---

## 📁 Project Structure

```text
slmedia/
├── api/
│   └── telegram.go               # Vercel Serverless Function entrypoint
├── internal/
│   ├── config/
│   │   └── config.go             # Environment variable parser & defaults
│   ├── i18n/
│   │   └── i18n.go               # 12-language translation bundles & preferences
│   ├── media/
│   │   └── downloader.go         # Ephemeral streaming downloader with size bounds
│   ├── ratelimit/
│   │   └── ratelimit.go          # Sliding-window rate limiter
│   ├── resolver/
│   │   ├── detector.go           # Instagram URL validator & normalizer
│   │   ├── model.go              # MediaResult & MediaItem models
│   │   ├── resolver.go           # Fallback manager
│   │   ├── igexport.go           # igexport live adapter
│   │   ├── fastdl.go             # fastdl & sssinstagram adapter
│   │   ├── snapsave.go           # snapsave & saveclip adapter
│   │   └── savefromins.go        # savefromins adapter
│   ├── storage/
│   │   ├── storage.go            # Temporary storage interface
│   │   └── tmpfiles.go           # Free tmpfiles.org implementation
│   └── telegram/
│       ├── bot.go                # Webhook router, ForceSub, logs & delivery
│       ├── client.go             # Telegram Bot API client
│       └── models.go             # Telegram data models
├── .env.example
├── .gitignore
├── go.mod
├── vercel.json                   # Vercel serverless build config
└── README.md
```

---

## 🚀 Setup & Deployment Guide

### Step 1: Create Your Telegram Bot
1. Open Telegram and search for [@BotFather](https://t.me/BotFather).
2. Send `/newbot` and follow the prompts.
3. BotFather will provide an API token (e.g. `123456789:ABCdefGhIJKlmNoPQRsTUVwxyZ`).

### Step 2: Configure Environment Variables
Copy `.env.example` to `.env`:
```bash
cp .env.example .env
```

Edit `.env` and set:
```env
TELEGRAM_BOT_TOKEN=123456789:ABCdefGhIJKlmNoPQRsTUVwxyZ
TELEGRAM_WEBHOOK_SECRET=your_optional_secret_here
PROVIDER_ORDER=igexport,fastdl,sssinstagram,snapsave,saveclip,savefromins
```

*(Note: No API keys are required for the default providers or the free tmpfiles storage!)*

### Step 3: Test Locally Without Port Forwarding / Ngrok
You can test the bot **directly on your local machine** right now! Just put your bot token into `.env` and run:
```bash
go run ./cmd/local-dev
```
- It connects directly to Telegram using long-polling (`getUpdates`).
- Send any Instagram link or `/start` to your bot in Telegram.
- You can test reels, videos, posts, buttons, and languages live!

### Step 4: Run Tests & Verify Code
```bash
go test -v ./...
go vet ./...
go build ./...
```

---

## ☁️ Deploying to Vercel

### Option A: Using Vercel CLI
1. Install Vercel CLI (if not already installed):
   ```bash
   npm i -g vercel
   ```
2. Log in and deploy:
   ```bash
   vercel
   ```
3. Set your environment variables in Vercel:
   ```bash
   vercel env add TELEGRAM_BOT_TOKEN
   ```
4. Deploy to production:
   ```bash
   vercel --prod
   ```

### Option B: Using GitHub & Vercel Dashboard
1. Push this repository to GitHub / GitLab / Bitbucket.
2. Go to [vercel.com](https://vercel.com) and click **"New Project"**.
3. Import your repository.
4. Under **Environment Variables**, add:
   - `TELEGRAM_BOT_TOKEN`: Your bot token from BotFather.
   - `TELEGRAM_WEBHOOK_SECRET`: (Optional) A secret string for payload validation.
5. Click **Deploy**.

---

## 🔗 Configuring the Telegram Webhook

Once deployed on Vercel, you have two simple ways to link Telegram:

### Method 1: Instant Browser Auto-Setup (Recommended)
Simply open your browser and navigate to:
```
https://<your-project>.vercel.app/api/telegram?setup=true
```
The serverless function auto-detects its public domain and invokes `setWebhook` on the Telegram Bot API automatically!

### Method 2: Using the Included CLI Utility
```bash
go run ./cmd/webhook-setup -url https://<your-project>.vercel.app/api/telegram
```

To check webhook status:
```bash
go run ./cmd/webhook-setup -info
```

---

## 🧪 Testing the Bot in Telegram

1. Open your bot in Telegram and click **START** or send `/start`.
   - The bot displays the welcome message with **📖 Help**, **ℹ️ About**, and **🌐 Language** inline buttons.
2. Click **🌐 Language** to switch to any of the 12 supported languages (हिन्दी, தமிழ், తెలుగు, العربية, etc.).
3. Send any Instagram link:
   - **Reel**: `https://www.instagram.com/reel/DGndYOVpPVA/`
   - **Post / Video**: `https://www.instagram.com/p/...`
   - **Carousel**: The bot delivers items organized as an album.
4. Observe the clean progress status:
   - `🔎 Processing Instagram link...`
   - `⬇️ Resolving media...`
   - `⬆️ Uploading media to Telegram...`
   - `✅ Downloaded successfully!`

---

## 🛡️ Security & Safeguards

- **Rate Limiting**: In-memory sliding window protects against spam and runaway function invocations.
- **SSRF & Size Bounds**: All media downloads are guarded by `io.LimitReader` (max 50 MB) and context timeouts.
- **Ephemeral Storage**: Local temp files are deleted immediately after upload via `defer file.Close()`.
- **Secret Sanitization**: No bot tokens, credentials, or internal server errors are exposed to Telegram users.
