package helpers

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

// 短网址生成方法 这个方法会,生成四个短字符串,每一个字符串的长度为6
// 这个方法是从网上搜索的一个方法,但不知道出自何处了,稍微将key换了一下
// @param url
// @return
func short(str string) []string {
	// 要使用生成 URL 的字符
	chars := []string{
		"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l",
		"m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x",
		"y", "z", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
		"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L",
		"M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X",
		"Y", "Z",
	}

	// 对传入网址进行 MD5 加密
	md5Hash := md5.Sum([]byte(str))
	hex := fmt.Sprintf("%x", md5Hash)

	results := make([]string, 4)

	for i := 0; i < 4; i++ {
		// 把加密字符按照 8 位一组 16 进制与 0x3FFFFFFF 进行位与运算
		sTempSubString := hex[i*8 : i*8+8]

		// 这里需要使用 long 型来转换，因为 Inteper .parseInt() 只能处理 31 位 , 首位为符号位 , 如果不用
		// long ，则会越界
		lHex, _ := strconv.ParseInt(sTempSubString, 16, 64)

		char := ""

		for j := 0; j < 6; j++ {
			// 把得到的值与 0x0000003D 进行位与运算，取得字符数组 chars 索引
			index := int(lHex & 0x0000003D)

			// 把取得的字符相加
			char += chars[index]

			// 每次循环按位右移 5 位
			lHex >>= 5
		}
		// 把字符串存入对应索引的输出数组
		results[i] = char
	}

	return results
}

func Short(str string) string {
	results := short(str)

	return results[0] + results[1]
}

func ShortUnique(str string) string {
	// 可以自定义生成 MD5 加密字符传前的混合 KEY
	slat := strconv.FormatInt(time.Now().UnixNano(), 10) + strconv.Itoa(rand.Intn(100000000))
	//混淆key,加上当前时间,并且取一个随机字符串
	results := short(slat + str)

	return results[0] + results[1]
}
