package metrics

import (
	"database/sql"
	"time"
)

func StartDBPoolMetrics(db *sql.DB) {
	go func() {
		for {
			stats := db.Stats()

			DBPoolOpen.Set(float64(stats.OpenConnections))
			DBPoolInUse.Set(float64(stats.InUse))
			DBPoolIdle.Set(float64(stats.Idle))

			time.Sleep(5 * time.Second)
		}
	}()
}
