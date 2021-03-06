//go:generate go-bindata -o bin_data.go --pkg lang --nometadata --prefix ../../../../lang ../../../../lang/...

package lang

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"meguca/common"
	"meguca/config"
)

var (
	// Packs contains all loaded language packs
	Packs map[string]Pack

	// Precompiled table of relations between browser Accept-Language HTTP
	// header values and internal POSIX language codes
	languageCodes map[string]string
)

// Pack contains a localization language pack for a single language
type Pack struct {
	ID              string
	Tabs, SortModes []string
	Forms           map[string][2]string
	UI, Options     map[string]string
	Templates       map[string][]string
	Common
}

// Common contains localization common to both the server and the client
type Common struct {
	Posts, UI map[string]string
	Time      map[string][]string
	Plurals   map[string][2]string
}

// Load loads and parses all JSON language packs
func Load() error {
	n := len(common.Langs)
	Packs = make(map[string]Pack, n)
	languageCodes = make(map[string]string, n*2)

	for _, lang := range common.Langs {
		buf, err := Asset(filepath.Join(lang, "server.json"))
		if err != nil {
			return err
		}
		var p Pack
		if err := json.Unmarshal(buf, &p); err != nil {
			return err
		}

		buf, err = Asset(filepath.Join(lang, "common.json"))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(buf, &p.Common); err != nil {
			return err
		}

		p.ID = lang
		Packs[lang] = p

		// Compute all possible Accept-Language HTTP header values that could
		// point to this language pack
		languageCodes[strings.Replace(p.ID, "_", "-", 1)] = p.ID
		languageCodes[p.ID[:2]] = p.ID
	}

	return nil
}

// Get determines the language pack of the client based on fallback order:
// cookie || Accept-Language header || default language
func Get(w http.ResponseWriter, r *http.Request) (Pack, error) {
	var id string

	switch cookie, err := r.Cookie("lang"); err {
	case http.ErrNoCookie:
	case nil:
		if _, ok := Packs[cookie.Value]; ok {
			id = cookie.Value
		}
	default:
		return Pack{}, err
	}

	if id == "" {
		for _, h := range r.Header["Accept-Language"] {
			if internal, ok := languageCodes[h]; ok {
				id = internal
				break
			}
		}
	}

	if id == "" {
		id = config.Get().DefaultLang
	}

	w.Header().Set("Content-Language", strings.Replace(id, "_", "-", 1))
	return Packs[id], nil
}
