// Package cmd contains the command-line interface for the application.
package cmd

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/AdguardTeam/golibs/log"
	"github.com/adguardteam/go-webext/internal/chrome"
	"github.com/adguardteam/go-webext/internal/edge"
	"github.com/adguardteam/go-webext/internal/firefox"
	firefoxapi "github.com/adguardteam/go-webext/internal/firefox/api"
	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"
)

func getChromeStore() (*chrome.Store, error) {
	type config struct {
		ClientID     string `env:"CHROME_CLIENT_ID,notEmpty"`
		ClientSecret string `env:"CHROME_CLIENT_SECRET,notEmpty"`
		RefreshToken string `env:"CHROME_REFRESH_TOKEN,notEmpty"`
	}

	cfg := config{}
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	client := chrome.Client{
		URL:          "https://accounts.google.com/o/oauth2/token",
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RefreshToken: cfg.RefreshToken,
	}

	store := chrome.Store{
		Client: &client,
		URL: &url.URL{
			Scheme: "https",
			Host:   "www.googleapis.com",
		},
	}

	return &store, nil
}

func getFirefoxStore() (*firefox.Store, error) {
	const DefaultBaseURL = "addons.mozilla.org"

	type config struct {
		ClientID     string `env:"FIREFOX_CLIENT_ID,notEmpty"`
		ClientSecret string `env:"FIREFOX_CLIENT_SECRET,notEmpty"`
		BaseURL      string `env:"FIREFOX_BASE_URL"`
	}

	cfg := config{
		BaseURL: DefaultBaseURL,
	}
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	firefoxAPI := firefoxapi.NewAPI(firefoxapi.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		URL: &url.URL{
			Scheme: "https",
			Host:   cfg.BaseURL,
		},
	})

	store := &firefox.Store{API: firefoxAPI}

	return store, nil
}

func getEdgeStore() (*edge.Store, error) {
	type config struct {
		ClientID       string `env:"EDGE_CLIENT_ID,notEmpty"`
		ClientSecret   string `env:"EDGE_CLIENT_SECRET"`
		AccessTokenURL string `env:"EDGE_ACCESS_TOKEN_URL"`
		APIKey         string `env:"EDGE_API_KEY"`
		APIVersion     string `env:"EDGE_API_VERSION" envDefault:"v1"`
	}

	cfg := config{}

	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	var clientConfig edge.ClientConfig

	if cfg.APIVersion == edge.APIVersionV1 {
		if cfg.ClientSecret == "" || cfg.AccessTokenURL == "" {
			return nil, fmt.Errorf("EDGE_CLIENT_SECRET and EDGE_ACCESS_TOKEN_URL are required for v1")
		}
		accessTokenURL, err := url.Parse(cfg.AccessTokenURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse access token URL: %w", err)
		}
		clientConfig = edge.NewV1Config(cfg.ClientID, cfg.ClientSecret, accessTokenURL)
	} else if cfg.APIVersion == edge.APIVersionV1_1 {
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("EDGE_API_KEY is required for v1.1")
		}
		clientConfig = edge.NewV1_1Config(cfg.ClientID, cfg.APIKey)
	} else {
		return nil, fmt.Errorf("unsupported API version: %s", cfg.APIVersion)
	}

	client := edge.NewClient(clientConfig)

	store := edge.Store{
		Client: client,
		URL: &url.URL{
			Scheme: "https",
			Host:   "api.addons.microsoftedge.microsoft.com",
		},
	}

	return &store, nil
}

func firefoxStatusAction(c *cli.Context) error {
	store, err := getFirefoxStore()
	if err != nil {
		return fmt.Errorf("initializing firefox store: %w", err)
	}

	appID := c.String("app")

	status, err := store.Status(appID)
	if err != nil {
		return fmt.Errorf("getting status: %w", err)
	}

	fmt.Printf("%+v\n", status)

	return nil
}

func chromeStatusAction(c *cli.Context) error {
	store, err := getChromeStore()
	if err != nil {
		return fmt.Errorf("initializing chrome store: %w", err)
	}

	appID := c.String("app")
	status, err := store.Status(appID)
	if err != nil {
		return fmt.Errorf("getting status from chrome store: %w", err)
	}

	fmt.Printf("%s\n", status)

	return nil
}

func chromeInsertAction(c *cli.Context) error {
	store, err := getChromeStore()
	if err != nil {
		return fmt.Errorf("initializing chrome store: %w", err)
	}

	filepath := c.String("file")

	result, err := store.Insert(filepath)
	if err != nil {
		return fmt.Errorf("inserting extension: %w", err)
	}

	fmt.Println(result)

	return nil
}

func edgeInsertAction(_ *cli.Context) error {
	store, err := getEdgeStore()
	if err != nil {
		return fmt.Errorf("initializing edge store: %w", err)
	}

	result, err := store.Insert()
	if err != nil {
		return fmt.Errorf("inserting extension: %w", err)
	}

	fmt.Println(result)

	return nil
}

func firefoxInsertAction(c *cli.Context) error {
	store, err := getFirefoxStore()
	if err != nil {
		return fmt.Errorf("initializing firefox store: %w", err)
	}

	filepath := c.String("file")
	sourcepath := c.String("source")

	err = store.Insert(filepath, sourcepath)
	if err != nil {
		return fmt.Errorf("inserting extension: %w", err)
	}

	fmt.Println("extension inserted")

	return nil
}

func chromeUpdateAction(c *cli.Context) error {
	store, err := getChromeStore()
	if err != nil {
		return fmt.Errorf("initializing chrome store: %w", err)
	}

	filepath := c.String("file")
	appID := c.String("app")

	result, err := store.Update(appID, filepath)
	if err != nil {
		return fmt.Errorf("updating extension: %w", err)
	}

	fmt.Printf("updated: %v", result)

	return nil
}

func firefoxUpdateAction(c *cli.Context) error {
	store, err := getFirefoxStore()
	if err != nil {
		return fmt.Errorf("getting firefox store: %w", err)
	}

	filepath := c.String("file")
	sourcepath := c.String("source")
	channel, err := firefox.NewChannel(c.String("channel"))
	if err != nil {
		return fmt.Errorf("parsing channel: %w", err)
	}

	err = store.Update(filepath, sourcepath, channel)
	if err != nil {
		return fmt.Errorf("updating extension: %w", err)
	}

	fmt.Println("extension updated")

	return nil
}

func edgeUpdateAction(c *cli.Context) error {
	store, err := getEdgeStore()
	if err != nil {
		return fmt.Errorf("getting edge store: %w", err)
	}

	filepath := c.String("file")
	appID := c.String("app")
	timeout := c.Int("timeout")

	result, err := store.Update(appID, filepath, edge.UpdateOptions{
		UploadTimeout: time.Duration(timeout) * time.Second,
	})
	if err != nil {
		return fmt.Errorf("updating extension: %w", err)
	}

	fmt.Println(result)

	return nil
}

func chromePublishAction(c *cli.Context) error {
	store, err := getChromeStore()
	if err != nil {
		return fmt.Errorf("initializing chrome store: %w", err)
	}

	appID := c.String("app")

	// percentage is a pointer to distinguish between unset (nil) and zero values,
	// allowing optional deployment percentage in the API call
	var percentage *int
	if c.IsSet("percentage") {
		p := c.Int("percentage")
		percentage = &p
	}

	opts := &chrome.PublishOptions{
		Target:           c.String("target"),
		DeployPercentage: percentage,
		ReviewExemption:  c.Bool("expedited"),
	}

	result, err := store.Publish(appID, opts)
	if err != nil {
		return fmt.Errorf("publishing extension: %w", err)
	}

	fmt.Println(result)

	return nil
}

func edgePublishAction(c *cli.Context) error {
	store, err := getEdgeStore()
	if err != nil {
		return fmt.Errorf("getting edge store: %w", err)
	}

	appID := c.String("app")

	result, err := store.Publish(appID)
	if err != nil {
		return fmt.Errorf("publishing extension: %w", err)
	}

	fmt.Println(result)

	return nil
}

func firefoxSignAction(c *cli.Context) error {
	store, err := getFirefoxStore()
	if err != nil {
		return fmt.Errorf("getting firefox store: %w", err)
	}

	filepath := c.String("file")
	sourcepath := c.String("source")
	output := c.String("output")

	err = store.Sign(filepath, sourcepath, output)
	if err != nil {
		return fmt.Errorf("signing extension: %w", err)
	}

	fmt.Printf("Signed file saved to %s\n", output)
	return nil
}

// Main is the entry point for the command-line application.
func Main() {
	// we don't care if method fails on reading .env file, we will try to read config from environment
	// variables later
	_ = godotenv.Load()

	app := &cli.App{
		Name:  "webext",
		Usage: "CLI app for managing extensions in the stores",
		Before: func(ctx *cli.Context) error {
			verbose := ctx.Bool("verbose")
			if verbose {
				log.SetLevel(log.DEBUG)
			}
			return nil
		},
	}

	appFlag := &cli.StringFlag{Name: "app", Aliases: []string{"a"}, Required: true}
	fileFlag := &cli.StringFlag{Name: "file", Aliases: []string{"f"}, Required: true}
	sourceFlag := &cli.StringFlag{Name: "source", Aliases: []string{"s"}}
	timeoutFlag := &cli.IntFlag{
		Name:        "timeout",
		Aliases:     []string{"t"},
		Usage:       "timeout in seconds",
		DefaultText: fmt.Sprintf("%ds", int(edge.DefaultUploadTimeout.Seconds())),
	}
	verboseFlag := &cli.BoolFlag{
		Name:     "verbose",
		Aliases:  []string{"v"},
		Usage:    "increase verbosity",
		Category: "Miscellaneous:",
	}
	channelFlag := &cli.StringFlag{Name: "channel", Aliases: []string{"c"}, Required: true}

	app.Flags = []cli.Flag{verboseFlag}

	app.Commands = []*cli.Command{{
		Name:  "status",
		Usage: "returns extension info",
		Subcommands: []*cli.Command{{
			Name:   "firefox",
			Usage:  "Firefox Store",
			Action: firefoxStatusAction,
			Flags:  []cli.Flag{appFlag},
		}, {
			Name:   "chrome",
			Usage:  "Chrome Store",
			Action: chromeStatusAction,
			Flags:  []cli.Flag{appFlag},
		}},
	}, {
		Name:  "insert",
		Usage: "uploads extension to the store",
		Subcommands: []*cli.Command{{
			Name:   "chrome",
			Usage:  "inserts new extension to the chrome store",
			Flags:  []cli.Flag{fileFlag},
			Action: chromeInsertAction,
		}, {
			Name:   "edge",
			Usage:  "inserts new extension to the edge store",
			Action: edgeInsertAction,
		}, {
			Name:  "firefox",
			Usage: "inserts new extension to the firefox store",
			Flags: []cli.Flag{
				fileFlag,
				sourceFlag,
			},
			Action: firefoxInsertAction,
		}},
	}, {
		Name:  "update",
		Usage: "uploads new version of extension to the store",
		Subcommands: []*cli.Command{{
			Name:  "chrome",
			Usage: "updates version of extension in the chrome store",
			Flags: []cli.Flag{
				appFlag,
				fileFlag,
			},
			Action: chromeUpdateAction,
		}, {
			Name:  "firefox",
			Usage: "updates version of extension in the firefox store",
			Flags: []cli.Flag{
				fileFlag,
				sourceFlag,
				channelFlag,
			},
			Action: firefoxUpdateAction,
		}, {
			Name:  "edge",
			Usage: "updates version of extension in the edge store",
			Flags: []cli.Flag{
				fileFlag,
				appFlag,
				timeoutFlag,
			},
			Action: edgeUpdateAction,
		}},
	}, {
		Name:  "publish",
		Usage: "publishes extension to the store",
		Subcommands: []*cli.Command{{
			Name:  "chrome",
			Usage: "publishes extension in the chrome store",
			Flags: []cli.Flag{
				appFlag,
				&cli.StringFlag{
					Name:    "target",
					Aliases: []string{"t"},
					Usage:   "publish target (trustedTesters or default)",
				},
				&cli.IntFlag{
					Name:    "percentage",
					Aliases: []string{"p"},
					Usage:   "percentage of existing users to receive update (0-100, new users always get latest)",
				},
				&cli.BoolFlag{
					Name:    "expedited",
					Aliases: []string{"e"},
					Usage:   "request expedited review",
				},
			},
			Action: chromePublishAction,
		}, {
			Name:  "edge",
			Usage: "publishes extension in the edge store",
			Flags: []cli.Flag{
				appFlag,
			},
			Action: edgePublishAction,
		}},
	}, {
		Name:  "sign",
		Usage: "signs extension in the store",
		Subcommands: []*cli.Command{{
			Name:  "firefox",
			Usage: "signs extension in the firefox store",
			Flags: []cli.Flag{
				fileFlag,
				sourceFlag,
				&cli.StringFlag{
					Name:     "output",
					Aliases:  []string{"o"},
					Value:    "firefox.xpi", // Default value
					Required: false,
				},
			},
			Action: firefoxSignAction,
		}},
	}}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("error occurred: %s", err)
	}
}
