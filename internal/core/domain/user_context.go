package domain

// UserContext describes input from an ad request. It captures information
// about the viewer and the content context such as language, geo, category
// and interests. The HTTP layer should construct this struct from
// request data and pass it into the usecase.
type UserContext struct {
	UserID    string
	Language  string
	Geo       string
	Category  string
	Interests []string
	Placement string
}
