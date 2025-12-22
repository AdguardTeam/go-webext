// Package cmd contains the command-line interface for the application.
package cmd

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/AdguardTeam/golibs/logutil/slogutil"
	"github.com/AdguardTeam/golibs/validate"
	"github.com/adguardteam/go-webext/internal/chrome"
	"github.com/adguardteam/go-webext/internal/edge"
	"github.com/adguardteam/go-webext/internal/firefox"
	firefoxapi "github.com/adguardteam/go-webext/internal/firefox/api"
	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"
)

const (
	chromeAPIVersionV1 = "v1"
	chromeAPIVersionV2 = "v2"
)

type chromeConfig struct {
	ClientID     string `env:"CHROME_CLIENT_ID,notEmpty"`
	ClientSecret string `env:"CHROME_CLIENT_SECRET,notEmpty"`
	RefreshToken string `env:"CHROME_REFRESH_TOKEN,notEmpty"`
	PublisherID  string `env:"CHROME_PUBLISHER_ID"` // Required only for v2
	APIVersion   string `env:"CHROME_API_VERSION" envDefault:"v1"`
}

func newChromeConfig() (*chromeConfig, error) {
	cfg := &chromeConfig{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse Chrome environment variables: %w", err)
	}
	return cfg, nil
}

func getChromeV1Store() (*chrome.StoreV1, error) {
	cfg, err := newChromeConfig()
	if err != nil {
		return nil, err
	}

	chromeLogger := slog.Default().With(slogutil.KeyPrefix, "chrome")

	client := chrome.NewClient(chrome.ClientConfig{
		URL:          "https://accounts.google.com/o/oauth2/token",
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RefreshToken: cfg.RefreshToken,
		Logger:       chromeLogger,
	})

	store := chrome.NewStoreV1(chrome.StoreV1Config{
		Client: client,
		URL: &url.URL{
			Scheme: "https",
			Host:   "www.googleapis.com",
		},
		Logger: chromeLogger,
	})

	return store, nil
}

func getChromeV2Store() (*chrome.StoreV2, error) {
	cfg, err := newChromeConfig()
	if err != nil {
		return nil, err
	}

	if err := validate.NotEmpty("CHROME_PUBLISHER_ID", cfg.PublisherID); err != nil {
		return nil, err
	}

	chromeLogger := slog.Default().With(slogutil.KeyPrefix, "chrome")

	client := chrome.NewClient(chrome.ClientConfig{
		URL:          "https://accounts.google.com/o/oauth2/token",
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RefreshToken: cfg.RefreshToken,
		Logger:       chromeLogger,
	})

	store := chrome.NewStoreV2(chrome.StoreV2Config{
		Client: client,
		URL: &url.URL{
			Scheme: "https",
			Host:   "chromewebstore.googleapis.com",
		},
		PublisherID: cfg.PublisherID,
		Logger:      chromeLogger,
	})

	return store, nil
}

// chromeStore holds either a V1 or V2 store based on API version.
type chromeStore struct {
	v1         *chrome.StoreV1
	v2         *chrome.StoreV2
	apiVersion string
}

// getChromeStore returns a chrome store supporting the configured API version.
func getChromeStore() (*chromeStore, error) {
	cfg, err := newChromeConfig()
	if err != nil {
		return nil, err
	}

	apiVersion := cfg.APIVersion

	switch apiVersion {
	case chromeAPIVersionV1:
		store, err := getChromeV1Store()
		if err != nil {
			return nil, fmt.Errorf("initializing chrome store v1: %w", err)
		}
		return &chromeStore{v1: store, apiVersion: apiVersion}, nil
	case chromeAPIVersionV2:
		store, err := getChromeV2Store()
		if err != nil {
			return nil, fmt.Errorf("initializing chrome store v2: %w", err)
		}
		return &chromeStore{v2: store, apiVersion: apiVersion}, nil
	default:
		return nil, fmt.Errorf("invalid CHROME_API_VERSION: %s (must be %s or %s)", apiVersion, chromeAPIVersionV1, chromeAPIVersionV2)
	}
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
		Logger: slog.Default().With(slogutil.KeyPrefix, "firefox/api"),
	})

	store := firefox.NewStore(firefox.StoreConfig{
		API:    firefoxAPI,
		Logger: slog.Default().With(slogutil.KeyPrefix, "firefox"),
	})

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
		if err := validate.NotEmpty("EDGE_CLIENT_SECRET", cfg.ClientSecret); err != nil {
			return nil, err
		}
		if err := validate.NotEmpty("EDGE_ACCESS_TOKEN_URL", cfg.AccessTokenURL); err != nil {
			return nil, err
		}
		accessTokenURL, err := url.Parse(cfg.AccessTokenURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse access token URL: %w", err)
		}
		clientConfig = edge.NewV1Config(cfg.ClientID, cfg.ClientSecret, accessTokenURL)
	} else if cfg.APIVersion == edge.APIVersionV1_1 {
		if err := validate.NotEmpty("EDGE_API_KEY", cfg.APIKey); err != nil {
			return nil, err
		}
		clientConfig = edge.NewV1_1Config(cfg.ClientID, cfg.APIKey)
	} else {
		return nil, fmt.Errorf("unsupported API version: %s", cfg.APIVersion)
	}

	client := edge.NewClient(clientConfig)

	store := edge.NewStore(edge.StoreConfig{
		Client: client,
		URL: &url.URL{
			Scheme: "https",
			Host:   "api.addons.microsoftedge.microsoft.com",
		},
		Logger: slog.Default().With(slogutil.KeyPrefix, "edge"),
	})

	return store, nil
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
		return err
	}

	appID := c.String("app")

	switch store.apiVersion {
	case chromeAPIVersionV1:
		status, err := store.v1.Status(appID)
		if err != nil {
			return fmt.Errorf("getting status: %w", err)
		}

		fmt.Printf("Item ID: %s\n", status.ID)
		fmt.Printf("Upload State: %s\n", status.UploadStateV1)
		if status.CrxVersion != "" {
			fmt.Printf("Version: %s\n", status.CrxVersion)
		}
	case chromeAPIVersionV2:
		status, err := store.v2.Status(appID)
		if err != nil {
			return fmt.Errorf("getting status: %w", err)
		}

		fmt.Printf("Item ID: %s\n", status.ItemID)
		if status.PublishedItemRevisionStatus != nil {
			fmt.Printf("Published State: %s\n", status.PublishedItemRevisionStatus.State.String())
			if len(status.PublishedItemRevisionStatus.DistributionChannels) > 0 {
				fmt.Printf("Published Version: %s\n", status.PublishedItemRevisionStatus.DistributionChannels[0].CrxVersion)
				fmt.Printf("Rollout: %d%%\n", status.PublishedItemRevisionStatus.DistributionChannels[0].DeployPercentage)
			}
		}
		if status.SubmittedItemRevisionStatus != nil {
			fmt.Printf("Submitted State: %s\n", status.SubmittedItemRevisionStatus.State.String())
		}
	}

	return nil
}

func chromeInsertAction(c *cli.Context) error {
	store, err := getChromeV1Store()
	if err != nil {
		return fmt.Errorf("initializing chrome store: %w", err)
	}

	filepath := c.String("file")

	result, err := store.Insert(filepath)
	if err != nil {
		return fmt.Errorf("inserting extension: %w", err)
	}

	fmt.Println("Insert completed")
	fmt.Printf("Item ID: %s\n", result.ID)
	fmt.Printf("Upload State: %s\n", result.UploadStateV1)

	return nil
}

func chromeUpdateAction(c *cli.Context) error {
	store, err := getChromeStore()
	if err != nil {
		return err
	}

	filepath := c.String("file")
	appID := c.String("app")

	switch store.apiVersion {
	case chromeAPIVersionV1:
		result, err := store.v1.Update(appID, filepath)
		if err != nil {
			return fmt.Errorf("updating extension: %w", err)
		}

		fmt.Println("Update completed")
		fmt.Printf("Item ID: %s\n", result.ID)
		fmt.Printf("Upload State: %s\n", result.UploadStateV1)
	case chromeAPIVersionV2:
		result, err := store.v2.Upload(appID, filepath)
		if err != nil {
			return fmt.Errorf("uploading extension: %w", err)
		}

		fmt.Println("Upload completed")
		fmt.Printf("Item ID: %s\n", result.ItemID)
		fmt.Printf("Version: %s\n", result.CrxVersion)
		fmt.Printf("Upload State: %s\n", result.UploadStateV2)
	}

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
		return err
	}

	appID := c.String("app")

	switch store.apiVersion {
	case chromeAPIVersionV1:
		opts := &chrome.PublishOptionsV1{}

		target := c.String("target")
		if target == "trustedTesters" {
			opts.Target = "trustedTesters"
		}

		if c.IsSet("percentage") {
			p := c.Int("percentage")
			opts.DeployPercentage = &p
		}

		opts.ReviewExemption = c.Bool("expedited")

		result, err := store.v1.Publish(appID, opts)
		if err != nil {
			return fmt.Errorf("publishing extension: %w", err)
		}

		fmt.Println("Publish operation completed")
		fmt.Printf("Item ID: %s\n", result.ItemID)
		if len(result.Status) > 0 {
			fmt.Printf("Status: %v\n", result.Status)
		}
	case chromeAPIVersionV2:
		opts := &chrome.PublishOptions{
			PublishType: chrome.PublishTypeDefault,
		}

		if c.Bool("staged") {
			opts.PublishType = chrome.PublishTypeStaged
		}

		if c.IsSet("percentage") {
			p := c.Int("percentage")
			opts.DeployInfos = []chrome.DeployInfo{{DeployPercentage: p}}
		}

		opts.SkipReview = c.Bool("expedited")

		result, err := store.v2.Publish(appID, opts)
		if err != nil {
			return fmt.Errorf("publishing extension: %w", err)
		}

		fmt.Println("Publish operation completed")
		fmt.Printf("Item ID: %s\n", result.ItemID)
		fmt.Printf("State: %s\n", result.State.String())
	}

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
			logLevel := slog.LevelInfo
			if verbose {
				logLevel = slog.LevelDebug
			}

			handler := slogutil.New(&slogutil.Config{
				Level:        logLevel,
				AddTimestamp: true,
				Format:       slogutil.FormatText, // or FormatJSON if needed
			})
			slog.SetDefault(handler)

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
			Usage:  "inserts new extension to the chrome store (v1 API)",
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
				&cli.BoolFlag{
					Name:    "staged",
					Aliases: []string{"s"},
					Usage:   "(v2 only) stage for publishing in the future instead of publishing immediately",
				},
				&cli.StringFlag{
					Name:    "target",
					Aliases: []string{"t"},
					Usage:   "(v1 only) publish target (trustedTesters or default)",
				},
				&cli.IntFlag{
					Name:    "percentage",
					Aliases: []string{"p"},
					Usage:   "deployment percentage (0-100) for gradual rollout",
				},
				&cli.BoolFlag{
					Name:    "expedited",
					Aliases: []string{"e"},
					Usage:   "request skip review if qualified",
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
		slog.Error(
			"fatal error occurred",
			"error", err,
		)
		os.Exit(1)
	}
}
