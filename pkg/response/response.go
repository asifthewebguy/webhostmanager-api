package response

// Envelope is the standard JSON response envelope.
type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta holds optional pagination metadata.
type Meta struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	TotalItems int `json:"total_items,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

// OK returns a successful response envelope.
func OK(data interface{}) Envelope {
	return Envelope{Success: true, Data: data}
}

// OKWithMeta returns a successful response envelope with pagination metadata.
func OKWithMeta(data interface{}, meta *Meta) Envelope {
	return Envelope{Success: true, Data: data, Meta: meta}
}

// Error returns an error response envelope.
func Error(msg string) Envelope {
	return Envelope{Success: false, Error: msg}
}
