package security

import "strings"

// commonPasswords 内置常见弱密码列表（来源：多份公开泄露密码排行榜 Top 1000+）
// 用于离线兜底：当 HIBP API 不可用时仍能拦截最常见的弱密码
var commonPasswords = func() map[string]struct{} {
	list := []string{
		// ---- 数字类 ----
		"123456", "123456789", "12345678", "1234567", "1234567890",
		"12345", "123123", "1234", "123459", "111111", "000000",
		"121212", "666666", "888888", "159753", "987654321",
		"0123456789", "13579", "246810", "11111111", "222222",
		"333333", "444444", "555555", "777777", "999999",
		"112233", "131313", "100100", "654321", "696969",
		"101010", "987654", "54321", "147258", "147852",
		"258456", "159357", "741852", "963852", "741963",
		"369258", "258963", "147258369", "951753", "357159",
		"102030", "505050", "606060", "707070", "808080",
		"909090", "100200", "110110", "120120", "040404",
		"010101", "020202", "030303", "050505", "060606",
		"070707", "080808", "090909", "123321", "456789",
		"456123", "789456", "789123", "159951", "753951",
		"142536", "111222", "222333", "333444", "444555",
		"555666", "666777", "777888", "888999", "999000",
		"000001", "000002", "000003", "000004", "000005",
		"000006", "000007", "000008", "000009", "000010",
		// ---- 字母类 ----
		"password", "Password", "PASSWORD", "Password1", "password1",
		"pass", "Pass", "admin", "Admin", "ADMIN",
		"administrator", "Administrator", "root", "Root", "ROOT",
		"qwerty", "Qwerty", "QWERTY", "qwerty123", "Qwerty123",
		"abc123", "Abc123", "ABC123", "abc", "ABC",
		"abcdef", "Abcdef", "ABCDEF", "abcdefg", "abcdef123",
		"test", "Test", "TEST", "test123", "Test123",
		"guest", "Guest", "GUEST", "guest123", "Guest123",
		"user", "User", "USER", "user123", "User123",
		"login", "Login", "LOGIN", "login123", "Login123",
		"welcome", "Welcome", "WELCOME", "welcome1", "Welcome1",
		"letmein", "Letmein", "LETMEIN", "letmein123",
		"monkey", "Monkey", "MONKEY", "monkey123",
		"dragon", "Dragon", "DRAGON", "dragon123",
		"master", "Master", "MASTER", "master123",
		"shadow", "Shadow", "SHADOW", "shadow123",
		"sunshine", "Sunshine", "SUNSHINE", "sunshine1",
		"princess", "Princess", "PRINCESS", "princess1",
		"football", "Football", "FOOTBALL", "football1",
		"charlie", "Charlie", "CHARLIE", "charlie1",
		"iloveyou", "Iloveyou", "ILOVEYOU", "iloveyou1",
		"trustno1", "Trustno1", "TRUSTNO1",
		"superman", "Superman", "SUPERMAN", "superman1",
		"batman", "Batman", "BATMAN", "batman1",
		"access", "Access", "ACCESS", "access14",
		"flower", "Flower", "FLOWER", "flower123",
		"summer", "Summer", "SUMMER", "summer123",
		"winter", "Winter", "WINTER", "winter123",
		"baseball", "Baseball", "BASEBALL", "baseball1",
		"michael", "Michael", "MICHAEL", "michael1",
		"jordan", "Jordan", "JORDAN", "jordan23",
		"harley", "Harley", "HARLEY", "harley1",
		"robert", "Robert", "ROBERT", "robert1",
		"thomas", "Thomas", "THOMAS", "thomas1",
		"hockey", "Hockey", "HOCKEY", "hockey1",
		"ranger", "Ranger", "RANGER", "ranger1",
		"george", "George", "GEORGE", "george1",
		"computer", "Computer", "COMPUTER", "computer1",
		"sexy", "Sexy", "SEXY", "sexy123",
		"ashley", "Ashley", "ASHLEY", "ashley1",
		"thunder", "Thunder", "THUNDER", "thunder1",
		"ginger", "Ginger", "GINGER", "ginger1",
		"hammer", "Hammer", "HAMMER", "hammer1",
		"silver", "Silver", "SILVER", "silver1",
		"internet", "Internet", "INTERNET", "internet1",
		"whatever", "Whatever", "WHATEVER", "whatever1",
		"nicole", "Nicole", "NICOLE", "nicole1",
		"jennifer", "Jennifer", "JENNIFER", "jennifer1",
		"starwars", "Starwars", "STARWARS", "starwars1",
		"pepper", "Pepper", "PEPPER", "pepper1",
		"matrix", "Matrix", "MATRIX", "matrix1",
		"maverick", "Maverick", "MAVERICK", "maverick1",
		"hello", "Hello", "HELLO", "hello123",
		"freedom", "Freedom", "FREEDOM", "freedom1",
		"love", "Love", "LOVE", "love123",
		"sex", "Sex", "SEX", "sex123",
		"secret", "Secret", "SECRET", "secret1",
		"money", "Money", "MONEY", "money123",
		"hunter", "Hunter", "HUNTER", "hunter2",
		"joshua", "Joshua", "JOSHUA", "joshua1",
		"amanda", "Amanda", "AMANDA", "amanda1",
		"jessica", "Jessica", "JESSICA", "jessica1",
		"passw0rd", "P@ssw0rd", "P@ssword", "P@ssw0rd1",
		"passw0rd!", "P@ssword1", "P@$$w0rd", "P@ssw0rd!",
		// ---- 混合类 ----
		"abc12345", "Abc12345", "abc123456", "Abc123456",
		"test1234", "Test1234", "test12345", "Test12345",
		"admin123", "Admin123", "admin1234", "Admin1234",
		"root123", "Root123", "root1234", "Root1234",
		"user1234", "User1234", "guest1234", "Guest1234",
		"pass123", "Pass123", "pass1234", "Pass1234",
		"pass12345", "Pass12345", "password12", "Password12",
		"password123", "Password123", "password1234", "Password1234",
		"qwerty1", "Qwerty1", "qwerty12", "Qwerty12",
		"1q2w3e", "1q2w3e4r", "1q2w3e4r5t", "1qaz2wsx",
		"1qaz@wsx", "1qaz!QAZ", "zaq12wsx", "ZAQ!2wsx",
		"q1w2e3r4", "qaz123", "qazwsx", "qazwsxedc",
		"asdf1234", "Asdf1234", "asdfgh", "asdfghjkl",
		"zxcvbn", "zxcvbnm", "zxcvbn123", "Zxcvbn123",
		"changeme", "Changeme", "CHANGEME",
		"temp", "Temp", "TEMP", "temp123",
		"temp1234", "Temp1234", "temporary", "Temporary",
		"default", "Default", "DEFAULT", "default123",
		// ---- 键盘图案类 ----
		"qwertyuiop", "asdfghjkl", "zxcvbnm",
		"qwer1234", "Qwer1234", "qweasd", "Qweasd",
		"qweasdzxc", "Qweasdzxc", "qweasd123",
		"1qaz2wsx3edc", "!qaz2wsx", "!qaz@wsx",
		// ---- 中文拼音类 ----
		"woaini", "woaini520", "woaini1314", "woaini521",
		"aini", "aini1314", "iloveyou520",
		"5201314", "5201314520", "520520",
		"1314520", "13145201314", "13141314",
		"zhangsan", "lisi", "wangwu",
		"mima", "mima123", "mima520",
		"wang123", "wang1234", "li123", "li1234",
		"zhang123", "zhang1234",
		// ---- 其他常见 ----
		"access14", "baseball1", "soccer", "Soccer",
		"buster", "Buster", "mustang", "Mustang",
		"merlin", "Merlin", "falcon", "Falcon",
		"tigger", "Tigger", "andrea", "Andrea",
		"matthew", "Matthew", "joshua1", "Joshua1",
		"chester", "Chester", "taylor", "Taylor",
		"andrew", "Andrew", "dallas", "Dallas",
		"cookie", "Cookie", "knight", "Knight",
		"richard", "Richard", "samantha", "Samantha",
		"charles", "Charles", "bonnie", "Bonnie",
		"orange", "Orange", "purple", "Purple",
		"diamond", "Diamond", "jasmine", "Jasmine",
		"angel", "Angel", "ANGEL", "angel123",
		"soccer1", "Soccer1", "buster1", "Buster1",
	}

	m := make(map[string]struct{}, len(list))
	for _, p := range list {
		m[p] = struct{}{}
	}
	return m
}()

// isCommonPassword 检查密码是否在本地常见弱密码列表中
func isCommonPassword(password string) bool {
	if password == "" {
		return false
	}
	// 精确匹配
	if _, ok := commonPasswords[password]; ok {
		return true
	}
	// 尝试常见变体：首字母大写
	if len(password) > 1 {
		capitalized := strings.ToUpper(password[:1]) + password[1:]
		if _, ok := commonPasswords[capitalized]; ok {
			return true
		}
	}
	// 全小写匹配
	lower := strings.ToLower(password)
	if _, ok := commonPasswords[lower]; ok {
		return true
	}
	return false
}
