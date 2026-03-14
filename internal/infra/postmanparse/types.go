package postmanparse

import "encoding/json"

// PostmanCollection represents a Postman Collection v2.1 export.
type PostmanCollection struct {
	Info     PostmanInfo    `json:"info"`
	Item     []PostmanItem  `json:"item"`
	Variable []PostmanKV    `json:"variable"`
	Event    []PostmanEvent `json:"event"`
}

// PostmanInfo holds collection metadata.
type PostmanInfo struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
}

// PostmanItem is a request or folder in the collection.
type PostmanItem struct {
	Name    string          `json:"name"`
	Request *PostmanRequest `json:"request"`
	Item    []PostmanItem   `json:"item"` // nested folders
	Event   []PostmanEvent  `json:"event"`
}

// PostmanRequest describes an HTTP request.
type PostmanRequest struct {
	Method string       `json:"method"`
	URL    PostmanURL   `json:"url"`
	Header []PostmanKV  `json:"header"`
	Body   *PostmanBody `json:"body"`
	Auth   *PostmanAuth `json:"auth"`
}

// PostmanURL can be a string or an object in Postman exports.
type PostmanURL struct {
	Raw   string      `json:"raw"`
	Query []PostmanKV `json:"query"`
}

// UnmarshalJSON handles the case where url is a plain string.
func (u *PostmanURL) UnmarshalJSON(data []byte) error {
	// Try string first.
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		u.Raw = s
		return nil
	}

	// Object form.
	type alias PostmanURL
	var obj alias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*u = PostmanURL(obj)
	return nil
}

// PostmanKV is a generic key-value pair used for headers, variables, etc.
type PostmanKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PostmanBody describes the request body.
type PostmanBody struct {
	Mode       string              `json:"mode"`
	Raw        string              `json:"raw"`
	Options    *PostmanBodyOptions `json:"options"`
	URLEncoded []PostmanKV         `json:"urlencoded"`
	FormData   []PostmanKV         `json:"formdata"`
}

// PostmanBodyOptions holds body mode options (e.g., raw language).
type PostmanBodyOptions struct {
	Raw struct {
		Language string `json:"language"`
	} `json:"raw"`
}

// PostmanAuth describes authentication configuration.
type PostmanAuth struct {
	Type string `json:"type"`
}

// PostmanEvent represents pre-request or test scripts.
type PostmanEvent struct {
	Listen string `json:"listen"`
}
