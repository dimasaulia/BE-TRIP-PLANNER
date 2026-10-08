package controllers

import "time"

func timeIn(seconds int) time.Time {
	return time.Now().Add(time.Duration(seconds) * time.Second)
}
