package models

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
)

type ConvertibleBoolean bool

func (bit *ConvertibleBoolean) MarshalJSON() ([]byte, error) {
	var bitSetVar int8
	if *bit {
		bitSetVar = 1
	}

	return json.Marshal(bitSetVar)
}

func (bit *ConvertibleBoolean) UnmarshalJSON(data []byte) error {
	asString := string(data)
	if asString == "1" || asString == "true" {
		*bit = true
	} else if asString == "0" || asString == "false" {
		*bit = false
	} else {
		return fmt.Errorf("Boolean unmarshal error: invalid input %s", asString)
	}
	return nil
}

// MarshalXML used to determine if the element is present or not. see https://stackoverflow.com/a/46516243
func (bit *ConvertibleBoolean) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	var bitSetVar int8
	if *bit {
		bitSetVar = 1
	}

	return e.EncodeElement(bitSetVar, start)
}

// UnmarshalXML used to determine if the element is present or not. see https://stackoverflow.com/a/46516243
func (bit *ConvertibleBoolean) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var asString string
	if decodeErr := d.DecodeElement(&asString, &start); decodeErr != nil {
		return decodeErr
	}
	if asString == "1" || asString == "true" {
		*bit = true
	} else if asString == "0" || asString == "false" {
		*bit = false
	} else {
		return fmt.Errorf("Boolean unmarshal error: invalid input %s", asString)
	}
	return nil
}
