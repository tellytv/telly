package xmltv

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/kr/pretty"
)

func dummyReader(charset string, input io.Reader) (io.Reader, error) {
	return input, nil
}

func TestDecode(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Example downloaded from http://wiki.xmltv.org/index.php/internal/xmltvFormat
	// One may check it with `xmllint --noout --dtdvalid xmltv.dtd example.xml`
	f, err := os.Open(fmt.Sprintf("%s/example.xml", dir))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var tv TV
	dec := xml.NewDecoder(f)
	dec.CharsetReader = dummyReader
	err = dec.Decode(&tv)
	if err != nil {
		t.Fatal(err)
	}

	ch := Channel{
		ID: "I10436.labs.zap2it.com",
		DisplayNames: []CommonElement{
			CommonElement{
				Value: "13 KERA",
			},
			CommonElement{
				Value: "13 KERA TX42822:-",
			},
			CommonElement{
				Value: "13",
			},
			CommonElement{
				Value: "13 KERA fcc",
			},
			CommonElement{
				Value: "KERA",
			},
			CommonElement{
				Value: "KERA",
			},
			CommonElement{
				Value: "PBS Affiliate",
			},
		},
		Icons: []Icon{
			Icon{
				Source: `file://C:\Perl\site/share/xmltv/icons/KERA.gif`,
			},
		},
	}
	if !reflect.DeepEqual(ch, tv.Channels[0]) {
		t.Errorf("\texpected: %# v\n\t\tactual:   %# v\n", pretty.Formatter(ch), pretty.Formatter(tv.Channels[0]))
	}

	loc := time.FixedZone("", -6*60*60)
	date := time.Date(2008, 07, 11, 0, 0, 0, 0, time.UTC)
	pr := Programme{
		ID:      "someId",
		Date:    Date(date),
		Channel: "I10436.labs.zap2it.com",
		Start:   &Time{time.Date(2008, 07, 15, 0, 30, 0, 0, loc)},
		Stop:    &Time{time.Date(2008, 07, 15, 1, 0, 0, 0, loc)},
		Titles: []CommonElement{
			CommonElement{
				Lang:  "en",
				Value: "NOW on PBS",
			},
		},
		Descriptions: []CommonElement{
			CommonElement{
				Lang:  "en",
				Value: "Jordan's Queen Rania has made job creation a priority to help curb the staggering unemployment rates among youths in the Middle East.",
			},
		},
		Categories: []CommonElement{
			CommonElement{
				Lang:  "en",
				Value: "Newsmagazine",
			},
			CommonElement{
				Lang:  "en",
				Value: "Interview",
			},
			CommonElement{
				Lang:  "en",
				Value: "Public affairs",
			},
			CommonElement{
				Lang:  "en",
				Value: "Series",
			},
		},
		EpisodeNums: []EpisodeNum{
			EpisodeNum{
				System: "dd_progid",
				Value:  "EP01006886.0028",
			},
			EpisodeNum{
				System: "onscreen",
				Value:  "427",
			},
		},
		Audio: &Audio{
			Stereo: "stereo",
		},
		PreviouslyShown: &PreviouslyShown{
			Start: "20080711000000",
		},
		Subtitles: []Subtitle{
			Subtitle{
				Type: "teletext",
			},
		},
	}
	if !reflect.DeepEqual(pr, tv.Programmes[0]) {
		expected := fmt.Sprintf("\texpected: %# v\n\t\t\texpected start: %s\n\t\t\texpected stop : %s", pretty.Formatter(pr), pr.Start, pr.Stop)
		actual := fmt.Sprintf("\tactual:   %# v\n\t\t\tactual start:   %s\n\t\t\tactual stop:    %s", pretty.Formatter(tv.Programmes[0]), tv.Programmes[0].Start, tv.Programmes[0].Stop)
		t.Errorf("%s\n%s\n", expected, actual)
	}
}
