package num

import (
	"github.com/pkg/errors"
	"github.com/shawnwyckoff/gpkg/dsa/stringz"
	"math/big"
	"strconv"
	"strings"
)

func ParseFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// 123
// 00123
// +123
// -123
// 123.456
// 2.07539829195e-05
func IsDigit(s string) bool {
	_, _, err := big.ParseFloat(s, 10, 1, big.ToNearestEven)
	//_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func ParseBigInt(num string, base int) (big.Int, error) {
	if base == 16 {
		lowernum := strings.ToLower(num)
		if stringz.StartWith(lowernum, "0x") {
			num = num[2:]
		}
	}

	var bi big.Int
	result, success := bi.SetString(num, base)
	if success {
		return *result, nil
	} else {
		return *big.NewInt(0), errors.New("Parse error")
	}
}
