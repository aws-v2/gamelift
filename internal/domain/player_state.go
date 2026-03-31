package domain

type PlayerState struct {
	ID        string  `json:"id"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Z         float64 `json:"z"`
	Yaw       float64 `json:"yaw"`
	Pitch     float64 `json:"pitch"`
	VX        float64 `json:"vx"`
	VY        float64 `json:"vy"`
	VZ        float64 `json:"vz"`
	Anim      string  `json:"anim"`
	IsOnFloor bool    `json:"is_on_floor"`
	HP        int     `json:"health"`
}
