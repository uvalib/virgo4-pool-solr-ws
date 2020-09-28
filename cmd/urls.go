package main

import (
	"net/http"
	"strings"
)

func getGenericURL(t poolConfigURLTemplate, id string) string {
	if strings.Contains(t.Path, t.Pattern) == false {
		return ""
	}

	return t.Host + strings.Replace(t.Path, t.Pattern, id, -1)
}

func (s *searchContext) getSirsiURL(id string) string {
	return getGenericURL(s.pool.config.Global.Service.URLTemplates.Sirsi, id)
}

func (s *searchContext) getCoverImageURL(cfg *poolConfigFieldTypeCustom, doc *solrDocument, authorValues []string) string {
	// compose a url to the cover image service

	id := doc.getFirstString(cfg.IDField)

	url := getGenericURL(s.pool.config.Global.Service.URLTemplates.CoverImages, id)

	if url == "" {
		return ""
	}

	// also add query parameters:
	// doc_type: music or non_music
	// books require at least one of: isbn, oclc, lccn, upc
	// music requires: artist_name, album_name
	// all else is optional

	// build query parameters using http package to properly quote values
	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		return ""
	}

	qp := req.URL.Query()

	title := doc.getFirstString(cfg.TitleField)
	poolValues := doc.getStrings(cfg.PoolField)

	// remove extraneous dates from author
	author := strings.Trim(strings.Split(firstElementOf(authorValues), "[")[0], " ")

	if sliceContainsString(poolValues, cfg.MusicPool, true) == true {
		// music

		qp.Add("doc_type", "music")

		if len(author) > 0 {
			qp.Add("artist_name", author)
		}

		if len(title) > 0 {
			qp.Add("album_name", title)
		}
	} else {
		// books... and everything else

		qp.Add("doc_type", "non_music")

		if len(title) > 0 {
			qp.Add("title", title)
		}
	}

	// always throw these optional values at the cover image service

	isbnValues := doc.getStrings(cfg.ISBNField)
	if len(isbnValues) > 0 {
		qp.Add("isbn", strings.Join(isbnValues, ","))
	}

	oclcValues := doc.getStrings(cfg.OCLCField)
	if len(oclcValues) > 0 {
		qp.Add("oclc", strings.Join(oclcValues, ","))
	}

	lccnValues := doc.getStrings(cfg.LCCNField)
	if len(lccnValues) > 0 {
		qp.Add("lccn", strings.Join(lccnValues, ","))
	}

	upcValues := doc.getStrings(cfg.UPCField)
	if len(upcValues) > 0 {
		qp.Add("upc", strings.Join(upcValues, ","))
	}

	req.URL.RawQuery = qp.Encode()

	return req.URL.String()
}

func (s *searchContext) getDigitalContentURL(doc *solrDocument, idField string) string {
	id := doc.getFirstString(idField)

	return getGenericURL(s.pool.config.Global.Service.URLTemplates.DigitalContent, id)
}
