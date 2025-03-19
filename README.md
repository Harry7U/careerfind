# 🎯 CareerFind

## 🌟 Overview
`careerfind` is an advanced, all-in-one Go tool designed to extract job-related email addresses from career pages and job postings. It uses search engine dorks to locate career-related contact details, supports proxy usage and multi-threading, and sends results via a pre-configured Telegram bot. The tool also provides detailed verbose output and structured data export options.

## ✨ Features
- 📧 Extract job-related email addresses from career pages and job postings
- 🔍 Use Google, Bing, and DuckDuckGo dorks to locate career-related contact details
- 🛡️ Support proxy usage and multi-threading
- 🤖 Send results via a pre-configured Telegram bot
- 📊 Provide detailed verbose output and structured data export options

## 📋 Requirements
- Go 1.16+
- Dependencies:
  - `http`
  - `net/http`
  - `regexp`
  - `encoding/json`
  - `os`
  - `io`
  - `sync`

## 🛠️ Installation
1. Install Go from [golang.org](https://golang.org/dl/)
2. Install the tool:
   ```sh
   go install -v github.com/Harry7U/careerfind@latest
   ```
3. Create a `config.json` file with your Telegram bot token and chat ID:
   ```json
   {
     "telegram_bot_token": "YOUR_TELEGRAM_BOT_TOKEN",
     "telegram_chat_id": "YOUR_TELEGRAM_CHAT_ID"
   }
   ```

## 🚀 Usage
Run the tool with the desired options:
```sh
careerfind -L "San Francisco" -p -b "all" -l -o results.json -a -v
```

### Command-line Arguments & Options
| Option | Description |
|--------|-------------|
| `-L` | Filter by location (city/country) |
| `-p` | Enable proxy support for anonymous requests |
| `-b` | Select search engines (Google, Bing, DuckDuckGo) |
| `-l` | Enable LinkedIn mode to extract job post emails |
| `-o` | Specify output format (CSV, JSON, TXT) |
| `-m` | Notification method (Telegram) |
| `-T` | Pre-configured Telegram bot token (auto-loaded) |
| `-C` | Pre-configured Telegram chat ID (auto-loaded) |
| `-a` | Enable automation (daily cron job) |
| `-v` | Verbose mode (display detailed scan results) |

## 💡 Example Command Execution
```sh
careerfind -L "San Francisco" -p -b "all" -l -o results.json -a -v
```

### Expected Outcome:
- Scrapes job-related emails in San Francisco
- Uses proxy for anonymous browsing
- Searches across Google, Bing, and DuckDuckGo
- Extracts LinkedIn job post emails
- Saves output as JSON (`results.json`)
- Runs in verbose mode (`-v`) for detailed progress
- Automates daily scans (`-a` enabled)
- Sends results via Telegram

## 📜 License
[MIT](LICENSE)
