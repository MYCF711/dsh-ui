package dsopen

import "testing"

// TestFileURLShape 把链接形状单独钉住：Windows 盘符要补前导斜杠、空格要转义、
// 相对路径不能造出"看起来像绝对路径"的链接。
func TestFileURLShape(t *testing.T) {
	ok := []struct{ in, want string }{
		{`D:\ws`, "file:///D:/ws"},
		{`D:\ws\`, "file:///D:/ws/"},
		{`D:\工作 区\a b.txt`, "file:///D:/%E5%B7%A5%E4%BD%9C%20%E5%8C%BA/a%20b.txt"},
		{"/home/me/ws", "file:///home/me/ws"},
		{`\\server\share\ws`, "file:////server/share/ws"},
	}
	for _, tc := range ok {
		got, err := fileURL(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("fileURL(%q) = %q, %v；want %q", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{"", "ws", `ws\proj`, `.\proj`, `D:ws`, " D:\\ws", `D:\ws `, "//"} {
		if got, err := fileURL(bad); err == nil {
			t.Errorf("fileURL(%q) 不该成功，得到 %q", bad, got)
		}
	}
}

// TestAbsoluteCWDIsNotGuessed 钉住"绝对"的判据本身：
// 不认相对路径、不认盘符相对、不认首尾空白；认盘符根、UNC 与 POSIX 根。
func TestAbsoluteCWDIsNotGuessed(t *testing.T) {
	for _, good := range []string{`D:\ws`, `d:/ws`, `D:\`, `\\server\share`, "/srv/ws", "/"} {
		if !absoluteCWD(good) {
			t.Errorf("absoluteCWD(%q) = false，want true", good)
		}
	}
	for _, bad := range []string{"", " ", "ws", `ws\proj`, `.\ws`, `..\ws`, `D:ws`, `1:\ws`, " D:\\ws", `C:\ws `, "//"} {
		if absoluteCWD(bad) {
			t.Errorf("absoluteCWD(%q) = true，want false", bad)
		}
	}
}
