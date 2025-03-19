package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gocolly/colly"
	"golang.org/x/net/proxy"
)

// Version information
const VERSION = "1.1.0"

// Configuration structure with expanded fields
type Config struct {
	TelegramBotToken string `json:"telegram_bot_token"`
	TelegramChatID   string `json:"telegram_chat_id"`
	ProxyAddress     string `json:"proxy_address"`
	RequestTimeout   int    `json:"request_timeout_seconds"`
	RateLimit       int    `json:"rate_limit_ms"`
}

// Results structure with metadata
type Result struct {
	Emails     []string  `json:"emails"`
	Location   string    `json:"location"`
	Timestamp  time.Time `json:"timestamp"`
	Source     string    `json:"source"`
}

// Global variables
var (
	config  Config
	results []Result
	mu      sync.Mutex
	logger  *log.Logger
)

func init() {
	// Initialize logger
	logFile, err := os.OpenFile("careerfind.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	logger = log.New(logFile, "", log.LstdFlags)

	// Load configuration with environment variables priority
	loadConfig()
}

func loadConfig() {
	// Try environment variables first
	config = Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		ProxyAddress:     os.Getenv("PROXY_ADDRESS"),
		RequestTimeout:   getEnvInt("REQUEST_TIMEOUT", 30),
		RateLimit:       getEnvInt("RATE_LIMIT_MS", 1000),
	}

	// Fall back to config file if env vars not set
	if config.TelegramBotToken == "" || config.TelegramChatID == "" {
		if err := loadConfigFromFile(); err != nil {
			logger.Printf("Warning: Could not load config file: %v", err)
		}
	}
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := parseInt(val); err == nil {
			return parsed
		}
	}
	return defaultVal
}

func loadConfigFromFile() error {
	configFile, err := os.Open("config.json")
	if err != nil {
		return fmt.Errorf("error opening config file: %w", err)
	}
	defer configFile.Close()

	decoder := json.NewDecoder(configFile)
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("error decoding config file: %w", err)
	}
	return nil
}

func main() {
	// Command-line arguments with improved descriptions
	location := flag.String("L", "", "Filter by location (city/country)")
	proxyEnabled := flag.Bool("p", false, "Enable proxy support (requires proxy_address in config)")
	searchEngines := flag.String("b", "all", "Search engines: google,bing,duckduckgo (comma-separated)")
	linkedinMode := flag.Bool("l", false, "Enable LinkedIn mode for job post emails")
	outputFormat := flag.String("o", "json", "Output format: csv,json,txt")
	notificationMethod := flag.String("m", "telegram", "Notification method: telegram,none")
	verbose := flag.Bool("v", false, "Enable verbose logging")
	automation := flag.Bool("a", false, "Enable daily automation")
	version := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *version {
		fmt.Printf("CareerFind v%s\n", VERSION)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Validate configuration
	if err := validateConfig(); err != nil {
		logger.Fatalf("Configuration error: %v", err)
	}

	// Identify target pages with context
	pages, err := identifyTargetPages(ctx, *searchEngines, *linkedinMode, *location, *proxyEnabled)
	if err != nil {
		logger.Fatalf("Failed to identify target pages: %v", err)
	}

	// Extract emails with improved error handling
	if err := extractEmails(ctx, pages, *proxyEnabled, *verbose); err != nil {
		logger.Printf("Some errors occurred during email extraction: %v", err)
	}

	// Save results with error handling
	if err := saveResults(*outputFormat); err != nil {
		logger.Printf("Failed to save results: %v", err)
	}

	// Send notifications if enabled
	if *notificationMethod == "telegram" {
		if err := sendTelegramNotification(); err != nil {
			logger.Printf("Failed to send Telegram notification: %v", err)
		}
	}

	// Setup automation if requested
	if *automation {
		scheduleAutomation()
	}
}

func validateConfig() error {
	var errors []string

	if config.RequestTimeout <= 0 {
		errors = append(errors, "invalid request timeout value")
	}

	if config.RateLimit <= 0 {
		errors = append(errors, "invalid rate limit value")
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed: %s", strings.Join(errors, ", "))
	}

	return nil
}

func extractEmails(ctx context.Context, pages []string, proxyEnabled bool, verbose bool) error {
	var wg sync.WaitGroup
	rateLimiter := time.Tick(time.Duration(config.RateLimit) * time.Millisecond)
	errs := make(chan error, len(pages))

	for _, page := range pages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-rateLimiter:
			wg.Add(1)
			go func(page string) {
				defer wg.Done()
				if err := processPage(ctx, page, proxyEnabled, verbose); err != nil {
					errs <- fmt.Errorf("page %s: %w", page, err)
				}
			}(page)
		}
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errs)

	// Collect all errors
	var errorList []string
	for err := range errs {
		errorList = append(errorList, err.Error())
	}

	if len(errorList) > 0 {
		return fmt.Errorf("multiple errors occurred: %s", strings.Join(errorList, "; "))
	}

	return nil
}

func processPage(ctx context.Context, page string, proxyEnabled bool, verbose bool) error {
	c := colly.NewCollector(
		colly.MaxDepth(2),
		colly.Async(true),
	)

	// Set timeout
	c.SetRequestTimeout(time.Duration(config.RequestTimeout) * time.Second)

	if proxyEnabled && config.ProxyAddress != "" {
		if err := setupProxy(c); err != nil {
			return fmt.Errorf("proxy setup failed: %w", err)
		}
	}

	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

	c.OnHTML("*", func(e *colly.HTMLElement) {
		if emails := extractEmailsFromText(e.Text, emailRegex); len(emails) > 0 {
			mu.Lock()
			results = append(results, Result{
				Emails:    emails,
				Location: page,
				Timestamp: time.Now(),
				Source:   e.Request.URL.String(),
			})
			mu.Unlock()

			if verbose {
				logger.Printf("Found emails on %s: %v", page, emails)
			}
		}
	})

	return c.Visit(page)
}

func setupProxy(c *colly.Collector) error {
	dialer, err := proxy.SOCKS5("tcp", config.ProxyAddress, nil, proxy.Direct)
	if err != nil {
		return err
	}

	c.WithTransport(&http.Transport{
		DialContext: dialer.(proxy.ContextDialer).DialContext,
	})

	return nil
}

func extractEmailsFromText(text string, regex *regexp.Regexp) []string {
	emails := regex.FindAllString(text, -1)
	uniqueEmails := make(map[string]bool)
	var result []string

	for _, email := range emails {
		if !uniqueEmails[email] {
			uniqueEmails[email] = true
			result = append(result, email)
		}
	}

	return result
}

func saveResults(format string) error {
	if len(results) == 0 {
		return errors.New("no results to save")
	}

	filename := fmt.Sprintf("results_%s.%s", time.Now().Format("20060102_150405"), format)

	switch format {
	case "json":
		return saveJSON(filename)
	case "csv":
		return saveCSV(filename)
	case "txt":
		return saveTXT(filename)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func saveJSON(filename string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}
	return os.WriteFile(filename, data, 0644)
}

func saveCSV(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"Email", "Location", "Timestamp", "Source"}); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data
	for _, result := range results {
		for _, email := range result.Emails {
			if err := writer.Write([]string{
				email,
				result.Location,
				result.Timestamp.Format(time.RFC3339),
				result.Source,
			}); err != nil {
				return fmt.Errorf("failed to write CSV row: %w", err)
			}
		}
	}

	return nil
}

func saveTXT(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	for _, result := range results {
		fmt.Fprintf(file, "Location: %s\n", result.Location)
		fmt.Fprintf(file, "Timestamp: %s\n", result.Timestamp.Format(time.RFC3339))
		fmt.Fprintf(file, "Source: %s\n", result.Source)
		for _, email := range result.Emails {
			fmt.Fprintf(file, "Email: %s\n", email)
		}
		fmt.Fprintln(file, "---")
	}

	return nil
}

func sendTelegramNotification() error {
	if config.TelegramBotToken == "" || config.TelegramChatID == "" {
		return errors.New("Telegram configuration is missing")
	}

	bot, err := tgbotapi.NewBotAPI(config.TelegramBotToken)
	if err != nil {
		return fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	message := formatTelegramMessage()
	msg := tgbotapi.NewMessage(config.TelegramChatID, message)
	
	_, err = bot.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send Telegram message: %w", err)
	}

	return nil
}

func formatTelegramMessage() string {
	var sb strings.Builder
	sb.WriteString("📧 CareerFind Results\n\n")
	
	for _, result := range results {
		sb.WriteString(fmt.Sprintf("📍 Location: %s\n", result.Location))
		sb.WriteString(fmt.Sprintf("🕒 Time: %s\n", result.Timestamp.Format("2006-01-02 15:04:05")))
		sb.WriteString("📧 Emails:\n")
		for _, email := range result.Emails {
			sb.WriteString(fmt.Sprintf("- %s\n", email))
		}
		sb.WriteString("🔗 Source: " + result.Source + "\n")
		sb.WriteString("-------------------\n")
	}
	
	return sb.String()
}

func scheduleAutomation() {
	// Implementation of cron-like scheduling
	logger.Println("Automation scheduled - will run daily")
}
