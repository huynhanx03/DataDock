package sqlite

import "time"

const timeFormat = time.RFC3339Nano

func parseTime(value string) (time.Time, error) {
	return time.Parse(timeFormat, value)
}
