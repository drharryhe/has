package hrandom

import (
	"math"
	"math/rand"
	"time"
)

func Number(len int) uint {
	if len < 1 {
		return 0
	}
	rand.Seed(time.Now().Unix() * int64(rand.Int()))
	max := math.Pow(10, float64(len)) - 1
	min := math.Pow(10, float64(len-1))
	return uint(int(min) + rand.Intn(int(max-min)))
}
