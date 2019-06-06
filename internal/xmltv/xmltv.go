// Package xmltv provides structures for parsing XMLTV data.
package xmltv

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

// Time that holds the time which is parsed from XML
type Time struct {
	time.Time
}

// MarshalXMLAttr is used to marshal a Go time.Time into the XMLTV Format.
func (t *Time) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {
	return xml.Attr{
		Name:  name,
		Value: t.Format("20060102150405 -0700"),
	}, nil
}

// UnmarshalXMLAttr is used to unmarshal a time in the XMLTV format to a time.Time.
func (t *Time) UnmarshalXMLAttr(attr xml.Attr) error {
	fmtStr := "20060102150405"
	if strings.Contains(attr.Value, " ") {
		fmtStr = "20060102150405 -0700"
	}
	t1, err := time.Parse(fmtStr, attr.Value)
	if err != nil {
		return err
	}

	*t = Time{t1}
	return nil
}

// Date is the XMLTV specific formatting of a date (YYYYMMDD/20060102)
type Date time.Time

// MarshalXML is used to marshal a Go time.Time into the XMLTV Date Format.
func (p Date) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	t := time.Time(p)
	if t.IsZero() {
		return e.EncodeElement(nil, start)
	}
	return e.EncodeElement(t.Format("20060102"), start)
}

// UnmarshalXML is used to unmarshal a time in the XMLTV Date format to a time.Time.
func (p *Date) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	var content string
	if e := d.DecodeElement(&content, &start); e != nil {
		return fmt.Errorf("get the type Date field of %s error", start.Name.Local)
	}

	dateFormat := "20060102"

	if len(content) == 4 {
		dateFormat = "2006"
	}

	if strings.Contains(content, "|") {
		content = strings.Split(content, "|")[0]
		dateFormat = "2006"
	}

	v, e := time.Parse(dateFormat, content)
	if e != nil {
		return fmt.Errorf("the type Date field of %s is not a time, value is: %s", start.Name.Local, content)
	}
	*p = Date(v)
	return nil
}

// MarshalJSON is used to marshal a Go time.Time into the XMLTV Date Format.
func (p Date) MarshalJSON() ([]byte, error) {
	t := time.Time(p)
	str := "\"" + t.Format("20060102") + "\""

	return []byte(str), nil
}

// UnmarshalJSON is used to unmarshal a time in the XMLTV Date format to a time.Time.
func (p *Date) UnmarshalJSON(text []byte) (err error) {
	strDate := string(text[1 : 8+1])

	v, e := time.Parse("20060102", strDate)
	if e != nil {
		return fmt.Errorf("Date should be a time, error value is: %s", strDate)
	}
	*p = Date(v)
	return nil
}

// TV is the root element.
type TV struct {
	XMLName           xml.Name    `xml:"tv"                                 json:"-"                             db:"-"`
	Channels          []Channel   `xml:"channel"                            json:"channels"                      db:"channels"`
	Programmes        []Programme `xml:"programme"                          json:"programmes"                    db:"programmes"`
	Date              string      `xml:"date,attr,omitempty"                json:"date,omitempty"                db:"date,omitempty"`
	SourceInfoURL     string      `xml:"source-info-url,attr,omitempty"     json:"sourceInfoURL,omitempty"       db:"source_info_url,omitempty"`
	SourceInfoName    string      `xml:"source-info-name,attr,omitempty"    json:"sourceInfoName,omitempty"      db:"source_info_name,omitempty"`
	SourceDataURL     string      `xml:"source-data-url,attr,omitempty"     json:"sourceDataURL,omitempty"       db:"source_data_url,omitempty"`
	GeneratorInfoName string      `xml:"generator-info-name,attr,omitempty" json:"generatorInfoName,omitempty"   db:"generator_info_name,omitempty"`
	GeneratorInfoURL  string      `xml:"generator-info-url,attr,omitempty"  json:"generatorInfoURL,omitempty"    db:"generator_info_url,omitempty"`
}

// LoadXML loads the XMLTV XML from file.
func (t *TV) LoadXML(f io.Reader) error {
	decoder := xml.NewDecoder(f)
	decoder.CharsetReader = charset.NewReaderLabel

	err := decoder.Decode(&t)
	return err
}

// Channel details of a channel
type Channel struct {
	XMLName      xml.Name        `xml:"channel"        json:"-"               db:"-"`
	DisplayNames []CommonElement `xml:"display-name"   json:"displayNames"    db:"display_names"`
	Icons        []Icon          `xml:"icon,omitempty" json:"icons,omitempty" db:"icons,omitempty"`
	URLs         []string        `xml:"url,omitempty"  json:"urls,omitempty"  db:"urls,omitempty"`
	ID           string          `xml:"id,attr"        json:"id,omitempty"    db:"id,omitempty"`
	LCN          string          `xml:"lcn"            json:"lcn,omitempty"   db:"lcn,omitempty"` // LCN is the local channel number. Plex will show it in place of the channel ID if it exists.
}

// Programme details of a single programme transmission
type Programme struct {
	XMLName         xml.Name         `xml:"programme"                  json:"-"                          db:"-"`
	ID              string           `xml:"id,attr,omitempty"          json:"id,omitempty"               db:"id,omitempty"` // not defined by standard, but often present
	Titles          []CommonElement  `xml:"title"                      json:"titles"                     db:"titles"`
	SecondaryTitles []CommonElement  `xml:"sub-title,omitempty"        json:"secondaryTitles,omitempty"  db:"secondary_titles,omitempty"`
	Descriptions    []CommonElement  `xml:"desc,omitempty"             json:"descriptions,omitempty"     db:"descriptions,omitempty"`
	Credits         *Credits         `xml:"credits,omitempty"          json:"credits,omitempty"          db:"credits,omitempty"`
	Date            Date             `xml:"date,omitempty"             json:"date,omitempty"             db:"date,omitempty"`
	Categories      []CommonElement  `xml:"category,omitempty"         json:"categories,omitempty"       db:"categories,omitempty"`
	Keywords        []CommonElement  `xml:"keyword,omitempty"          json:"keywords,omitempty"         db:"keywords,omitempty"`
	Languages       []CommonElement  `xml:"language,omitempty"         json:"languages,omitempty"        db:"languages,omitempty"`
	OrigLanguages   []CommonElement  `xml:"orig-language,omitempty"    json:"origLanguages,omitempty"    db:"orig_languages,omitempty"`
	Length          *Length          `xml:"length,omitempty"           json:"length,omitempty"           db:"length,omitempty"`
	Icons           []Icon           `xml:"icon,omitempty"             json:"icons,omitempty"            db:"icons,omitempty"`
	URLs            []string         `xml:"url,omitempty"              json:"urls,omitempty"             db:"urls,omitempty"`
	Countries       []CommonElement  `xml:"country,omitempty"          json:"countries,omitempty"        db:"countries,omitempty"`
	EpisodeNums     []EpisodeNum     `xml:"episode-num,omitempty"      json:"episodeNums,omitempty"      db:"episode_nums,omitempty"`
	Video           *Video           `xml:"video,omitempty"            json:"video,omitempty"            db:"video,omitempty"`
	Audio           *Audio           `xml:"audio,omitempty"            json:"audio,omitempty"            db:"audio,omitempty"`
	PreviouslyShown *PreviouslyShown `xml:"previously-shown,omitempty" json:"previouslyShown,omitempty"  db:"previously_shown,omitempty"`
	Premiere        *CommonElement   `xml:"premiere,omitempty"         json:"premiere,omitempty"         db:"premiere,omitempty"`
	LastChance      *CommonElement   `xml:"last-chance,omitempty"      json:"lastChance,omitempty"       db:"last_chance,omitempty"`
	New             *ElementPresent  `xml:"new"                        json:"new,omitempty"              db:"new,omitempty"`
	Subtitles       []Subtitle       `xml:"subtitles,omitempty"        json:"subtitles,omitempty"        db:"subtitles,omitempty"`
	Ratings         []Rating         `xml:"rating,omitempty"           json:"ratings,omitempty"          db:"ratings,omitempty"`
	StarRatings     []Rating         `xml:"star-rating,omitempty"      json:"starRatings,omitempty"      db:"star_ratings,omitempty"`
	Reviews         []Review         `xml:"review,omitempty"           json:"reviews,omitempty"          db:"reviews,omitempty"`
	Start           *Time            `xml:"start,attr"                 json:"start"                      db:"start"`
	Stop            *Time            `xml:"stop,attr,omitempty"        json:"stop,omitempty"             db:"stop,omitempty"`
	PDCStart        *Time            `xml:"pdc-start,attr,omitempty"   json:"pdcStart,omitempty"         db:"pdc_start,omitempty"`
	VPSStart        *Time            `xml:"vps-start,attr,omitempty"   json:"vpsStart,omitempty"         db:"vps_start,omitempty"`
	Showview        string           `xml:"showview,attr,omitempty"    json:"showview,omitempty"         db:"showview,omitempty"`
	Videoplus       string           `xml:"videoplus,attr,omitempty"   json:"videoplus,omitempty"        db:"videoplus,omitempty"`
	Channel         string           `xml:"channel,attr"               json:"channel"                    db:"channel"`
	Clumpidx        string           `xml:"clumpidx,attr,omitempty"    json:"clumpidx,omitempty"         db:"clumpidx,omitempty"`
}

// CommonElement element structure that is common, i.e. <country lang="en">Italy</country>
type CommonElement struct {
	Lang  string `xml:"lang,attr,omitempty" json:"lang,omitempty"  db:"lang,omitempty" `
	Value string `xml:",chardata"           json:"value,omitempty" db:"value,omitempty"`
}

// ElementPresent used to determine if element is present or not
type ElementPresent bool

// MarshalXML used to determine if the element is present or not. see https://stackoverflow.com/a/46516243
func (c *ElementPresent) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if c == nil {
		return e.EncodeElement(nil, start)
	}
	return e.EncodeElement("", start)
}

// UnmarshalXML used to determine if the element is present or not. see https://stackoverflow.com/a/46516243
func (c *ElementPresent) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var v string
	if decodeErr := d.DecodeElement(&v, &start); decodeErr != nil {
		return decodeErr
	}
	*c = true
	return nil
}

// Icon associated with the element that contains it
type Icon struct {
	Source string `xml:"src,attr"              json:"source"           db:"source"`
	Width  int    `xml:"width,attr,omitempty"  json:"width,omitempty"  db:"width,omitempty"`
	Height int    `xml:"height,attr,omitempty" json:"height,omitempty" db:"height,omitempty"`
}

// Credits for the programme
type Credits struct {
	Directors    []string `xml:"director,omitempty"    json:"directors,omitempty"    db:"directors,omitempty"`
	Actors       []Actor  `xml:"actor,omitempty"       json:"actors,omitempty"       db:"actors,omitempty"`
	Writers      []string `xml:"writer,omitempty"      json:"writers,omitempty"      db:"writers,omitempty"`
	Adapters     []string `xml:"adapter,omitempty"     json:"adapters,omitempty"     db:"adapters,omitempty"`
	Producers    []string `xml:"producer,omitempty"    json:"producers,omitempty"    db:"producers,omitempty"`
	Composers    []string `xml:"composer,omitempty"    json:"composers,omitempty"    db:"composers,omitempty"`
	Editors      []string `xml:"editor,omitempty"      json:"editors,omitempty"      db:"editors,omitempty"`
	Presenters   []string `xml:"presenter,omitempty"   json:"presenters,omitempty"   db:"presenters,omitempty"`
	Commentators []string `xml:"commentator,omitempty" json:"commentators,omitempty" db:"commentators,omitempty"`
	Guests       []string `xml:"guest,omitempty"       json:"guests,omitempty"       db:"guests,omitempty"`
}

// Actor in a programme
type Actor struct {
	Role  string `xml:"role,attr,omitempty" json:"role,omitempty" db:"role,omitempty"`
	Value string `xml:",chardata"           json:"value"          db:"value"`
}

// Length of the programme
type Length struct {
	Units string `xml:"units,attr" json:"units" db:"units"`
	Value string `xml:",chardata"  json:"value" db:"value"`
}

// EpisodeNum of the programme
type EpisodeNum struct {
	System string `xml:"system,attr,omitempty" json:"system,omitempty" db:"system,omitempty"`
	Value  string `xml:",chardata"             json:"value"            db:"value"`
}

// Video details of the programme
type Video struct {
	Present string `xml:"present,omitempty" json:"present,omitempty" db:"present,omitempty"`
	Colour  string `xml:"colour,omitempty"  json:"colour,omitempty"  db:"colour,omitempty"`
	Aspect  string `xml:"aspect,omitempty"  json:"aspect,omitempty"  db:"aspect,omitempty"`
	Quality string `xml:"quality,omitempty" json:"quality,omitempty" db:"quality,omitempty"`
}

// Audio details of the programme
type Audio struct {
	Present string `xml:"present,omitempty" json:"present,omitempty" db:"present,omitempty"`
	Stereo  string `xml:"stereo,omitempty"  json:"stereo,omitempty"  db:"stereo,omitempty"`
}

// PreviouslyShown When and where the programme was last shown, if known.
type PreviouslyShown struct {
	Start   Time   `xml:"start,attr,omitempty"   json:"start,omitempty"   db:"start,omitempty"`
	Channel string `xml:"channel,attr,omitempty" json:"channel,omitempty" db:"channel,omitempty"`
}

// Subtitle in a programme
type Subtitle struct {
	Language *CommonElement `xml:"language,omitempty"  json:"language,omitempty" db:"language,omitempty"`
	Type     string         `xml:"type,attr,omitempty" json:"type,omitempty"     db:"type,omitempty"`
}

// Rating of a programme
type Rating struct {
	Value  string `xml:"value"                 json:"value"            db:"value"`
	Icons  []Icon `xml:"icon,omitempty"        json:"icons,omitempty"  db:"icons,omitempty"`
	System string `xml:"system,attr,omitempty" json:"system,omitempty" db:"system,omitempty"`
}

// Review of a programme
type Review struct {
	Value    string `xml:",chardata"          json:"value"              db:"value"`
	Type     string `xml:"type"               json:"type"               db:"type"`
	Source   string `xml:"source,omitempty"   json:"source,omitempty"   db:"source,omitempty"`
	Reviewer string `xml:"reviewer,omitempty" json:"reviewer,omitempty" db:"reviewer,omitempty"`
	Lang     string `xml:"lang,omitempty"     json:"lang,omitempty"     db:"lang,omitempty"`
}
