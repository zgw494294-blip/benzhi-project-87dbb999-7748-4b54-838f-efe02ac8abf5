package storage

import "time"

func parseTime(value string) (time.Time, error) { return time.Parse(timeFormat, value) }
