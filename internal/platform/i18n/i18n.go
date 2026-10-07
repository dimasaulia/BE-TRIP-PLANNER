package i18n

import (
	"embed"
	"encoding/json"
	"strings"
)

//go:embed locales/*.json
var localeFiles embed.FS

type Translator struct {
	defaultLanguage string
	messages        map[string]map[string]string
}

func NewTranslator() (*Translator, error) {
	translator := &Translator{
		defaultLanguage: "id",
		messages:        map[string]map[string]string{},
	}

	for _, language := range []string{"id", "en"} {
		content, err := localeFiles.ReadFile("locales/" + language + ".json")
		if err != nil {
			return nil, err
		}

		messages := map[string]string{}
		if err := json.Unmarshal(content, &messages); err != nil {
			return nil, err
		}

		translator.messages[language] = messages
	}

	return translator, nil
}

func (t *Translator) Translate(language string, key string, params map[string]string) string {
	language = t.normalizeLanguage(language)
	message := t.find(language, key)

	for paramKey, paramValue := range params {
		message = strings.ReplaceAll(message, "{"+paramKey+"}", paramValue)
	}

	return message
}

func (t *Translator) normalizeLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		return t.defaultLanguage
	}

	if strings.HasPrefix(language, "id") {
		return "id"
	}
	if strings.HasPrefix(language, "en") {
		return "en"
	}

	return t.defaultLanguage
}

func (t *Translator) find(language string, key string) string {
	if messages, exists := t.messages[language]; exists {
		if message, exists := messages[key]; exists {
			return message
		}
	}

	if messages, exists := t.messages[t.defaultLanguage]; exists {
		if message, exists := messages[key]; exists {
			return message
		}
	}

	return key
}
