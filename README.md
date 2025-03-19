# 🎯 CareerFind v1.1.0

## 🌟 Overview
`careerfind` is an advanced, all-in-one Go tool designed to extract job-related email addresses from career pages and job postings. It features intelligent rate limiting, robust error handling, and multiple output formats.

## ✨ Features
- 📧 Extract job-related email addresses from career pages and job postings
- 🔍 Use Google, Bing, and DuckDuckGo dorks to locate career-related contact details
- 🛡️ Support proxy usage with configurable settings
- 🤖 Send results via a pre-configured Telegram bot
- 📊 Multiple output formats (JSON, CSV, TXT) with timestamps
- 📝 Comprehensive logging system
- ⏱️ Smart rate limiting to prevent blocking
- 🔄 Automated daily execution support
- 🚦 Request timeout management

## 📋 Requirements
- Go 1.16+
- Dependencies:
  ```sh
  go get -u github.com/go-telegram-bot-api/telegram-bot-api
  go get -u github.com/gocolly/colly
  go get -u golang.org/x/net/proxy
  ```

## 🛠️ Installation
1. Install Go from [golang.org](https://golang.org/dl/)
2. Clone the repository:
   ```sh
   git clone https://github.com/Harry7U/careerfind.git
   cd careerfind
   ```
3. Configure the tool (choose one method):

   A. Environment Variables (Recommended):
   ```sh
   export TELEGRAM_BOT_TOKEN="your_bot_token"
   export TELEGRAM_CHAT_ID="your_chat_id"
   export PROXY_ADDRESS="localhost:1080"
   export REQUEST_TIMEOUT=30
   export RATE_LIMIT_MS=1000
   ```

   B. Config File (config.json):
   ```json
   {
     "telegram_bot_token": "YOUR_TELEGRAM_BOT_TOKEN",
     "telegram_chat_id": "YOUR_TELEGRAM_CHAT_ID",
     "proxy_address": "localhost:1080",
     "request_timeout_seconds": 30,
     "rate_limit_ms": 1000
   }
   ```

4. Build the application:
   ```sh
   go build -o careerfind
   ```

## 🚀 Usage
Run the tool with desired options:
```sh
./careerfind -L "San Francisco" -p -b "all" -l -o json -a -v
```

### Command-line Arguments & Options
| Option | Description | Default |
|--------|-------------|---------|
| `-L` | Filter by location (city/country) | Required |
| `-p` | Enable proxy support | false |
| `-b` | Search engines (google,bing,duckduckgo,all) | "all" |
| `-l` | Enable LinkedIn mode | false |
| `-o` | Output format (json,csv,txt) | "json" |
| `-m` | Notification method (telegram,none) | "telegram" |
| `-a` | Enable automation (daily cron job) | false |
| `-v` | Verbose mode | false |
| `-version` | Show version information | false |

## 💡 Example Commands

1. Basic search:
```sh
./careerfind -L "New York"
```

2. Full featured search:
```sh
./careerfind -L "San Francisco" -p -b "all" -l -o json -m telegram -v -a
```

3. Quick test without notifications:
```sh
./careerfind -L "Test Location" -o json -m none -v
```

4. Automated daily run:
```sh
./careerfind -L "Multiple Cities" -a -o json -v
```

### Output Files
- Results: `results_YYYYMMDD_HHMMSS.{json|csv|txt}`
- Logs: `careerfind.log`

### Expected Output Structure
```json
{
  "results": [
    {
      "emails": ["example@company.com"],
      "location": "https://example.com/careers",
      "timestamp": "2025-03-19T17:48:28Z",
      "source": "https://example.com/careers/job-posting"
    }
  ]
}
```

## 🔍 Troubleshooting
1. Check version:
```sh
./careerfind -version
```

2. Enable verbose logging:
```sh
./careerfind -L "Test" -v 2>&1 | tee debug.log
```

3. Common issues:
- Rate limiting: Adjust `RATE_LIMIT_MS` environment variable
- Timeout errors: Increase `REQUEST_TIMEOUT` value
- Proxy errors: Verify proxy server is running and accessible

## 🔐 Security Notes
- Use environment variables instead of config.json for sensitive data
- Always use proxy support (-p) when scraping at scale
- Review the logs for any blocked requests or errors
- Consider using different proxy addresses for different search engines

## 📜 License
[MIT](LICENSE)

## 🤝 Contributing
Contributions are welcome! Please feel free to submit a Pull Request.

Last updated: 2025-03-19
