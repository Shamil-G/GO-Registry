package metrics

type SpeedClass int

const (
	FAST SpeedClass = iota
	MIDDLE
	SLOW
)

type EndpointConfig struct {
	Method string
	Speed  SpeedClass
}
