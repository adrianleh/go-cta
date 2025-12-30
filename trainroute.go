package cta

import (
	"errors"
	"strings"
)

const (
	CTA_BLUE_LINE   = "Blue"
	CTA_BROWN_LINE  = "Brn"
	CTA_GREEN_LINE  = "G"
	CTA_ORANGE_LINE = "Org"
	CTA_PURPLE_LINE = "P"
	CTA_PINK_LINE   = "Pink"
	CTA_RED_LINE    = "Red"
	CTA_YELLOW_LINE = "Y"
)

func IsValidTrainRoute(route string) bool {
	switch route {
	case CTA_BLUE_LINE, CTA_BROWN_LINE, CTA_GREEN_LINE, CTA_ORANGE_LINE, CTA_PURPLE_LINE, CTA_PINK_LINE, CTA_RED_LINE, CTA_YELLOW_LINE:
		return true
	default:
		return false
	}
}

func GetFriendlyTrainRouteName(route string) string {
	switch strings.ToLower(route) {
	case strings.ToLower(CTA_BLUE_LINE):
		return "Blue Line"
	case strings.ToLower(CTA_BROWN_LINE):
		return "Brown Line"
	case strings.ToLower(CTA_GREEN_LINE):
		return "Green Line"
	case strings.ToLower(CTA_ORANGE_LINE):
		return "Orange Line"
	case strings.ToLower(CTA_PURPLE_LINE):
		return "Purple Line"
	case strings.ToLower(CTA_PINK_LINE):
		return "Pink Line"
	case strings.ToLower(CTA_RED_LINE):
		return "Red Line"
	case strings.ToLower(CTA_YELLOW_LINE):
		return "Yellow Line"
	default:
		return route
	}
}

func FriendlyNameToTrainRoute(friendlyName string) (string, error) {
	cleanFriendlyName := strings.ToLower(friendlyName)
	if strings.HasSuffix(cleanFriendlyName, "line") {
		cleanFriendlyName = strings.TrimSpace(strings.TrimSuffix(cleanFriendlyName, "line"))
	}
	switch cleanFriendlyName {
	case "blue":
		return CTA_BLUE_LINE, nil
	case "brown":
		return CTA_BROWN_LINE, nil
	case "green":
		return CTA_GREEN_LINE, nil
	case "orange":
		return CTA_ORANGE_LINE, nil
	case "purple":
		return CTA_PURPLE_LINE, nil
	case "pink":
		return CTA_PINK_LINE, nil
	case "red":
		return CTA_RED_LINE, nil
	case "yellow":
		return CTA_YELLOW_LINE, nil
	default:
		return "", errors.New("invalid friendly train route name: " + friendlyName + ". Must be one of Blue, Brown, Green, Orange, Purple, Pink, Red, or Yellow. with optional 'Line' suffix. Case insensitive.")
	}
}

func IsValidFriendlyTrainRouteName(friendlyName string) bool {
	_, err := FriendlyNameToTrainRoute(friendlyName)
	return err == nil
}
