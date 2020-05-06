package main

import (
	"net/http"
	"strings"
)

func getGenericURL(t poolConfigURLTemplate, id string) string {
	if strings.Contains(t.Template, t.Pattern) == false {
		return ""
	}

	return strings.Replace(t.Template, t.Pattern, id, -1)
}

func (s *searchContext) getSirsiURL(id string) string {
	return getGenericURL(s.pool.config.Global.Service.URLTemplates.Sirsi, id)
}

func (s *searchContext) getCoverImageURL(cfg *poolConfigFieldTypeCoverImageURL, doc *solrDocument, authorValues []string) string {
	// use solr-provided url if present

	thumbnailValues := doc.getValuesByTag(cfg.ThumbnailField)

	if thumbnailURL := firstElementOf(thumbnailValues); thumbnailURL != "" {
		return thumbnailURL
	}

	// otherwise, compose a url to the cover image service

	idValues := doc.getValuesByTag(cfg.IDField)

	url := getGenericURL(s.pool.config.Global.Service.URLTemplates.CoverImages, firstElementOf(idValues))

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

	titleValues := doc.getValuesByTag(cfg.TitleField)
	poolValues := doc.getValuesByTag(cfg.PoolField)

	// remove extraneous dates from author
	author := strings.Trim(strings.Split(firstElementOf(authorValues), "[")[0], " ")
	title := firstElementOf(titleValues)

	if sliceContainsString(poolValues, cfg.MusicPool) == true {
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

	isbnValues := doc.getValuesByTag(cfg.ISBNField)
	if len(isbnValues) > 0 {
		qp.Add("isbn", strings.Join(isbnValues, ","))
	}

	oclcValues := doc.getValuesByTag(cfg.OCLCField)
	if len(oclcValues) > 0 {
		qp.Add("oclc", strings.Join(oclcValues, ","))
	}

	lccnValues := doc.getValuesByTag(cfg.LCCNField)
	if len(lccnValues) > 0 {
		qp.Add("lccn", strings.Join(lccnValues, ","))
	}

	upcValues := doc.getValuesByTag(cfg.UPCField)
	if len(upcValues) > 0 {
		qp.Add("upc", strings.Join(upcValues, ","))
	}

	req.URL.RawQuery = qp.Encode()

	return req.URL.String()
}

func (s *searchContext) getIIIFBaseURL(doc *solrDocument, idField string) string {
	// FIXME: update after iiif_image_url is correct

	// construct iiif image base url from known image identifier prefixes.
	// this fallback url conveniently points to an "orginial image missing" image

	pid := s.pool.config.Global.Service.URLTemplates.IIIF.Fallback

	idValues := doc.getValuesByTag(idField)

	for _, id := range idValues {
		for _, prefix := range s.pool.config.Global.Service.URLTemplates.IIIF.Prefixes {
			if strings.HasPrefix(id, prefix) {
				pid = id
				break
			}
		}
	}

	return getGenericURL(s.pool.config.Global.Service.URLTemplates.IIIF, pid)
}

func (s *searchContext) getDigitalContentURL(doc *solrDocument, idField string) string {
	idValues := doc.getValuesByTag(idField)

	id := firstElementOf(idValues)

	return getGenericURL(s.pool.config.Global.Service.URLTemplates.DigitalContent, id)
}
