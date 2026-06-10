// metrics/metrics_endpoints.go

package metrics

var Endpoints = map[string]EndpointConfig{
	"/check": {
		Method: "POST",
		Speed:  FAST,
	},
	"/close": {
		Method: "POST",
		Speed:  FAST,
	},
	"/bd": {
		Method: "GET",
		Speed:  MIDDLE,
	},
	"/list": {
		Method: "GET",
		Speed:  MIDDLE,
	},
	"/login": {
		Method: "POST",
		Speed:  SLOW,
	},
}
