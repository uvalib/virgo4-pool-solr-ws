package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

/*
	params is :id plus query params

    def get(params)
      return unless PROXY_COVER_IMAGES
      id    = params[:id].to_s.gsub(/\.|#{ENCODED_DOT}/i, '%2E')
      path  = "cover_images/#{id}.json"
      query = to_query(params.except(:id))
      url   = "#{ENV['COVER_IMAGE_URL']}/#{path}?#{query}"
      cache_params = generate_cache_params(query)
      cache_fetch(cache_params) do
        URI.parse(url).read rescue nil
      end
    end

        var query = {};
        var data_attribute = {
            param        element      solr field
            doc_type:    'doc-type',  n/a.  "music" for sound_recordings pool, "non_music" otherwise
            title:       'title',     title_a
            album_name:  'title',     title_a
            author:      'artist',    author_facet_a (dates stripped?  other formatting?)
            artist_name: 'artist',    author_facet_a (dates stripped?  other formatting?)
            issn:        'issn',      issn_a (comma-separated)
            isbn:        'isbn',      isbn_a (comma-separated)
            oclc:        'oclc',      oclc_a (comma-separated)
            lccn:        'lccn',      lccn_a (comma-separated)
            upc:         'upc',       upc_a (comma-separated)
            ht_id:       'ht-id'      n/a (hathi trust id)
        };
        $.each(data_attribute, function(key, value) {
            var data = $element.attr('data-' + value);
            if (data) { query[key] = data; }
        });

    def link_to_document_from_cover(doc, opt = nil)
      opt = opt ? opt.dup : {}
      opt[:label] = ajax_loader_image
      opt[:class] = "cover-image #{opt[:class]}".strip
      opt[:title]   ||= doc.doc_id
      opt[:counter] ||= -1

      # Provide the link with data items that will be used when requesting the
      # image from the cover image server.
      data_items = {
        'doc-id'   => doc.doc_id,
        'doc-type' => ((doc.doc_type == :lib_album) ? 'music' : 'non_music'),
        'title'    => doc.title,
        'upc'      => doc.upcs,
        'issn'     => doc.issns.join(','),
        'isbn'     => doc.isbns.join(','),
        'oclc'     => doc.oclcs.join(','),
        'lccn'     => doc.lccns.join(','),
        'artist'   => doc.get_authors.first,
        'ht-id'    => doc.values_for(:hathi_id_display).first
      }
      data_items.each_pair do |item, value|
        opt["data-#{item}".to_sym] ||= value if value.present?
      end

      # Wrapping the link in a <div> is required to make the position of the
      # result adjustable.
      content_tag(:div, class: 'simple-thumbnail') do
        link_to_document(doc, opt)
      end
    end


  def generate_doc_type
    # rubocop:disable Metrics/LineLength

    # Extremely specialized views.
    return :hathi          if has?(:source_facet, 'Hathi Trust Digital Library')
    return :kluge          if has?(:doc_type_facet, 'klugeruhe')
    return :dataverse      if has?(:doc_type_facet, 'dataverse')
    return :hsl_tutorial   if has?(:digital_collection_facet, 'Bioconnector Tutorials')
    return :lib_coins      if has?(:format_facet, 'Coin')
    return :dl_image       if has?(:content_model_facet, 'media')
    return :dl_wsls_video  if has?(:content_model_facet, /uva-lib:pbcore2CModel/i)
    return :dl_text        if has?(:content_model_facet, 'text')
    return :lib_technical_report \
                           if has?(:doc_type_facet, 'libra')

    # The "data_driven" partial takes everything left that has any
    # "feature_facet" except those items that are from the "Library Catalog".
    catalog = has?(:source_facet, 'Library Catalog')
    return :data_driven    if !catalog && values_for(:feature_facet).present?

    # Choose the correct partial when ambiguous based on the following order of
    # precedence.
    return :lib_album      if has?(:format_facet, /Sound Recording/i)
    return :lib_video_full if has?(:format_facet, 'Video')
    return :lib_catalog    if catalog || has?(:marc_display_facet, 'true')

    # Otherwise...
    :default

    # rubocop:enable Metrics/LineLength
  end
*/

func (s *searchContext) getCoverImageData(doc *solrDocument) (string, error) {
	type coverImageResponse struct {
		ImageBase64 string      `json:"image_base64,omitempty"`
		NotFound    bool        `json:"not_found,omitempty"`
		Status      string      `json:"status,omitempty"`
		Errors      interface{} `json:"errors,omitempty"`
	}

	var coverRes coverImageResponse

	url := fmt.Sprintf("https://coverimages.lib.virginia.edu/cover_images/%s.json", doc.ID)

	req, reqErr := http.NewRequest("GET", url, nil)
	if reqErr != nil {
		return "", fmt.Errorf("Failed to create cover image request")
	}

	client := &http.Client{Timeout: 3 * time.Second}

	start := time.Now()

	res, resErr := client.Do(req)

	elapsedNanoSec := time.Since(start)
	elapsedMS := int64(elapsedNanoSec / time.Millisecond)

	if resErr != nil {
		return "", fmt.Errorf("Failed to receive cover image response")
	}

	s.log("cover image elapsed time: %d ms", elapsedMS)

	defer res.Body.Close()

	buf, _ := ioutil.ReadAll(res.Body)

	if jErr := json.Unmarshal(buf, &coverRes); jErr != nil {
		return "", fmt.Errorf("Failed to unmarshal cover image response")
	}

	return coverRes.ImageBase64, nil
}
