package api

import (
	"fmt"
	"html"
)

// BadgeColor represents different badge color schemes
type BadgeColor struct {
	Left  string // Color for the left side (label)
	Right string // Color for the right side (value)
}

// Predefined badge colors
var (
	BadgeColorSuccess = BadgeColor{Left: "#555", Right: "#4c1"}    // Green for success
	BadgeColorError   = BadgeColor{Left: "#555", Right: "#e05d44"} // Red for error
	BadgeColorInfo    = BadgeColor{Left: "#555", Right: "#007ec6"} // Blue for info
	BadgeColorWarning = BadgeColor{Left: "#555", Right: "#dfb317"} // Yellow for warning
	BadgeColorGray    = BadgeColor{Left: "#555", Right: "#9f9f9f"} // Gray for unknown
)

// BadgeOptions holds configuration for badge generation
type BadgeOptions struct {
	Label string     // Left side text (e.g., "production")
	Value string     // Right side text (e.g., "v1.2.3")
	Color BadgeColor // Color scheme
}

// GenerateSVGBadge creates a shields.io style SVG badge
func GenerateSVGBadge(opts BadgeOptions) string {
	// Escape HTML entities
	label := html.EscapeString(opts.Label)
	value := html.EscapeString(opts.Value)

	// Calculate text widths (approximate)
	labelWidth := calculateTextWidth(label)
	valueWidth := calculateTextWidth(value)

	// Add padding
	labelPadding := 12
	valuePadding := 12

	// Calculate total dimensions
	labelBoxWidth := labelWidth + labelPadding
	valueBoxWidth := valueWidth + valuePadding
	totalWidth := labelBoxWidth + valueBoxWidth
	height := 20

	// Generate SVG
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="%d" height="%d" role="img" aria-label="%s: %s">
  <title>%s: %s</title>
  <linearGradient id="s" x2="0" y2="100%%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="r">
    <rect width="%d" height="%d" rx="3" fill="#fff"/>
  </clipPath>
  <g clip-path="url(#r)">
    <rect width="%d" height="%d" fill="%s"/>
    <rect x="%d" width="%d" height="%d" fill="%s"/>
    <rect width="%d" height="%d" fill="url(#s)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="110">
    <text aria-hidden="true" x="%d" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="%d">%s</text>
    <text x="%d" y="140" transform="scale(.1)" fill="#fff" textLength="%d">%s</text>
    <text aria-hidden="true" x="%d" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="%d">%s</text>
    <text x="%d" y="140" transform="scale(.1)" fill="#fff" textLength="%d">%s</text>
  </g>
</svg>`,
		totalWidth, height, label, value, label, value,
		totalWidth, height,
		labelBoxWidth, height, opts.Color.Left,
		labelBoxWidth, valueBoxWidth, height, opts.Color.Right,
		totalWidth, height,
		// Label text (shadow)
		(labelBoxWidth*10)/2, labelWidth*10, label,
		// Label text (main)
		(labelBoxWidth*10)/2, labelWidth*10, label,
		// Value text (shadow)
		(labelBoxWidth*10)+(valueBoxWidth*10)/2, valueWidth*10, value,
		// Value text (main)
		(labelBoxWidth*10)+(valueBoxWidth*10)/2, valueWidth*10, value,
	)

	return svg
}

// calculateTextWidth approximates the width of text in pixels
// This is a simplified calculation based on average character widths
func calculateTextWidth(text string) int {
	// Average character width in Verdana 11px font is approximately 6.5 pixels
	// This is a rough approximation for badge sizing
	baseWidth := 6.5
	width := 0.0

	for _, char := range text {
		switch {
		case char >= 'A' && char <= 'Z':
			width += baseWidth * 1.1 // Uppercase letters are slightly wider
		case char >= 'a' && char <= 'z':
			width += baseWidth
		case char >= '0' && char <= '9':
			width += baseWidth * 0.9 // Numbers are slightly narrower
		case char == '.':
			width += baseWidth * 0.3
		case char == '-' || char == '_':
			width += baseWidth * 0.6
		case char == ' ':
			width += baseWidth * 0.4
		default:
			width += baseWidth // Default for other characters
		}
	}

	return int(width)
}

// CreateSuccessBadge creates a green badge for successful deployments
func CreateSuccessBadge(envName, version string) string {
	return GenerateSVGBadge(BadgeOptions{
		Label: envName,
		Value: version,
		Color: BadgeColorSuccess,
	})
}

// CreateErrorBadge creates a red badge for errors
func CreateErrorBadge(envName, message string) string {
	return GenerateSVGBadge(BadgeOptions{
		Label: envName,
		Value: message,
		Color: BadgeColorError,
	})
}

// CreateNotFoundBadge creates a gray badge for when no deployment is found
func CreateNotFoundBadge(envName string) string {
	return GenerateSVGBadge(BadgeOptions{
		Label: envName,
		Value: "not deployed",
		Color: BadgeColorGray,
	})
}

// CreateMultipleFoundBadge creates a warning badge for when multiple deployments are found
func CreateMultipleFoundBadge(envName string) string {
	return GenerateSVGBadge(BadgeOptions{
		Label: envName,
		Value: "multiple found",
		Color: BadgeColorWarning,
	})
}
