package cta

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type busTrackerTimeResponse struct {
	BustimeResponse struct {
		Error []string `json:"error,omitempty"`
		TM    string   `json:"tm"`
	} `json:"bustime-response"`
}

func (c *BusClient) GetUnixTime(ctx context.Context) (int64, error) {
	timeStr, err := c.getTime(ctx, true)
	if err != nil {
		return 0, err
	}
	unixTime, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing unix time: %w", err)
	}
	return unixTime, nil
}

func (c *BusClient) GetTime(ctx context.Context) (time.Time, error) {
	unixTime, err := c.GetUnixTime(ctx)
	if err != nil {
		return time.Time{}, err
	}
	return unixTimeToTimeUnknownGranularity(unixTime), nil
}

func (c *BusClient) GetStringTime(ctx context.Context) (string, error) {
	return c.getTime(ctx, false)
}

func (c *BusClient) getTime(ctx context.Context, unixTime bool) (string, error) {
	if c == nil {
		return "", errors.New("nil client")
	}

	query := map[string]string{}
	if unixTime {
		query["unixTime"] = "true"
	}

	time, err := processRequest[busTrackerTimeResponse](c.c, ctx, "gettime", query)
	if err != nil {
		return "", fmt.Errorf("getting time: %w", err)
	}
	if len(time.BustimeResponse.Error) > 0 {
		return "", errors.New(strings.Join(time.BustimeResponse.Error, "; "))
	}
	return time.BustimeResponse.TM, nil
}
