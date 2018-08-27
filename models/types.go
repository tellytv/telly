package models

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
)

// ConvertibleBoolean is a helper type to allow JSON documents using 0/1 or "true" and "false" be converted to bool.
type ConvertibleBoolean bool

// MarshalJSON returns a 0 or 1 depending on bool state.
func (bit *ConvertibleBoolean) MarshalJSON() ([]byte, error) {
	var bitSetVar int8
	if *bit {
		bitSetVar = 1
	}

	return json.Marshal(bitSetVar)
}

// UnmarshalJSON converts a 0, 1, true or false into a bool
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
