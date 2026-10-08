package services

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/open-suite/boilerplate-golang/internal/modules/calendar/repositories"
	"github.com/open-suite/boilerplate-golang/internal/platform/google"
)

const descriptionLimit = 300

var eventIDEncoding = base32.HexEncoding.WithPadding(base32.NoPadding)

// EventID is deterministic, so redelivering a job can never create a duplicate:
// sha256(sync_id + block_id) in lowercase base32hex, which is what Google accepts.
func EventID(syncID uuid.UUID, blockID uuid.UUID) string {
	sum := sha256.Sum256([]byte(syncID.String() + blockID.String()))
	return strings.ToLower(eventIDEncoding.EncodeToString(sum[:]))
}

// BuildEvent maps a block to a Google event as the TRD's table defines.
func BuildEvent(syncID uuid.UUID, data repositories.BlockData, appBaseURL string) google.Event {
	summary := ""
	if data.Title != nil {
		summary = *data.Title
	} else if data.ItemTitle != nil {
		summary = *data.ItemTitle
	}

	location := ""
	if data.LocationName != nil {
		location = *data.LocationName
	}

	source := data.Note
	if data.DescriptionMD != nil {
		source = *data.DescriptionMD
	}
	description := PlainText(source, descriptionLimit)
	link := fmt.Sprintf("%s/trips/%s", appBaseURL, data.TripID)
	if description != "" {
		description += "\n\n"
	}
	description += link

	return google.Event{
		ID:          EventID(syncID, data.ID),
		Summary:     summary,
		Location:    location,
		Description: description,
		Start:       data.StartAt,
		End:         data.EndAt,
		TimeZone:    data.Timezone,
	}
}

// ContentHash fingerprints everything that is sent, so an unchanged block is skipped.
func ContentHash(event google.Event) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s",
		event.ID, event.Summary, event.Location, event.Description,
		event.Start.UTC().Format("2006-01-02T15:04:05Z"), event.End.UTC().Format("2006-01-02T15:04:05Z"), event.TimeZone)
	return hex.EncodeToString(hash.Sum(nil))
}

var (
	codeFence  = regexp.MustCompile("(?s)```.*?```")
	imageMD    = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	linkMD     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	htmlTag    = regexp.MustCompile(`<[^>]+>`)
	lineMarker = regexp.MustCompile(`(?m)^\s{0,3}(#{1,6}\s+|>\s?|[-*+]\s+(\[[ xX]\]\s+)?|\d+\.\s+)`)
	emphasis   = regexp.MustCompile("[*_~`]+")
	spaces     = regexp.MustCompile(`\s+`)
)

// PlainText strips Markdown down to readable text and cuts it to limit runes.
func PlainText(markdown string, limit int) string {
	text := codeFence.ReplaceAllString(markdown, " ")
	text = imageMD.ReplaceAllString(text, " ")
	text = linkMD.ReplaceAllString(text, "$1")
	text = htmlTag.ReplaceAllString(text, " ")
	text = lineMarker.ReplaceAllString(text, "")
	text = emphasis.ReplaceAllString(text, "")
	text = strings.TrimSpace(spaces.ReplaceAllString(text, " "))

	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit])
}
