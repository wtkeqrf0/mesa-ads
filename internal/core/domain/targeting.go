package domain

// Targeting describes who should see a campaign
type Targeting struct {
	Languages  []string `json:"languages"`
	Geos       []string `json:"geos"`
	Categories []string `json:"categories"`
	Interests  []string `json:"interests"`
	Placements []string `json:"placements"`
}
