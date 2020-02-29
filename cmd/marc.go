package main

import (
	"encoding/xml"
	"log"
)

type marcSubField struct {
	XMLName xml.Name `xml:"subfield"`
	Code    string   `xml:"code,attr"`
	Value   string   `xml:",chardata"`
}

type marcDataField struct {
	XMLName   xml.Name       `xml:"datafield"`
	Tag       string         `xml:"tag,attr"`
	SubFields []marcSubField `xml:"subfield"`
}

type marcRecord struct {
	XMLName    xml.Name        `xml:"record"`
	DataFields []marcDataField `xml:"datafield"`
}

type marcCollection struct {
	XMLName xml.Name     `xml:"collection"`
	Record  []marcRecord `xml:"record"`
}

func getSubFieldCodeForDataFieldTagAndSubfieldCode(marcXML string, wantCode string, tag string, code string) (map[string]string, error) {
	marc := marcCollection{}

	if err := xml.Unmarshal([]byte(marcXML), &marc); err != nil {
		log.Printf("MARC XML parsing error: %v", err)
		return nil, err
	}

	fieldMap := make(map[string]string)

	for _, r := range marc.Record {
		for _, df := range r.DataFields {
			wantValue := ""
			value := ""

			if df.Tag != tag {
				continue
			}

			for _, sf := range df.SubFields {
				switch sf.Code {
				case wantCode:
					if wantValue == "" {
						wantValue = sf.Value
					}
				case code:
					if value == "" {
						value = sf.Value
					}
				}
			}

			if value != "" {
				fieldMap[value] = wantValue
			}
		}
	}

	log.Printf("MARC XML field map: %v", fieldMap)

	return fieldMap, nil
}
