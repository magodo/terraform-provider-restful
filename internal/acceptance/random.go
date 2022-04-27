package acceptance

import (
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type Rd int

// Grab from https//github.com/hashicorp/terraform-provider-azurerm
func NewRd() Rd {
	// acctest.RantInt() returns a value of size:
	// 000000000000000000
	// YYMMddHHmmsshhRRRR

	// go format: 2006-01-02 15:04:05.00

	timeStr := strings.Replace(time.Now().Local().Format("060102150405.00"), ".", "", 1) // no way to not have a .?
	postfix := randStringFromCharSet(4, "0123456789")

	i, err := strconv.Atoi(timeStr + postfix)
	if err != nil {
		panic(err)
	}

	return Rd(i)
}

func randStringFromCharSet(strlen int, charSet string) string {
	result := make([]byte, strlen)
	for i := 0; i < strlen; i++ {
		result[i] = charSet[randIntRange(0, len(charSet))]
	}
	return string(result)
}

func randIntRange(min int, max int) int {
	return rand.Intn(max-min) + min
}

// RandomIntOfLength is a random 8 to 18 digit integer which is unique to this test case
func (rd Rd) RandomIntOfLength(len int) int {
	// len should not be
	//  - greater then 18, longest a int can represent
	//  - less then 8, as that gives us YYMMDDRR
	if 8 > len || len > 18 {
		panic("Invalid Test: RandomIntOfLength: len is not between 8 or 18 inclusive")
	}

	// 18 - just return the int
	if len >= 18 {
		return int(rd)
	}

	// 16-17 just strip off the last 1-2 digits
	if len >= 16 {
		return int(rd) / int(math.Pow10(18-len))
	}

	// 8-15 keep len - 2 digits and add 2 characters of randomness on
	s := strconv.Itoa(int(rd))
	r := s[16:18]
	v := s[0 : len-2]
	i, _ := strconv.Atoi(v + r)

	return i
}
