package cta

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

const (
	OS_ENV_BUS_API_KEY   = "CTA_BUS_API_KEY"
	OS_ENV_TRAIN_API_KEY = "CTA_TRAIN_API_KEY"
)

type Client struct {
	apiKey       string
	baseURL      *url.URL
	HTTPClient   *http.Client
	logger       *slog.Logger
	redactLogger bool
	locale       string
}

func newClientFromEnv(baseUrl string, envVar string) (*Client, error) {
	apiKey, apiKeyFound := os.LookupEnv(envVar)
	if !apiKeyFound {
		return nil, errors.New(envVar + " environment variable not set")
	}
	return newClient(baseUrl, apiKey)
}

func newClient(baseUrl string, apiKey string) (*Client, error) {
	baseURL, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	return &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		HTTPClient: &http.Client{},
		logger:     nil,
		locale:     "en",
	}, nil
}

func (c *Client) getBaseQueryParams() map[string]string {
	return map[string]string{
		"key":    c.apiKey,
		"format": "json",
		"locale": c.locale,
	}
}

func (c *Client) getRequest(ctx context.Context, endpoint string, queryParams map[string]string) (*http.Request, error) {
	fullQueryParams := c.getBaseQueryParams()
	if queryParams != nil {
		maps.Copy(fullQueryParams, queryParams)
	}

	u := *c.baseURL
	endpoint = strings.TrimPrefix(endpoint, "/")
	endpoint = strings.TrimSuffix(endpoint, "/")
	u.Path = path.Join(u.Path, endpoint)

	// Build query string
	vals := url.Values{}
	for k, v := range fullQueryParams {
		vals.Set(k, v)
	}
	u.RawQuery = vals.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (c *Client) getUrlAsString(u *url.URL) string {
	urlStr := u.String()
	if c.redactLogger {
		return redactApiKey(urlStr)
	}
	return urlStr
}

func processRequest[T any](c *Client, ctx context.Context, endpoint string, queryParams map[string]string) (T, error) {
	getReq, err := c.getRequest(ctx, endpoint, queryParams)
	if c.logger != nil {
		c.logger.Info("Sending Request: ", slog.String("url", c.getUrlAsString(getReq.URL)))
	}
	var zero T
	if err != nil {
		return zero, err
	}
	resp, err := c.HTTPClient.Do(getReq)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if c.logger != nil {
		c.logger.Info("Got response code", slog.Int("resp", resp.StatusCode))
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			c.logger.Info("Got Response: ", slog.String("body", string(bodyBytes)))
		}
	}

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return zero, fmt.Errorf("decoding response: %w", err)
	}
	return out, nil
}

const (
	BUS_BASE_URL = "https://www.ctabustracker.com/bustime/api/v3/"
)

type BusClient struct {
	c *Client
}

func NewBusClient(apiKey string) (*BusClient, error) {
	c, err := newClient(BUS_BASE_URL, apiKey)
	if err != nil {
		return nil, err
	}
	return &BusClient{c: c}, nil
}

func NewBusClientFromEnv() (*BusClient, error) {
	c, err := newClientFromEnv(BUS_BASE_URL, OS_ENV_BUS_API_KEY)
	if err != nil {
		return nil, err
	}
	return &BusClient{c: c}, nil
}

func (bc *BusClient) WithLogger(logger *slog.Logger) *BusClient {
	bc.c.logger = logger
	bc.c.redactLogger = true
	return bc
}

func (bc *BusClient) WithUnredactedLogger(logger *slog.Logger) *BusClient {
	bc.c.logger = logger
	bc.c.redactLogger = false
	logger.Warn("Using an unredacted logger will print your API key to terminal. Be very careful!")
	return bc
}

func (bc *BusClient) SpanishLocale() *BusClient {
	if bc.c.logger != nil {
		bc.c.logger.Warn("Spanish locale is very poorly supported by CTA. ¡Lo siento!")
	}
	bc.c.locale = "es"
	return bc
}

func (bc *BusClient) EnglishLocale() *BusClient {
	bc.c.locale = "en"
	return bc
}

const (
	TRAIN_BASE_URL = "https://lapi.transitchicago.com/api/1.0/"
)

type TrainClient struct {
	c *Client
}

func NewTrainClient(apiKey string) (*TrainClient, error) {
	c, err := newClient(BUS_BASE_URL, apiKey)
	if err != nil {
		return nil, err
	}
	return &TrainClient{c: c}, nil
}

func NewTrainClientFromEnv() (*TrainClient, error) {
	c, err := newClientFromEnv(BUS_BASE_URL, OS_ENV_BUS_API_KEY)
	if err != nil {
		return nil, err
	}
	return &TrainClient{c: c}, nil
}

func (tc *TrainClient) WithLogger(logger *slog.Logger) *TrainClient {
	tc.c.logger = logger
	tc.c.redactLogger = true
	return tc
}

func (tc *TrainClient) WithUnredactedLogger(logger *slog.Logger) *TrainClient {
	tc.c.logger = logger
	tc.c.redactLogger = false
	logger.Warn("Using an unredacted logger will print your API key to terminal. Be very careful!")
	return tc
}
