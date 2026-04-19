package domain

type DocType string

const (
	PublicDoc   DocType = "public"
	InternalDoc DocType = "internal"
)


 
// DocMetadata is read from <slug>.json alongside the markdown file.
type DocMetadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	LastUpdated string   `json:"lastUpdated"`
	Tags        []string `json:"tags"`
}
 
// DocResponse is the structured payload returned to the frontend.
// Serialises as: { "metadata": { ... }, "content": "..." }
// The HTTP handler wraps this in the standard envelope: { "data": <DocResponse> }
type DocResponse struct {
	Metadata DocMetadata `json:"metadata"`
	Content  string      `json:"content"`
}
 