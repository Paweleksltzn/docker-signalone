package models

type PredictedSolutionSource struct {
	Id             string    `json:"id"`
	Url            string    `json:"url"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	FeaturedAnswer string    `json:"featuredAnswer"`
	Vector         []float32 `json:"vector"`
}
