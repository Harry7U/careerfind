package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gocolly/colly"
	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron/v3"
	"golang.org/x/net/proxy"
)

// Version information
const VERSION = "2.0.0"

// Configuration structure with expanded fields
type Config struct {
	TelegramBotToken string `json:"telegram_bot_token"`
	TelegramChatID   string `json:"telegram_chat_id"`
	ProxyAddress     string `json:"proxy_address"`
	RequestTimeout   int    `json:"request_timeout_seconds"`
	RateLimit        int    `json:"rate_limit_ms"`
	UserAgent        string `json:"user_agent"`
}

// Results structure with metadata
type Result struct {
	Emails    []string  `json:"emails"`
	Location  string    `json:"location"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
}

// Global variables
var (
	config  Config
	results []Result
	mu      sync.Mutex
	logger  *log.Logger
	db      *sql.DB
)

func init() {
	// Initialize logger
	logFile, err := os.OpenFile("careerfind.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	logger = log.New(logFile, "", log.LstdFlags)

	// Load configuration with environment variables priority
	loadConfig()

	// Initialize database
	initDB()
}

func loadConfig() {
	// Try environment variables first
	config = Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		ProxyAddress:     os.Getenv("PROXY_ADDRESS"),
		RequestTimeout:   getEnvInt("REQUEST_TIMEOUT", 30),
		RateLimit:        getEnvInt("RATE_LIMIT_MS", 1000),
		UserAgent:        os.Getenv("USER_AGENT"),
	}

	// Fall back to config file if env vars not set
	if config.TelegramBotToken == "" || config.TelegramChatID == "" {
		if err := loadConfigFromFile(); err != nil {
			logger.Printf("Warning: Could not load config file: %v", err)
		}
	}

	// Set default user agent if not specified
	if config.UserAgent == "" {
		config.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	}
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
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

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./careerfind.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	createTableSQL := `CREATE TABLE IF NOT EXISTS results (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,		
		"emails" TEXT,
		"location" TEXT,
		"timestamp" DATETIME,
		"source" TEXT
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
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

	// Set logger output based on verbose flag
	if *verbose {
		log.Printf("Starting CareerFind with location: %s", *location)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Validate configuration
	if err := validateConfig(); err != nil {
		log.Printf("Configuration error: %v", err)
		os.Exit(1)
	}

	// Update the identifyTargetPages function to include more search variations
func identifyTargetPages(ctx context.Context, searchEngines string, linkedinMode bool, location string, proxyEnabled bool) ([]string, error) {
    if location == "" {
        return nil, errors.New("location cannot be empty")
    }

    var pages []string
    engines := strings.Split(strings.ToLower(searchEngines), ",")

    // Handle "all" option
    if searchEngines == "all" {
        engines = []string{"google", "bing", "duckduckgo"}
    }

    // Load search parameters from config
    searchQueries := []string{
        fmt.Sprintf("email careers %s", location),
        fmt.Sprintf("contact us jobs %s", location),
        fmt.Sprintf("careers@company %s", location),
        fmt.Sprintf("hr@company %s", location),
        fmt.Sprintf("recruitment %s email", location),
        fmt.Sprintf("apply jobs %s contact", location),
    }

    for _, engine := range engines {
        for _, query := range searchQueries {
            encoded := url.QueryEscape(query)
            var searchURL string

            switch engine {
            case "google":
                searchURL = fmt.Sprintf("https://www.google.com/search?q=%s&num=100", encoded)
            case "bing":
                searchURL = fmt.Sprintf("https://www.bing.com/search?q=%s&count=100", encoded)
            case "duckduckgo":
                searchURL = fmt.Sprintf("https://duckduckgo.com/?q=%s", encoded)
            default:
                continue
            }

            if searchURL != "" {
                pages = append(pages, searchURL)
            }
        }
    }

    if linkedinMode {
        queries := []string{
            fmt.Sprintf("jobs %s", location),
            fmt.Sprintf("careers %s", location),
            fmt.Sprintf("hiring %s", location),
        }
        for _, q := range queries {
            linkedinURL := fmt.Sprintf("https://www.linkedin.com/jobs/search?keywords=%s", url.QueryEscape(q))
            pages = append(pages, linkedinURL)
        }
    }

    if len(pages) == 0 {
        return nil, errors.New("no valid search engines specified")
    }

    logger.Printf("Generated %d search URLs", len(pages))
    return pages, nil
}

// Update the processPage function with better email extraction
func processPage(ctx context.Context, page string, proxyEnabled bool, verbose bool) error {
    c := colly.NewCollector(
        colly.MaxDepth(config.SearchDepth),
        colly.Async(true),
        colly.UserAgent(config.UserAgent),
        colly.AllowURLRevisit(),
    )

    // Set timeout
    c.SetRequestTimeout(time.Duration(config.RequestTimeout) * time.Second)

    if proxyEnabled && config.ProxyAddress != "" {
        if err := setupProxy(c); err != nil {
            return fmt.Errorf("proxy setup failed: %w", err)
        }
        if verbose {
            logger.Printf("Using proxy: %s", config.ProxyAddress)
        }
    }

    // Add retry on error
    c.OnError(func(r *colly.Response, err error) {
        if verbose {
            logger.Printf("Error on %s: %v", r.Request.URL, err)
        }
        retries := 0
        for retries < config.MaxRetries {
            if verbose {
                logger.Printf("Retrying %s (attempt %d/%d)", r.Request.URL, retries+1, config.MaxRetries)
            }
            time.Sleep(time.Duration(1<<uint(retries)) * time.Second) // Exponential backoff
            err := c.Visit(r.Request.URL.String())
            if err == nil {
                break
            }
            retries++
        }
    })

    emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
    
    c.OnHTML("*", func(e *colly.HTMLElement) {
        // Extract from text content
        if emails := extractEmailsFromText(e.Text, emailRegex); len(emails) > 0 {
            storeResults(emails, page, e.Request.URL.String(), verbose)
        }

        // Extract from links
        e.ForEach("a[href^='mailto:']", func(_ int, el *colly.HTMLElement) {
            if href := el.Attr("href"); strings.HasPrefix(href, "mailto:") {
                email := strings.TrimPrefix(href, "mailto:")
                email = strings.Split(email, "?")[0] // Remove any parameters
                if isValidEmail(email) {
                    storeResults([]string{email}, page, e.Request.URL.String(), verbose)
                }
            }
        })
    })

    return c.Visit(page)
}

// Add helper function to store results
func storeResults(emails []string, page string, source string, verbose bool) {
    mu.Lock()
    defer mu.Unlock()

    // Filter duplicate emails
    uniqueEmails := make(map[string]bool)
    var filteredEmails []string
    
    for _, email := range emails {
        if !uniqueEmails[email] {
            uniqueEmails[email] = true
            filteredEmails = append(filteredEmails, email)
        }
    }

    if len(filteredEmails) > 0 {
        results = append(results, Result{
            Emails:    filteredEmails,
            Location:  page,
            Timestamp: time.Now().UTC(),
            Source:    source,
        })
        
        if verbose {
            logger.Printf("Found %d unique email(s) on %s", len(filteredEmails), source)
            for _, email := range filteredEmails {
                logger.Printf("- %s", email)
            }
        }
    }
}

	// Extract emails with improved error handling
	if *verbose {
		log.Printf("Starting email extraction from pages...")
	}
	if err := extractEmails(ctx, pages, *proxyEnabled, *verbose); err != nil {
		log.Printf("Some errors occurred during email extraction: %v", err)
	}

	// Save results with error handling
	if err := saveResults(*outputFormat); err != nil {
		log.Printf("Failed to save results: %v", err)
		os.Exit(1)
	}

	// Save results to database
	if err := saveResultsToDB(); err != nil {
		log.Printf("Failed to save results to database: %v", err)
	}

	// Send notifications if enabled
	if *notificationMethod == "telegram" {
		if err := sendTelegramNotification(); err != nil {
			log.Printf("Failed to send Telegram notification: %v", err)
		}
	}

	// Setup automation if requested
	if *automation {
		scheduleAutomation()
	}

	if *verbose {
		log.Printf("CareerFind execution completed")
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

	if config.UserAgent == "" {
		errors = append(errors, "user agent cannot be empty")
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed: %s", strings.Join(errors, ", "))
	}

	return nil
}

func identifyTargetPages(ctx context.Context, searchEngines string, linkedinMode bool, location string, proxyEnabled bool) ([]string, error) {
	if location == "" {
		return nil, errors.New("location cannot be empty")
	}

	var pages []string
	engines := strings.Split(strings.ToLower(searchEngines), ",")

	// Handle "all" option
	if searchEngines == "all" {
		engines = []string{"google", "bing", "duckduckgo"}
	}

	for _, engine := range engines {
		searchQuery := url.QueryEscape("email careers " + location)
		var searchURL string

		switch engine {
		case "google":
			searchURL = "https://www.google.com/search?q=" + searchQuery
		case "bing":
			searchURL = "https://www.bing.com/search?q=" + searchQuery
		case "duckduckgo":
			searchURL = "https://duckduckgo.com/?q=" + searchQuery
		default:
			continue
		}

		if searchURL != "" {
			pages = append(pages, searchURL)
		}
	}

	if linkedinMode {
		linkedinURL := "https://www.linkedin.com/jobs/search?keywords=" + url.QueryEscape(location)
		pages = append(pages, linkedinURL)
	}

	if len(pages) == 0 {
		return nil, errors.New("no valid search engines specified")
	}

	return pages, nil
}

func extractEmails(ctx context.Context, pages []string, proxyEnabled bool, verbose bool) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(pages))

	// Create a ticker for rate limiting instead of time.Tick
	ticker := time.NewTicker(time.Duration(config.RateLimit) * time.Millisecond)
	defer ticker.Stop()

	for _, page := range pages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
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
		colly.UserAgent(config.UserAgent),
	)

	// Set timeout
	c.SetRequestTimeout(time.Duration(config.RequestTimeout) * time.Second)

	if proxyEnabled && config.ProxyAddress != "" {
		if err := setupProxy(c); err != nil {
			return fmt.Errorf("proxy setup failed: %w", err)
		}
	}

	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

	// Add error handling for responses
	c.OnError(func(r *colly.Response, err error) {
		if verbose {
			logger.Printf("Error scraping %s: %v", r.Request.URL, err)
		}
	})

	// Add response handling to check status
	c.OnResponse(func(r *colly.Response) {
		if verbose {
			logger.Printf("Visited %s (status: %d)", r.Request.URL, r.StatusCode)
		}
	})

	c.OnHTML("*", func(e *colly.HTMLElement) {
		if emails := extractEmailsFromText(e.Text, emailRegex); len(emails) > 0 {
			mu.Lock()
			results = append(results, Result{
				Emails:    emails,
				Location:  page,
				Timestamp: time.Now(),
				Source:    e.Request.URL.String(),
			})
			mu.Unlock()

			if verbose {
				logger.Printf("Found emails on %s: %v", page, emails)
			}
		}
	})

	// Add headers to look more like a browser
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.5")
		if verbose {
			logger.Printf("Visiting %s", r.URL)
		}
	})

	err := c.Visit(page)
	if err != nil {
		return fmt.Errorf("failed to visit page %s: %w", page, err)
	}

	// Wait for all requests to finish
	c.Wait()
	return nil
}

func setupProxy(c *colly.Collector) error {
	dialer, err := proxy.SOCKS5("tcp", config.ProxyAddress, nil, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
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
		if isValidEmail(email) && !uniqueEmails[email] {
			uniqueEmails[email] = true
			result = append(result, email)
		}
	}

	return result
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
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

func saveResultsToDB() error {
	if len(results) == 0 {
		return errors.New("no results to save")
	}

	for _, result := range results {
		emails := strings.Join(result.Emails, ",")
		_, err := db.Exec("INSERT INTO results (emails, location, timestamp, source) VALUES (?, ?, ?, ?)",
			emails, result.Location, result.Timestamp, result.Source)
		if err != nil {
			return fmt.Errorf("failed to insert result into database: %w", err)
		}
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

	// Convert chat ID from string to int64
	chatID, err := strconv.ParseInt(config.TelegramChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid Telegram chat ID: %w", err)
	}

	msg := tgbotapi.NewMessage(chatID, message)
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
	c := cron.New()
	_, err := c.AddFunc("@daily", func() {
		ctx := context.Background()
		if err := runAutomatedSearch(ctx); err != nil {
			logger.Printf("Automated search failed: %v", err)
		}
	})

	if err != nil {
		logger.Printf("Failed to schedule automation: %v", err)
		return
	}

	c.Start()
	logger.Println("Automation scheduled - will run daily at midnight")
}

func runAutomatedSearch(ctx context.Context) error {
	// Default automated search parameters
	location := "worldwide"
	searchEngines := "google,bing"
	proxyEnabled := true
	verbose := true

	pages, err := identifyTargetPages(ctx, searchEngines, false, location, proxyEnabled)
	if err != nil {
		return fmt.Errorf("failed to identify target pages: %w", err)
	}

	if err := extractEmails(ctx, pages, proxyEnabled, verbose); err != nil {
		return fmt.Errorf("failed to extract emails: %w", err)
	}

	if err := saveResults("json"); err != nil {
		return fmt.Errorf("failed to save results: %w", err)
	}

	if err := sendTelegramNotification(); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}
